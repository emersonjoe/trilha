package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// #10 — DevReload can be turned off.
func TestDevReloadOff(t *testing.T) {
	page := func(c *Ctx) (h.Node, error) { return h.P(h.Text("x")), nil }
	serve := func(cfg Config) string {
		cfg.Logger = quiet()
		a := New(cfg)
		a.Register(Route{Pattern: "/", Page: page})
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		return rec.Body.String()
	}
	if !strings.Contains(serve(Config{Env: Dev}), "/_trilha/events") {
		t.Fatal("dev must inject by default")
	}
	if strings.Contains(serve(Config{Env: Dev, DevReload: Off}), "/_trilha/events") {
		t.Fatal("DevReload: Off must not inject")
	}
	t.Setenv("TRILHA_DEV_RELOAD", "off")
	t.Setenv("TRILHA_ENV", "dev")
	cfg := ConfigFromEnv()
	if cfg.DevReload != Off || cfg.Env != Dev {
		t.Fatalf("%+v", cfg)
	}
	if strings.Contains(serve(cfg), "/_trilha/events") {
		t.Fatal("TRILHA_DEV_RELOAD=off must not inject")
	}
}

// #11 — pages that answered by themselves do not get a second body.
func TestAlreadyWrittenPages(t *testing.T) {
	a := New(Config{Logger: quiet(), Env: Dev})
	a.SetRootLayout(func(c *Ctx, ch h.Node) (h.Node, error) { return h.Html(h.Body(ch)), nil })
	a.SetNotFound(func(c *Ctx) (h.Node, error) {
		http.NotFound(c.Writer(), c.Request())
		return nil, nil
	})
	a.SetErrorPage(func(c *Ctx, err error) (h.Node, error) {
		return nil, c.Text(500, "falhou: "+err.Error())
	})
	a.Register(Route{Pattern: "/self", Page: func(c *Ctx) (h.Node, error) {
		return nil, c.Text(200, "texto puro")
	}})
	a.Register(Route{Pattern: "/nothing", Page: func(c *Ctx) (h.Node, error) { return nil, nil }})
	a.Register(Route{Pattern: "/boom", Page: func(c *Ctx) (h.Node, error) { return nil, errFake }})
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		return rec
	}
	if rec := get("/nope"); rec.Code != 404 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") || strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("%d %s %q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if rec := get("/self"); rec.Code != 200 || rec.Body.String() != "texto puro" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if rec := get("/nothing"); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	if rec := get("/boom"); rec.Code != 500 || rec.Body.String() != "falhou: fake" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	// A not_found page that returns nil without writing falls back to the framework page.
	a.SetNotFound(func(c *Ctx) (h.Node, error) { return nil, nil })
	if rec := get("/nope"); rec.Code != 404 || !strings.Contains(rec.Body.String(), "404 Not Found") {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

type fakeErr struct{}

func (fakeErr) Error() string { return "fake" }

var errFake error = fakeErr{}

// #12 — route.go errors: HTML for browsers, JSON otherwise, Kind overrides.
func TestRouteKindErrors(t *testing.T) {
	a := New(Config{Logger: quiet()})
	forbid := func(c *Ctx) error { return Errorf(403, "Origem não permitida") }
	a.Register(Route{Pattern: "/auto", Methods: map[string]HandlerFunc{"GET": forbid}})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{"GET": forbid}})
	a.Register(Route{Pattern: "/forced-api", Kind: KindAPI, Methods: map[string]HandlerFunc{"GET": forbid}})
	a.Register(Route{Pattern: "/forced-page", Kind: KindPage, Methods: map[string]HandlerFunc{"GET": forbid, "POST": func(c *Ctx) error { return c.Text(200, "ok") }}})
	do := func(method, p, accept string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, p, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		a.Handler().ServeHTTP(rec, req)
		return rec
	}
	html := "text/html,application/xhtml+xml,*/*;q=0.8"
	if rec := do("GET", "/auto", html); rec.Code != 403 || !strings.Contains(rec.Body.String(), "<html") || !strings.Contains(rec.Body.String(), "Origem não permitida") {
		t.Fatalf("browser must get HTML: %d %q", rec.Code, rec.Body.String())
	}
	for _, accept := range []string{"", "*/*", "application/json", "text/html, application/json"} {
		if rec := do("GET", "/auto", accept); rec.Code != 403 || !strings.HasPrefix(rec.Body.String(), `{"error"`) {
			t.Fatalf("accept %q must get JSON: %q", accept, rec.Body.String())
		}
	}
	if rec := do("GET", "/api/x", html); !strings.HasPrefix(rec.Body.String(), `{"error"`) {
		t.Fatal("/api/ stays JSON:", rec.Body.String())
	}
	if rec := do("GET", "/forced-api", html); !strings.HasPrefix(rec.Body.String(), `{"error"`) {
		t.Fatal("KindAPI stays JSON:", rec.Body.String())
	}
	if rec := do("GET", "/forced-page", "*/*"); !strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("KindPage renders HTML:", rec.Body.String())
	}
	if rec := do("POST", "/forced-page", ""); rec.Code != 403 || !strings.Contains(rec.Body.String(), "CSRF") {
		t.Fatalf("KindPage must enforce CSRF: %d %q", rec.Code, rec.Body.String())
	}
	// 405 follows the same rule.
	if rec := do("POST", "/auto", html); rec.Code != 405 || !strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("405 for browser must be HTML: %d %q", rec.Code, rec.Body.String())
	}
}

// #13 — shutdown hooks run in reverse order and errors are joined.
func TestOnShutdown(t *testing.T) {
	a := New(Config{Logger: quiet()})
	var order []string
	a.OnShutdown(func(*App) error { order = append(order, "pool"); return nil })
	a.OnShutdown(func(*App) error { order = append(order, "fila"); return errFake })
	err := a.runShutdown()
	if strings.Join(order, ",") != "fila,pool" || err == nil || err.Error() != "fake" {
		t.Fatal(order, err)
	}
	if or(Timeouts{}.Shutdown, 5) != 5 || or(Timeouts{Shutdown: 7}.Shutdown, 5) != 7 {
		t.Fatal("shutdown timeout default")
	}
}
