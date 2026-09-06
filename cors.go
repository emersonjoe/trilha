package trilha

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CORS is the cross-origin policy of the app, in Config.CORS. The zero value
// is off: no header is added and OPTIONS keeps reaching the router.
//
//	CORS: trilha.CORS{
//		Origins:     []string{"https://app.example.com"},
//		Credentials: true,
//		MaxAge:      10 * time.Minute,
//	}
//
// Origins are exact ("scheme://host[:port]", no path, no trailing slash), or
// the single entry "*" for a public API. An unsafe or malformed policy panics
// in New: a CORS mistake that only shows up on the first request from outside
// is a mistake nobody sees in development.
type CORS struct {
	// Origins may call this app. Empty disables CORS; "*" (alone) allows any.
	Origins []string
	// Methods the other origin may use (default GET, HEAD, POST, PUT, PATCH,
	// DELETE).
	Methods []string
	// Headers the other origin may send (default Content-Type, Authorization,
	// X-CSRF-Token, Trilha-Fragment).
	Headers []string
	// Expose lists the response headers the other origin's script may read.
	Expose []string
	// Credentials allows cookies and Authorization. Incompatible with "*".
	Credentials bool
	// MaxAge is how long the browser may cache the preflight (zero omits the
	// header, and the browser uses its own short default).
	MaxAge time.Duration
}

// corsPolicy is the CORS config turned into the strings the response carries,
// so a request only compares and writes.
type corsPolicy struct {
	any         bool
	origins     map[string]bool
	methods     map[string]bool
	methodList  string
	headerList  string
	exposeList  string
	credentials bool
	maxAge      string
}

var (
	corsDefaultMethods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"}
	corsDefaultHeaders = []string{"Content-Type", "Authorization", CSRFHeader, fragmentHeader}
)

func newCORSPolicy(c CORS) *corsPolicy {
	if len(c.Origins) == 0 {
		return nil
	}
	p := &corsPolicy{origins: map[string]bool{}, methods: map[string]bool{}, credentials: c.Credentials}
	for _, o := range c.Origins {
		o = strings.TrimSpace(o)
		if o == "*" {
			if len(c.Origins) > 1 {
				panic(`trilha: Config.CORS.Origins: "*" cannot be mixed with other origins`)
			}
			if c.Credentials {
				// The browser refuses this pair, and the usual "fix" — echoing
				// whatever Origin arrives — hands every site a session.
				panic(`trilha: Config.CORS: Origins "*" with Credentials is not allowed`)
			}
			p.any = true
			continue
		}
		checkOrigin(o)
		p.origins[o] = true
	}
	methods := c.Methods
	if len(methods) == 0 {
		methods = corsDefaultMethods
	}
	for _, m := range methods {
		p.methods[strings.ToUpper(strings.TrimSpace(m))] = true
	}
	p.methodList = strings.Join(methods, ", ")
	headers := c.Headers
	if len(headers) == 0 {
		headers = corsDefaultHeaders
	}
	p.headerList = strings.Join(headers, ", ")
	p.exposeList = strings.Join(c.Expose, ", ")
	if c.MaxAge > 0 {
		p.maxAge = strconv.Itoa(int(c.MaxAge / time.Second))
	}
	return p
}

func checkOrigin(o string) {
	u, err := url.Parse(o)
	bad := err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "http" && u.Scheme != "https")
	if bad {
		panic("trilha: Config.CORS.Origins: " + strconv.Quote(o) + " must be scheme://host[:port]")
	}
}

// allowed reports the value of Access-Control-Allow-Origin for this origin, or
// "" when the origin is not on the list.
func (p *corsPolicy) allowed(origin string) string {
	switch {
	case origin == "":
		return ""
	case p.origins[origin]:
		return origin
	case p.any && p.credentials:
		return origin
	case p.any:
		return "*"
	}
	return ""
}

// handle answers the preflight and, for every other request, adds what the
// browser needs before the router runs. It reports whether the response is
// already finished.
func (p *corsPolicy) handle(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	h := w.Header()
	// Without this a shared cache serves the allowed origin's response to
	// somebody else.
	h.Add("Vary", "Origin")
	allow := p.allowed(origin)
	if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		h.Add("Vary", "Access-Control-Request-Method")
		h.Add("Vary", "Access-Control-Request-Headers")
		method := strings.ToUpper(r.Header.Get("Access-Control-Request-Method"))
		if allow == "" || !p.methods[method] {
			// The browser is asking, so it gets an answer: a 403 here shows up
			// in the network tab, an empty 204 looks like the app is broken.
			http.Error(w, "cross-origin request not allowed", http.StatusForbidden)
			return true
		}
		h.Set("Access-Control-Allow-Origin", allow)
		h.Set("Access-Control-Allow-Methods", p.methodList)
		h.Set("Access-Control-Allow-Headers", p.headerList)
		if p.credentials {
			h.Set("Access-Control-Allow-Credentials", "true")
		}
		if p.maxAge != "" {
			h.Set("Access-Control-Max-Age", p.maxAge)
		}
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	if allow == "" {
		// A simple request from an origin nobody listed is served as usual: the
		// browser hides the response from the script, and a client that is not
		// a browser was never the one being protected here.
		return false
	}
	h.Set("Access-Control-Allow-Origin", allow)
	if p.credentials {
		h.Set("Access-Control-Allow-Credentials", "true")
	}
	if p.exposeList != "" {
		h.Set("Access-Control-Expose-Headers", p.exposeList)
	}
	return false
}
