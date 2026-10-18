package trilha

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

var csrfValue = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

// #308: the token in the HTML changes on every response, so a compressed page
// does not carry a fixed secret next to text the visitor controls (BREACH).
// Every masked token still checks against the same cookie.
func TestCSRFTokenMaskedPerResponse(t *testing.T) {
	a := csrfApp(false)
	first := get(t, a, "GET", "/form", "", nil)
	cookie := first.Result().Cookies()[0].Value
	jar := map[string]string{"Cookie": CSRFCookie + "=" + cookie}

	var tokens []string
	for _, rec := range []string{first.Body.String(), get(t, a, "GET", "/form", "", jar).Body.String(), get(t, a, "GET", "/form", "", jar).Body.String()} {
		m := csrfValue.FindStringSubmatch(rec)
		if m == nil {
			t.Fatalf("no token in %s", rec)
		}
		tokens = append(tokens, m[1])
	}
	for i, tok := range tokens {
		if strings.Contains(tok, cookie) {
			t.Fatalf("response %d carries the cookie's secret: %s", i, tok)
		}
		for j := range tokens[:i] {
			if tokens[j] == tok {
				t.Fatalf("responses %d and %d carry the same token", j, i)
			}
		}
		form := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": jar["Cookie"]}
		if rec := get(t, a, "POST", "/form", "q=1&_csrf="+url.QueryEscape(tok), form); rec.Code != 200 {
			t.Fatalf("masked token %d refused: %d", i, rec.Code)
		}
		if rec := get(t, a, "POST", "/form", "", map[string]string{"Cookie": jar["Cookie"], CSRFHeader: tok}); rec.Code != 200 {
			t.Fatalf("masked token %d by header refused: %d", i, rec.Code)
		}
	}

	// A page rendered before the upgrade posted the cookie's value itself: it
	// still passes, and it adds nothing to what the HTML gives away.
	form := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": jar["Cookie"]}
	if rec := get(t, a, "POST", "/form", "q=1&_csrf="+cookie, form); rec.Code != 200 {
		t.Fatalf("raw token refused: %d", rec.Code)
	}
	// A tampered token, a token of another session, and a cut one do not.
	other := maskCSRF("another-session-secret-of-43-characters-xx")
	// The middle of the masked half: the last character of base64 carries
	// padding bits, and flipping one of those changes no byte.
	tampered := []byte(tokens[0])
	if tampered[60] == 'A' {
		tampered[60] = 'B'
	} else {
		tampered[60] = 'A'
	}
	for name, tok := range map[string]string{"tampered": string(tampered), "other session": other, "cut": tokens[0][:20], "half": tokens[0][:len(tokens[0])/2]} {
		if rec := get(t, a, "POST", "/form", "q=1&_csrf="+url.QueryEscape(tok), form); rec.Code != 403 {
			t.Errorf("%s token accepted: %d", name, rec.Code)
		}
	}
}

// Inside one response the token is one value: two forms on a page, an island
// and the meta tag all carry the same string.
func TestCSRFTokenStableWithinAResponse(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		return h.Div(h.Form(h.Method("post"), CSRFInput(c)), h.Form(h.Method("post"), CSRFInput(c)), h.P(h.Text(c.CSRFToken()))), nil
	}})
	body := get(t, a, "GET", "/", "", nil).Body.String()
	m := csrfValue.FindAllStringSubmatch(body, -1)
	if len(m) != 2 || m[0][1] != m[1][1] || !strings.Contains(body, "<p>"+m[0][1]+"</p>") {
		t.Fatalf("one response, several tokens: %s", body)
	}
}
