package trilha

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// ProblemMediaType is what an API error is sent as (RFC 9457).
const ProblemMediaType = "application/problem+json"

// Problem is an API error in the RFC 9457 shape. Return one from a handler to
// say more than a status code:
//
//	return &trilha.Problem{
//		Type:   "https://example.com/probs/out-of-credit",
//		Title:  "Out of credit",
//		Status: http.StatusPaymentRequired,
//		Detail: "The account has $3 and the operation costs $10.",
//		Extra:  map[string]any{"balance": 300},
//	}
//
// Empty fields are filled in by the framework: Type becomes "about:blank" (or
// what ProblemType returns), Title the status text, Instance the request path,
// and request_id the id of the request. Extra members are written next to the
// standard ones, at the top level, which is what the RFC calls an extension.
type Problem struct {
	// Type is a URI identifying the kind of problem — usually a page of yours
	// documenting it. "about:blank" means "nothing beyond the status".
	Type string `json:"type,omitempty"`
	// Title is the short, human-readable summary, the same for every
	// occurrence of this Type.
	Title string `json:"title"`
	// Status is the HTTP status code.
	Status int `json:"status"`
	// Detail explains this occurrence. It is read by a person: never put a
	// stack trace, a query or a DSN in it.
	Detail string `json:"detail,omitempty"`
	// Instance identifies this occurrence (default: the request path).
	Instance string `json:"instance,omitempty"`
	// Fields carries the per-field messages of a 422, unchanged.
	Fields FieldErrors `json:"fields,omitempty"`
	// Extra members are merged into the top-level object.
	Extra map[string]any `json:"-"`
}

func (p *Problem) Error() string {
	if p.Detail != "" {
		return "trilha: " + strconv.Itoa(p.Status) + " " + p.Title + ": " + p.Detail
	}
	return "trilha: " + strconv.Itoa(p.Status) + " " + p.Title
}

// MarshalJSON writes the standard members and then the extension ones, so
// Extra reads like part of the object instead of a bag inside it.
func (p *Problem) MarshalJSON() ([]byte, error) {
	type plain Problem
	b, err := json.Marshal((*plain)(p))
	if err != nil || len(p.Extra) == 0 {
		return b, err
	}
	ex, err := json.Marshal(p.Extra)
	if err != nil {
		return nil, err
	}
	if len(ex) <= 2 {
		return b, nil
	}
	return append(append(b[:len(b)-1:len(b)-1], ','), ex[1:]...), nil
}

// ProblemType gives the "type" URI of a status, for an app that documents its
// errors ("https://example.com/probs/404"). nil keeps "about:blank".
var ProblemType func(status int) string

// problemFor builds the problem actually sent: what the handler said, plus
// what the framework knows, minus what production must not tell.
func (a *App) problemFor(c *Ctx, err error, code int) *Problem {
	p := &Problem{}
	var from *Problem
	if errors.As(err, &from) {
		copied := *from
		p = &copied
		p.Extra = copyExtra(from.Extra)
	}
	if p.Status == 0 {
		p.Status = code
	}
	if p.Type == "" && ProblemType != nil {
		p.Type = ProblemType(p.Status)
	}
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Title == "" {
		p.Title = http.StatusText(p.Status)
	}
	if p.Detail == "" {
		var he *HTTPError
		switch {
		case p.Status < 500 && errors.As(err, &he) && he.Message != "":
			p.Detail = he.Message
		case p.Status >= 500 && a.cfg.Env == Dev:
			// Only in dev: in production the message goes to the log, with the
			// request id, and the client gets the status alone (ASVS V7.4.1).
			p.Detail = err.Error()
		}
	}
	if p.Instance == "" {
		p.Instance = c.r.URL.Path
	}
	if len(p.Fields) == 0 {
		var fe FieldErrors
		if errors.As(err, &fe) {
			p.Fields = fe
		}
	}
	if c.requestID != "" {
		if p.Extra == nil {
			p.Extra = map[string]any{}
		}
		if _, ok := p.Extra["request_id"]; !ok {
			// In the body, not only in X-Request-ID: a script from another
			// origin cannot read the header without Expose-Headers, and this
			// is the number support will ask for.
			p.Extra["request_id"] = c.requestID
		}
	}
	return p
}

func copyExtra(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (c *Ctx) writeProblem(p *Problem) {
	c.w.Header().Set("Content-Type", ProblemMediaType+"; charset=utf-8")
	c.w.WriteHeader(p.Status)
	_ = json.NewEncoder(c.w).Encode(p)
}

// Accepts returns the offer the client prefers, according to the Accept
// header and its q values, or "" when it accepts none of them. An absent or
// */* Accept is not a preference: the first offer wins, so put your default
// first.
//
//	switch c.Accepts("text/html", "application/json") {
//	case "application/json": return c.JSON(200, v)
//	default:                 return c.Render(200, page(v))
//	}
func (c *Ctx) Accepts(offers ...string) string {
	return negotiate(c.r.Header.Get("Accept"), offers)
}

func negotiate(header string, offers []string) string {
	if len(offers) == 0 {
		return ""
	}
	if strings.TrimSpace(header) == "" {
		return offers[0]
	}
	best, bestQ, bestSpec := "", 0.0, -1
	for _, o := range offers {
		q, spec := quality(header, o)
		if q <= 0 {
			continue
		}
		if q > bestQ || (q == bestQ && spec > bestSpec) {
			best, bestQ, bestSpec = o, q, spec
		}
	}
	return best
}

// quality is the q of the most specific media range that matches the offer,
// with the specificity itself (2 = type/subtype, 1 = type/*, 0 = */*).
func quality(header, offer string) (float64, int) {
	otype, osub, _ := strings.Cut(offer, "/")
	q, spec := 0.0, -1
	for _, part := range strings.Split(header, ",") {
		media, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		t, s, ok := strings.Cut(strings.TrimSpace(media), "/")
		if !ok {
			continue
		}
		this := -1
		switch {
		case t == otype && s == osub:
			this = 2
		case t == otype && s == "*":
			this = 1
		case t == "*" && s == "*":
			this = 0
		default:
			continue
		}
		if this <= spec {
			continue
		}
		spec, q = this, qValue(params)
	}
	return q, spec
}

func qValue(params string) float64 {
	for _, p := range strings.Split(params, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok || strings.TrimSpace(k) != "q" {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 1
		}
		return n
	}
	return 1
}

// prefersHTML reports that the client asked for a page rather than an API
// answer — a browser in the address bar, not a fetch.
func prefersHTML(r *http.Request) bool {
	return negotiate(r.Header.Get("Accept"), []string{"application/json", "text/html"}) == "text/html"
}

// prefersJSON is the same question the other way round, for the request that
// matched no route and has no route kind to fall back on.
func prefersJSON(r *http.Request) bool {
	return negotiate(r.Header.Get("Accept"), []string{"text/html", "application/json"}) == "application/json"
}
