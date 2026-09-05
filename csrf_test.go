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
	req := httptest.NewRequest("GET", "/form", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if !rec.Result().Cookies()[0].Secure {
		t.Fatal("expected Secure cookie")
	}
}
