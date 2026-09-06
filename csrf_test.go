package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

func csrfApp(apiToo bool) *App {
	a := New(Config{Logger: quiet(), CSRFForAPI: apiToo})
	a.Register(Route{Pattern: "/form",
		Page: func(c *Ctx) (h.Node, error) {
			return h.Form(h.Method("post"), CSRFInput(c), h.Input(h.Name("q"))), nil
		},
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return c.Text(200, "ok "+c.Form("q")) }},
	})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return c.Text(200, "api") }}})
	return a
}

func TestCSRFTokenCreatedOnRender(t *testing.T) {
	a := csrfApp(false)
	rec := get(t, a, "GET", "/form", "", nil)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CSRFCookie || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("%+v", cookies)
	}
	if !strings.Contains(rec.Body.String(), `name="_csrf" value="`+cookies[0].Value+`"`) {
		t.Fatal(rec.Body.String())
	}
	// Second render with the cookie reuses it.
	rec2 := get(t, a, "GET", "/form", "", map[string]string{"Cookie": CSRFCookie + "=" + cookies[0].Value})
	if len(rec2.Result().Cookies()) != 0 || !strings.Contains(rec2.Body.String(), cookies[0].Value) {
		t.Fatal("cookie should be reused")
	}
}

func TestCSRFEnforcedOnPageForms(t *testing.T) {
	a := csrfApp(false)
	tok := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEF"
	form := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": CSRFCookie + "=" + tok}
	if rec := get(t, a, "POST", "/form", "q=1", map[string]string{"Content-Type": form["Content-Type"]}); rec.Code != 403 {
		t.Fatalf("no cookie: %d", rec.Code)
	}
	if rec := get(t, a, "POST", "/form", "q=1", form); rec.Code != 403 {
		t.Fatalf("no field: %d", rec.Code)
	}
	if rec := get(t, a, "POST", "/form", "q=1&_csrf=wrong", form); rec.Code != 403 {
		t.Fatalf("wrong field: %d", rec.Code)
	}
	if rec := get(t, a, "POST", "/form", "q=1&_csrf="+tok, form); rec.Code != 200 || rec.Body.String() != "ok 1" {
		t.Fatalf("valid: %d %s", rec.Code, rec.Body.String())
	}
	hdr := map[string]string{"Cookie": CSRFCookie + "=" + tok, CSRFHeader: tok}
	if rec := get(t, a, "POST", "/form", "", hdr); rec.Code != 200 {
		t.Fatalf("header token: %d", rec.Code)
	}
	// API routes are exempt by default, enforced with CSRFForAPI.
	if rec := get(t, a, "POST", "/api/x", "", nil); rec.Code != 200 {
		t.Fatalf("api exempt: %d", rec.Code)
	}
	if rec := get(t, csrfApp(true), "POST", "/api/x", "", nil); rec.Code != 403 {
		t.Fatalf("api enforced: %d", rec.Code)
	}
}

func TestSecureCookieBehindProxy(t *testing.T) {
	a := csrfApp(false)
	a.cfg.TrustedProxies = []string{"192.0.2.1"} // httptest's RemoteAddr
	a.parseProxies()
	req := httptest.NewRequest("GET", "/form", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if !rec.Result().Cookies()[0].Secure {
		t.Fatal("expected Secure cookie")
	}
}

// Spec 046 (#54): inside a host that already has a _csrf of its own, the two
// names have to be tellable apart — in the HTML, in the cookie jar and in the
// header. The constants stay as the defaults; Config picks other names.
func TestCSRFNamesFromConfig(t *testing.T) {
	a := New(Config{Logger: quiet(), CSRF: CSRF{
		Cookie: "farol_trilha_csrf", Field: "_farol_trilha_csrf", Header: "X-Farol-Trilha-Token",
	}})
	a.Register(Route{Pattern: "/form",
		Page: func(c *Ctx) (h.Node, error) {
			return h.Form(h.Method("post"), CSRFInput(c)), nil
		},
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return c.Text(200, "ok") }},
	})

	rec := get(t, a, "GET", "/form", "", nil)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "farol_trilha_csrf" {
		t.Fatalf("cookie: %+v", cookies)
	}
	tok := cookies[0].Value
	if !strings.Contains(rec.Body.String(), `name="_farol_trilha_csrf" value="`+tok+`"`) {
		t.Fatalf("field: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `name="_csrf"`) {
		t.Fatal("the default field name is still in the page")
	}

	form := map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Cookie": "farol_trilha_csrf=" + tok}
	if rec := get(t, a, "POST", "/form", "_farol_trilha_csrf="+tok, form); rec.Code != 200 {
		t.Fatalf("renamed field: %d", rec.Code)
	}
	if rec := get(t, a, "POST", "/form", "_csrf="+tok, form); rec.Code != 403 {
		t.Fatalf("the old field name must not pass: %d", rec.Code)
	}
	if rec := get(t, a, "POST", "/form", "", map[string]string{"Cookie": "farol_trilha_csrf=" + tok, "X-Farol-Trilha-Token": tok}); rec.Code != 200 {
		t.Fatalf("renamed header: %d", rec.Code)
	}
	if rec := get(t, a, "POST", "/form", "", map[string]string{"Cookie": "farol_trilha_csrf=" + tok, CSRFHeader: tok}); rec.Code != 403 {
		t.Fatalf("the old header must not pass: %d", rec.Code)
	}
	// The token is still reachable by the app's own name, for the test client
	// and for whoever reads the jar instead of the HTML.
	if a.Config().CSRF.Cookie != "farol_trilha_csrf" {
		t.Fatalf("Config().CSRF: %+v", a.Config().CSRF)
	}
}

// The default stays what it was: nobody who does not embed writes a line.
func TestCSRFNamesDefault(t *testing.T) {
	a := csrfApp(false)
	a.Handler()
	if got := a.Config().CSRF; got.Cookie != CSRFCookie || got.Field != CSRFField || got.Header != CSRFHeader {
		t.Fatalf("%+v", got)
	}
}
