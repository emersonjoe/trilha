package trilha

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// PageSnapshot is the HTML a request got, with what changes from one request
// to the next already taken out: the CSP nonce, the CSRF token, the request
// id, cookie values, timestamps and the durations of Server-Timing become
// {{NONCE}}, {{CSRF}}, {{RID}}, {{COOKIE}}, {{DATE}} and {{DUR}}. Two
// snapshots of the same code are the same text, so a page can be held to a
// golden file and a change to its markup shows up as a diff.
//
// The volatile values are the ones the response itself reveals — the nonce in
// its Content-Security-Policy, the id in X-Request-ID, the tokens in the
// cookies — and they are replaced literally wherever they appear. Nothing is
// guessed from the shape of the HTML, so a value that is not volatile is never
// hidden by accident.
//
// The Has methods check what a page promises without a browser. Each returns
// nil or an error written for whoever has to fix the page: what was looked
// for, what was found, and what to change.
type PageSnapshot struct {
	Status int
	Header http.Header
	Body   string

	csrfField string
}

// CapturePage sends one request through the whole app, as TestRequest does,
// and returns its snapshot. Use (*TestResponse).Snapshot for a page behind a
// login, from a TestClient that already signed in.
func CapturePage(t TestingT, a *App, method, target string, opts ...TestOption) PageSnapshot {
	t.Helper()
	return TestRequest(t, a, method, target, opts...).Snapshot()
}

// Snapshot is this response with its volatile values replaced.
func (r *TestResponse) Snapshot() PageSnapshot {
	field, cookie := CSRFField, CSRFCookie
	if r.app != nil {
		names := r.app.cfg.CSRF.names()
		field, cookie = names.Field, names.Cookie
	}
	var swaps []swap
	if m := nonceInCSP().FindStringSubmatch(r.Header().Get("Content-Security-Policy")); m != nil {
		swaps = append(swaps, swap{m[1], "{{NONCE}}"})
	}
	if id := r.Header().Get("X-Request-ID"); id != "" {
		swaps = append(swaps, swap{id, "{{RID}}"})
	}
	cookies := r.Result().Cookies()
	if r.Request != nil {
		cookies = append(cookies, r.Request.Cookies()...)
	}
	for _, ck := range cookies {
		with := "{{COOKIE}}"
		if ck.Name == cookie {
			with = "{{CSRF}}"
		}
		swaps = append(swaps, swap{ck.Value, with})
	}
	// Longest first, so a value that contains another is replaced whole.
	sort.SliceStable(swaps, func(i, j int) bool { return len(swaps[i].value) > len(swaps[j].value) })

	s := PageSnapshot{Status: r.Code, Header: http.Header{}, Body: normalize(r.Body.String(), swaps), csrfField: field}
	for name, values := range r.Header() {
		for _, v := range values {
			v = normalize(v, swaps)
			if name == "Server-Timing" {
				v = durations().ReplaceAllString(v, "dur={{DUR}}")
			}
			s.Header.Add(name, v)
		}
	}
	return s
}

type swap struct{ value, with string }

var nonceInCSP = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`'nonce-([^']+)'`) })

// durations are the timings of Server-Timing, which are the clock too.
var durations = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`dur=[0-9.]+(?:µs|ms|s)?`) })

// timestamps are the dates that come from the clock: RFC 3339 with a time of
// day, and the HTTP date of Date, Expires and cookie expiry. A date without a
// time is content — the day a post was published — and stays.
var timestamps = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})` +
		`|[A-Z][a-z]{2}, \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT`)
})

func normalize(s string, swaps []swap) string {
	for _, sw := range swaps {
		// A short value would be found inside words that are not it.
		if len(sw.value) >= 8 {
			s = strings.ReplaceAll(s, sw.value, sw.with)
		}
	}
	return timestamps().ReplaceAllString(s, "{{DATE}}")
}

// String is the snapshot as a golden file holds it: the status, the headers
// in order, a blank line and the body.
func (s PageSnapshot) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s\n", s.Status, http.StatusText(s.Status))
	names := make([]string, 0, len(s.Header))
	for name := range s.Header {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, v := range s.Header[name] {
			fmt.Fprintf(&b, "%s: %s\n", name, v)
		}
	}
	b.WriteString("\n")
	b.WriteString(s.Body)
	return b.String()
}

// MatchGolden compares the snapshot with the file at path. With update true
// it writes the file instead — wire it to a -update flag of the test.
func (s PageSnapshot) MatchGolden(path string, update bool) error {
	got := s.String()
	if update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(got), 0o644)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("golden %s: %w (run the test with -update to write it)", path, err)
	}
	if got == string(want) {
		return nil
	}
	gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gl) || i < len(wl); i++ {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			col := 0
			for col < len(g) && col < len(w) && g[col] == w[col] {
				col++
			}
			from := max(0, col-60)
			return fmt.Errorf("golden %s differs at line %d, column %d:\n  want: …%s\n  got:  …%s\n"+
				"fix: if the change is intended, rerun with -update and review the diff; "+
				"if not, the markup of the page moved", path, i+1, col+1, clip(w[min(from, len(w)):]), clip(g[from:]))
		}
	}
	return nil
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// HasCSRFToken checks that every form that changes something — method other
// than GET — carries the hidden CSRF field. A form without it is refused with
// 403 when submitted, and the test that posts through TestClient does not see
// it, because the client sends the token by header.
func (s PageSnapshot) HasCSRFToken() error {
	field := s.csrfField
	if field == "" {
		field = CSRFField
	}
	var missing []string
	var open *element
	has := false
	for _, el := range parseElements(s.Body) {
		switch {
		case el.tag == "form":
			open, has = &el, false
			if m := strings.ToLower(el.attr("method")); m == "" || m == "get" {
				open = nil
			}
		case el.tag == "/form":
			if open != nil && !has {
				missing = append(missing, open.String())
			}
			open = nil
		case open != nil && el.tag == "input" && el.attr("name") == field:
			has = el.attr("value") != ""
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d form(s) post without the %s field: %s\n"+
			"fix: put trilha.CSRFInput(c) inside the form (ui.Form and the kit's forms already do)",
			len(missing), field, strings.Join(missing, ", "))
	}
	return nil
}

// HasAria checks that every element matching sel has attr — aria-label on an
// icon button, aria-describedby on a field with a hint. sel is a compound
// selector: tag, #id, .class, [attr] and [attr=value], together as in
// button.icon[type=submit]. No element matching sel is an error too: a check
// that passes over nothing proves nothing.
func (s PageSnapshot) HasAria(sel, attr string) error {
	m, err := parseSelector(sel)
	if err != nil {
		return err
	}
	var found int
	var without []string
	for _, el := range parseElements(s.Body) {
		if !m.match(el) {
			continue
		}
		found++
		if _, ok := el.attrs[attr]; !ok {
			without = append(without, el.String())
		}
	}
	switch {
	case found == 0:
		return fmt.Errorf("no element matches %q, so %s could not be checked\n"+
			"fix: check the selector against the page, or render the element", sel, attr)
	case len(without) > 0:
		return fmt.Errorf("%d of %d element(s) matching %q have no %s: %s\n"+
			"fix: add h.Attr(%q, …) — or the kit component that sets it", len(without), found, sel, attr,
			strings.Join(without, ", "), attr)
	}
	return nil
}

// FocusedOnError checks where the focus lands after a form comes back with
// its errors (422): the kit's script focuses the first element marked
// aria-invalid="true", in document order, and that has to be the element
// matching sel. It is the rule the browser applies, checked on the HTML.
func (s PageSnapshot) FocusedOnError(sel string) error {
	m, err := parseSelector(sel)
	if err != nil {
		return err
	}
	for _, el := range parseElements(s.Body) {
		if el.attr("aria-invalid") != "true" {
			continue
		}
		if m.match(el) {
			return nil
		}
		return fmt.Errorf("the focus goes to %s, the first aria-invalid element, and not to %q\n"+
			"fix: mark only the fields that failed with ui.InvalidIf, in the order of the form", el.String(), sel)
	}
	return fmt.Errorf("no element is aria-invalid=\"true\", so nothing receives the focus (status %d)\n"+
		"fix: answer the failed form with 422 and mark the field with ui.InvalidIf(err != nil) and ui.FormError", s.Status)
}

// HasCSPNonce checks that every inline <script> and <style> carries the nonce
// the Content-Security-Policy announces (E_CSP_NONCE). Without it the browser
// refuses to run the script — the page works in the test and breaks on the
// screen. A <script src> is covered by 'self' and needs none.
func (s PageSnapshot) HasCSPNonce() error {
	csp := s.Header.Get("Content-Security-Policy")
	if csp == "" {
		return fmt.Errorf("E_CSP_NONCE: the response has no Content-Security-Policy, so there is no nonce to check\n" +
			"fix: keep the default Security, or set Security.CSP with 'nonce-{nonce}'")
	}
	nonced := strings.Contains(csp, "'nonce-{{NONCE}}'")
	var bad []string
	for _, el := range parseElements(s.Body) {
		inline := el.tag == "style" || (el.tag == "script" && el.attr("src") == "" &&
			!strings.Contains(el.attr("type"), "json"))
		if !inline {
			continue
		}
		if el.attr("nonce") != "{{NONCE}}" {
			bad = append(bad, el.String())
		}
	}
	if len(bad) == 0 {
		return nil
	}
	if !nonced {
		return fmt.Errorf("E_CSP_NONCE: %d inline script/style and a policy without a nonce: %s\n"+
			"fix: use 'nonce-{nonce}' in Security.CSP, or move the code to a file under public/", len(bad), strings.Join(bad, ", "))
	}
	return fmt.Errorf("E_CSP_NONCE: %d inline script/style without the nonce: %s\n"+
		"fix: write them with trilha.NonceAttr(c) — h.Script(trilha.NonceAttr(c), …) — or move them to public/",
		len(bad), strings.Join(bad, ", "))
}

// HasSafeCookies checks every cookie the response sets for HttpOnly and
// SameSite: a cookie a script can read is a session a stolen script can take.
func (s PageSnapshot) HasSafeCookies() error {
	var bad []string
	for _, line := range s.Header.Values("Set-Cookie") {
		parts := strings.Split(line, ";")
		name, _, _ := strings.Cut(strings.TrimSpace(parts[0]), "=")
		httpOnly, sameSite, deleted := false, false, false
		for _, p := range parts[1:] {
			k, v, _ := strings.Cut(strings.TrimSpace(p), "=")
			switch strings.ToLower(k) {
			case "httponly":
				httpOnly = true
			case "samesite":
				sameSite = v != ""
			case "max-age":
				deleted = strings.HasPrefix(v, "-") || v == "0"
			}
		}
		if deleted {
			continue // being deleted
		}
		var why []string
		if !httpOnly {
			why = append(why, "HttpOnly")
		}
		if !sameSite {
			why = append(why, "SameSite")
		}
		if len(why) > 0 {
			bad = append(bad, name+" without "+strings.Join(why, " and "))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("unsafe cookie(s): %s\nfix: set HttpOnly: true and SameSite: http.SameSiteLaxMode on the http.Cookie",
			strings.Join(bad, "; "))
	}
	return nil
}

// HasNoSecret checks that none of values — a provider's key, a webhook
// secret, a password the test saved — is in the body or in a header. A screen
// that shows a secret back hands it to whoever looks over a shoulder, and to
// the browser cache.
func (s PageSnapshot) HasNoSecret(values ...string) error {
	all := s.String()
	for _, v := range values {
		if v == "" {
			continue
		}
		if i := strings.Index(all, v); i >= 0 {
			from, to := max(0, i-60), min(len(all), i+len(v)+20)
			return fmt.Errorf("a secret is in the page: …%s…\n"+
				"fix: show that it is set (ui.Badge, the last four characters), never the value", all[from:to])
		}
	}
	return nil
}

// element is one start tag: enough HTML to find a form, a field and its
// attributes, without a parser the runtime would have to depend on. End tags
// come through as "/tag" with no attributes.
type element struct {
	tag   string
	attrs map[string]string
	raw   string
}

func (e element) attr(name string) string { return e.attrs[name] }

func (e element) String() string { return clip(e.raw) }

// parseElements walks the tags of an HTML document in order. It skips
// comments and the contents of <script> and <style>, which are not markup.
func parseElements(doc string) []element {
	var out []element
	for i := 0; i < len(doc); {
		lt := strings.IndexByte(doc[i:], '<')
		if lt < 0 {
			break
		}
		i += lt
		if strings.HasPrefix(doc[i:], "<!--") {
			end := strings.Index(doc[i:], "-->")
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		end := tagEnd(doc, i)
		if end < 0 {
			break
		}
		el, ok := parseTag(doc[i : end+1])
		i = end + 1
		if !ok {
			continue
		}
		out = append(out, el)
		if el.tag == "script" || el.tag == "style" {
			if close := strings.Index(strings.ToLower(doc[i:]), "</"+el.tag); close >= 0 {
				i += close
			}
		}
	}
	return out
}

// tagEnd finds the > that closes the tag at i, outside quoted values.
func tagEnd(doc string, i int) int {
	var quote byte
	for j := i + 1; j < len(doc); j++ {
		c := doc[j]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return j
		}
	}
	return -1
}

func parseTag(raw string) (element, bool) {
	body := strings.TrimSuffix(strings.TrimSuffix(raw[1:len(raw)-1], "/"), " ")
	if body == "" || body[0] == '!' || body[0] == '?' {
		return element{}, false
	}
	n := strings.IndexAny(body, " \t\n\r")
	if n < 0 {
		n = len(body)
	}
	el := element{tag: strings.ToLower(body[:n]), attrs: map[string]string{}, raw: raw}
	rest := body[n:]
	for {
		rest = strings.TrimLeft(rest, " \t\n\r")
		if rest == "" {
			break
		}
		k := strings.IndexAny(rest, "= \t\n\r")
		if k < 0 {
			el.attrs[strings.ToLower(rest)] = ""
			break
		}
		name := strings.ToLower(rest[:k])
		rest = strings.TrimLeft(rest[k:], " \t\n\r")
		if !strings.HasPrefix(rest, "=") {
			el.attrs[name] = ""
			continue
		}
		rest = strings.TrimLeft(rest[1:], " \t\n\r")
		var v string
		if rest != "" && (rest[0] == '"' || rest[0] == '\'') {
			q := rest[0]
			e := strings.IndexByte(rest[1:], q)
			if e < 0 {
				v, rest = rest[1:], ""
			} else {
				v, rest = rest[1:e+1], rest[e+2:]
			}
		} else {
			e := strings.IndexAny(rest, " \t\n\r")
			if e < 0 {
				e = len(rest)
			}
			v, rest = rest[:e], rest[e:]
		}
		el.attrs[name] = unescapeAttr(v)
	}
	return el, true
}

func unescapeAttr(v string) string {
	if !strings.Contains(v, "&") {
		return v
	}
	return strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#34;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(v)
}

// selector is a compound selector: one tag, ids, classes and attributes that
// must all hold on the same element.
type selector struct {
	tag     string
	classes []string
	attrs   [][2]string // name, value; value "\x00" = only present
}

func parseSelector(sel string) (selector, error) {
	var s selector
	bad := func() (selector, error) {
		return selector{}, fmt.Errorf("selector %q: only tag, #id, .class, [attr] and [attr=value] together "+
			"(no spaces, no >) are understood here", sel)
	}
	rest := strings.TrimSpace(sel)
	outside := regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(rest, "")
	if rest == "" || strings.ContainsAny(outside, " >+~,:") {
		return bad()
	}
	word := func() string {
		n := strings.IndexAny(rest, "#.[")
		if n < 0 {
			n = len(rest)
		}
		w := rest[:n]
		rest = rest[n:]
		return w
	}
	s.tag = strings.ToLower(word())
	for rest != "" {
		switch rest[0] {
		case '#':
			rest = rest[1:]
			s.attrs = append(s.attrs, [2]string{"id", word()})
		case '.':
			rest = rest[1:]
			s.classes = append(s.classes, word())
		case '[':
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				return bad()
			}
			in := rest[1:end]
			rest = rest[end+1:]
			name, value, ok := strings.Cut(in, "=")
			if !ok {
				value = "\x00"
			} else {
				value = strings.Trim(value, `"'`)
			}
			s.attrs = append(s.attrs, [2]string{strings.ToLower(strings.TrimSpace(name)), value})
		default:
			return bad()
		}
	}
	return s, nil
}

func (s selector) match(el element) bool {
	if strings.HasPrefix(el.tag, "/") || (s.tag != "" && s.tag != el.tag) {
		return false
	}
	for _, a := range s.attrs {
		v, ok := el.attrs[a[0]]
		if !ok || (a[1] != "\x00" && v != a[1]) {
			return false
		}
	}
	if len(s.classes) > 0 {
		have := strings.Fields(el.attrs["class"])
		for _, c := range s.classes {
			if !containsString(have, c) {
				return false
			}
		}
	}
	return true
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
