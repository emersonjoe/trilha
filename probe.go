package trilha

import (
	"context"
	"net/http"
)

// A request an application builds itself — a bridge turning an MCP tool call
// into a call of a route, a job replaying a request — needs two things a
// request from the network does not: to say how it arrived, and to ask whether
// a caller would get through without doing anything.

type probeKey struct{}
type viaKey struct{}

// Probe runs the middleware chain of the route that would answer req and
// reports whether the handler would be reached. The handler itself does not
// run: what is being asked is "may this caller do this?", and the answer is
// the chain's — the key, the session, the policy, the tenant — not the
// handler's. Inside, Ctx.Probing is true, which is how auth.Keys knows not to
// spend the caller's rate limit or count a call that never happened. A probe
// is written neither to the access log nor to the metrics.
//
// An unknown path, a method the route does not answer and a chain that
// refuses all answer false.
func (a *App) Probe(req *http.Request) bool {
	req = req.WithContext(context.WithValue(req.Context(), probeKey{}, true))
	w := &probeWriter{h: http.Header{}, status: http.StatusOK}
	a.Handler().ServeHTTP(w, req)
	return w.status < 400
}

// Probing reports whether this request is an App.Probe: the chain is being
// asked, and the handler will not run.
func (c *Ctx) Probing() bool { return probing(c.r) }

func probing(r *http.Request) bool {
	v, _ := r.Context().Value(probeKey{}).(bool)
	return v
}

// WithVia marks how a request arrived, for the audit trail. The auth package
// records an actor as "session" or "api_key" — the mechanism that recognised
// it. A bridge that builds the request on the caller's behalf knows better:
// the key was presented to an MCP server, not to the API, and that is what an
// investigation wants to read. SetActor keeps the subject and takes this
// word for Via.
func WithVia(req *http.Request, via string) *http.Request {
	if via == "" {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), viaKey{}, via))
}

func viaOf(r *http.Request) string {
	v, _ := r.Context().Value(viaKey{}).(string)
	return v
}

// probeWriter keeps the status and discards the rest: a probe has no reader.
type probeWriter struct {
	h      http.Header
	status int
}

func (w *probeWriter) Header() http.Header         { return w.h }
func (w *probeWriter) Write(b []byte) (int, error) { return len(b), nil }
func (w *probeWriter) WriteHeader(code int)        { w.status = code }
