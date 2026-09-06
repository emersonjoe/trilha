package trilha

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Upstream forwards a URL prefix to an API that already exists, the way a
// Next.js rewrite does — and adds the two things a rewrite has no place for:
// the credential of the session and the CSRF check. The target is fixed here;
// the request only chooses the path below the prefix, so this is never an open
// proxy.
//
//	cfg.Upstreams = map[string]trilha.Upstream{
//		"/api/": {
//			Target: os.Getenv("API_URL"),
//			Headers: func(c *trilha.Ctx, hdr http.Header) {
//				if tok, ok := c.Signed("api_token"); ok {
//					hdr.Set("Authorization", "Bearer "+tok)
//				}
//			},
//		},
//	}
type Upstream struct {
	// Target is the base URL of the API ("http://api:8801"). A path in it is
	// a prefix of every forwarded request.
	Target string
	// Headers runs before the request leaves, with the outbound headers. It
	// is where the session credential is injected; the Authorization the
	// browser sent is not forwarded, so this is the only way one gets there.
	Headers func(c *Ctx, hdr http.Header)
	// Timeout bounds the whole exchange (default 30s; NoTimeout disables it,
	// for an endpoint that streams).
	Timeout time.Duration
	// CSRF set to Off stops requiring the token on POST/PUT/PATCH/DELETE, for
	// an API authenticated by key rather than by the session cookie.
	CSRF string
	// Transport replaces http.DefaultTransport (mTLS to the API, a test).
	Transport http.RoundTripper
}

// upstream is a parsed Upstream: the prefix it answers and the proxy that
// carries the request.
type upstream struct {
	prefix string
	cfg    Upstream
	target *url.URL
	proxy  *httputil.ReverseProxy
}

// parseUpstreams builds one proxy per prefix, longest first so that "/api/v2/"
// wins over "/api/". A target that does not parse is a boot-time complaint,
// not a 502 an hour later.
func (a *App) parseUpstreams() {
	a.upstreams = a.upstreams[:0]
	for p, u := range a.cfg.Upstreams {
		if p == "" || u.Target == "" {
			a.warnOnce("upstream-empty:"+p, "trilha: upstream ignored: no prefix or no target", "prefix", p)
			continue
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		t, err := url.Parse(strings.TrimSuffix(u.Target, "/"))
		if err != nil || t.Scheme == "" || t.Host == "" {
			a.warnOnce("upstream-target:"+p, "trilha: upstream ignored: invalid target", "prefix", p, "target", u.Target)
			continue
		}
		up := &upstream{prefix: p, cfg: u, target: t}
		up.proxy = a.newProxy(up)
		a.upstreams = append(a.upstreams, up)
	}
	sort.Slice(a.upstreams, func(i, j int) bool { return len(a.upstreams[i].prefix) > len(a.upstreams[j].prefix) })
}

// newProxy wires the stdlib reverse proxy: hop-by-hop headers, streaming and
// flushing are its job. What belongs to the framework is the policy — where
// the request may go, what credential it carries, and what an error looks like.
func (a *App) newProxy(up *upstream) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Transport: up.cfg.Transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			out, in := pr.Out, pr.In
			out.URL.Scheme, out.URL.Host = up.target.Scheme, up.target.Host
			out.Host = up.target.Host
			out.URL.Path = up.target.Path + in.URL.Path
			out.URL.RawPath = ""
			// The browser's own credential is not the app's: whoever sends a
			// Bearer from outside does not get to talk to the API with it.
			out.Header.Del("Authorization")
			pr.SetXForwarded()
			out.Header.Set("X-Forwarded-Host", in.Host)
			// Headers runs here, on the outbound copy and after the Del, so
			// the credential it sets is the one that leaves.
			if up.cfg.Headers != nil {
				if c, ok := in.Context().Value(ctxKeyUpstream).(*Ctx); ok {
					up.cfg.Headers(c, out.Header)
				}
			}
		},
		ModifyResponse: func(res *http.Response) error {
			// The stdlib proxy adds the upstream's headers to whatever is
			// already on the ResponseWriter, and applySecurity has already
			// written there. Clearing the keys the upstream speaks about
			// leaves it saying the last word on those, and the framework's
			// defaults standing on every other.
			if c, ok := res.Request.Context().Value(ctxKeyUpstream).(*Ctx); ok {
				dst := c.w.Header()
				for k := range res.Header {
					dst.Del(k)
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			c, _ := req.Context().Value(ctxKeyUpstream).(*Ctx)
			if c == nil {
				http.Error(w, "bad gateway", http.StatusBadGateway)
				return
			}
			code := http.StatusBadGateway
			if errors.Is(err, context.DeadlineExceeded) {
				code = http.StatusGatewayTimeout
			}
			c.Log().Error("trilha: upstream failed", "prefix", up.prefix, "target", up.target.Host, "err", err)
			// The client of an upstream is a script, never a browser in the
			// address bar: problem+json, whatever the Accept says.
			c.kind = kindAPI
			a.handleError(c, &HTTPError{Code: code})
		},
	}
}

// ctxKeyUpstream carries the *Ctx into the proxy's ErrorHandler, which only
// receives the outbound request.
type upstreamKey struct{}

var ctxKeyUpstream = upstreamKey{}

// matchUpstream returns the upstream that answers this path, if any.
func (a *App) matchUpstream(p string) *upstream {
	for _, up := range a.upstreams {
		if strings.HasPrefix(p, up.prefix) {
			return up
		}
	}
	return nil
}

// serveUpstream forwards the request. It reports whether it answered.
func (a *App) serveUpstream(c *Ctx, up *upstream) bool {
	// The path arrives cleaned by the mux; the belt is here because a proxy
	// that can be talked out of its target is the whole class of bug.
	if strings.Contains(c.r.URL.Path, "/../") || strings.HasSuffix(c.r.URL.Path, "/..") {
		c.kind = kindAPI
		a.handleError(c, &HTTPError{Code: http.StatusBadRequest, Message: "invalid path"})
		return true
	}
	if bodyMethods[c.r.Method] && up.cfg.CSRF != Off {
		if err := a.checkCSRF(c); err != nil {
			c.kind = kindAPI
			a.handleError(c, err)
			return true
		}
	}
	// A proxied body is not the app's to limit: an upload of 200 MB and a PDF
	// coming back are both streams that MaxBodyBytes and the write deadline
	// were written for handlers, not for a pipe.
	_ = c.NoWriteDeadline()
	req := c.r
	ctx := context.WithValue(req.Context(), ctxKeyUpstream, c)
	if d := up.cfg.Timeout; d != NoTimeout {
		if d == 0 {
			d = 30 * time.Second
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	req = req.WithContext(ctx)
	req.Header.Set("X-Request-ID", c.requestID)
	up.proxy.ServeHTTP(c.w, req)
	return true
}
