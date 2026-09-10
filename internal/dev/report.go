package dev

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BrowserPath is where the page tells the dev server what it saw. It is the
// other half of ReportPath: the browser reports what the policy refused, and
// the kit's script reports what the server answered.
const BrowserPath = "/_trilha/report"

// maxReport is the largest report body read. A report is a few hundred bytes;
// anything of this size is a mistake or somebody playing.
const maxReport = 64 << 10

// fixes is the line that follows a refusal: what the person does about it. A
// message that names the problem and stops is a message that sends somebody to
// a search engine.
var fixes = map[string]string{
	"frame-src":   "use ui.Preview to show the file, or allow it with Security.CSPExtra{\"frame-src\": {…}}",
	"script-src":  "a script of your own goes in public/ and is loaded with c.Asset; inline needs the nonce (trilha.NonceAttr)",
	"img-src":     "allow the origin with Security.CSPExtra{\"img-src\": {…}}, or serve the image from public/",
	"connect-src": "a call to another origin goes through Config.Upstreams, or is allowed in Security.CSPExtra",
	"style-src":   "put the CSS in a file under public/ instead of a <style> tag",
	"font-src":    "serve the font from public/, or allow its origin with Security.CSPExtra",
	"media-src":   "serve the media from public/, or allow its origin with Security.CSPExtra",
	"object-src":  "an <object>/<embed> has no equivalent here: use ui.Preview or a link to the file",
}

// cspReport prints what the browser refused, with the fix.
func (s *Server) cspReport(w http.ResponseWriter, r *http.Request) {
	defer w.WriteHeader(http.StatusNoContent)
	if r.Method != http.MethodPost {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxReport))
	if err != nil {
		return
	}
	for _, v := range parseCSP(body) {
		fix := fixes[v.directive]
		if fix == "" {
			fix = "allow it in Security.CSPExtra, or stop asking the browser for it"
		}
		fmt.Fprintf(s.Out, "⚠ CSP refused %s: %s\n  at %s — %s\n", v.directive, or(v.blocked, "(no address)"), or(v.page, "?"), fix)
	}
}

// violation is one refusal, read from either shape a browser sends.
type violation struct{ directive, blocked, page string }

// parseCSP reads the two shapes: the old application/csp-report, which Firefox
// and Safari still send, and the Reporting API's array, which is what Chrome
// sends for report-to. Reading both is cheaper than telling people which
// browser to debug in.
func parseCSP(body []byte) []violation {
	var one struct {
		Report struct {
			Directive  string `json:"violated-directive"`
			Effective  string `json:"effective-directive"`
			Blocked    string `json:"blocked-uri"`
			DocumentUR string `json:"document-uri"`
		} `json:"csp-report"`
	}
	if err := json.Unmarshal(body, &one); err == nil && (one.Report.Directive != "" || one.Report.Effective != "") {
		return []violation{{
			directive: name(or(one.Report.Effective, one.Report.Directive)),
			blocked:   one.Report.Blocked,
			page:      path(one.Report.DocumentUR),
		}}
	}
	var many []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
		Body struct {
			Effective string `json:"effectiveDirective"`
			Blocked   string `json:"blockedURL"`
		} `json:"body"`
	}
	if err := json.Unmarshal(body, &many); err != nil {
		return nil
	}
	out := make([]violation, 0, len(many))
	for _, m := range many {
		if m.Type != "" && m.Type != "csp-violation" {
			continue
		}
		out = append(out, violation{directive: name(m.Body.Effective), blocked: m.Body.Blocked, page: path(m.URL)})
	}
	return out
}

// browserReport prints what the kit's script saw. Today there is one kind:
// a fragment that came back as a whole page, which the browser then puts
// inside itself and nobody sees as an error, because the answer was 200.
func (s *Server) browserReport(w http.ResponseWriter, r *http.Request) {
	defer w.WriteHeader(http.StatusNoContent)
	if r.Method != http.MethodPost {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxReport))
	if err != nil {
		return
	}
	var in struct {
		Kind string `json:"kind"`
		URL  string `json:"url"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return
	}
	switch in.Kind {
	case "fragment-html":
		fmt.Fprintf(s.Out, "⚠ the fragment of %s came back as the whole page\n"+
			"  the route answered without looking at c.Fragment(): return only the piece when it is asked for\n",
			or(path(in.URL), "?"))
	}
}

// name trims the directive to its name: a browser may send "frame-src
// 'self'" in the old field.
func name(d string) string {
	if i := strings.IndexByte(d, ' '); i > 0 {
		return d[:i]
	}
	return d
}

// path is the address without the origin, which is what a person recognises.
func path(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.IndexByte(u[i+3:], '/'); j >= 0 {
			return u[i+3+j:]
		}
		return "/"
	}
	return u
}

func or(a, b string) string {
	if a == "" {
		return b
	}
	return a
}
