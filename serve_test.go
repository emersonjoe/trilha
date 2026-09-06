package trilha

import (
	"crypto/tls"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func rootLayout(c *Ctx, children h.Node) (h.Node, error) {
	return h.Html(h.Head(h.Title(h.Text(c.Title()))), h.Body(h.Div(h.ID("root"), children))), nil
}

func blogLayout(c *Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.ID("blog"), children), nil
}

func testApp(env Env, public fs.FS) *App {
	a := New(Config{Env: env, Logger: quiet(), Public: public})
	a.SetRootLayout(rootLayout)
	a.Register(Route{Pattern: "/", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		c.SetTitle("Home")
		return h.H1(h.Text("Início")), nil
	}})
	a.Register(Route{Pattern: "/blog/{slug}", Layouts: []LayoutFunc{blogLayout, rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		if c.Param("slug") == "missing" {
			return nil, ErrNotFound
		}
		if c.Param("slug") == "boom" {
			return nil, errors.New("kaboom")
		}
		if c.Param("slug") == "panic" {
			panic("ouch")
		}
		return h.P(h.Text("post " + c.Param("slug"))), nil
	}})
	a.Register(Route{Pattern: "/blog/novo", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		return h.Text("static wins"), nil
	}, Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		return c.Redirect("/blog/" + c.Form("slug"))
	}}})
	a.Register(Route{Pattern: "/docs/{path...}", Page: func(c *Ctx) (h.Node, error) {
		return h.Text("docs:" + c.Param("path")), nil
	}})
	a.Register(Route{Pattern: "/api/posts", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.JSON(200, []string{"a"}) },
		"POST": func(c *Ctx) error {
			var in struct{ Title string }
			if err := c.BindJSON(&in); err != nil {
				return err
			}
			if in.Title == "" {
				return Errorf(422, "title required")
			}
			return c.JSON(201, in)
		},
		"DELETE": func(c *Ctx) error { return ErrNotFound },
	}})
	return a
}

func get(t *testing.T, a *App, method, path string, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func TestPageWithNestedLayouts(t *testing.T) {
	a := testApp(Prod, nil)
	rec := get(t, a, "GET", "/", "", nil)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(body, "<!doctype html><html>") || !strings.Contains(body, "<title>Home</title>") || !strings.Contains(body, `<div id="root"><h1>Início</h1></div>`) {
		t.Fatalf("%d %s", rec.Code, body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatal(ct)
	}
	if strings.Contains(body, "_trilha/events") {
		t.Fatal("dev script must not be injected in prod")
	}
	rec = get(t, a, "GET", "/blog/ola", "", nil)
	if !strings.Contains(rec.Body.String(), `<div id="root"><section id="blog"><p>post ola</p></section></div>`) {
		t.Fatal(rec.Body.String())
	}
}

func TestSecurityHeadersAndRequestID(t *testing.T) {
	rec := get(t, testApp(Prod, nil), "GET", "/", "", map[string]string{"X-Request-ID": "abc"})
	for k, v := range map[string]string{"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "X-Request-ID": "abc"} {
		if rec.Header().Get(k) != v {
			t.Errorf("%s=%q", k, rec.Header().Get(k))
		}
	}
}

func TestCatchAllAndStaticPrecedence(t *testing.T) {
	a := testApp(Prod, nil)
	if rec := get(t, a, "GET", "/docs/a/b/c", "", nil); !strings.Contains(rec.Body.String(), "docs:a/b/c") {
		t.Fatal(rec.Body.String())
	}
	if rec := get(t, a, "GET", "/blog/novo", "", nil); !strings.Contains(rec.Body.String(), "static wins") {
		t.Fatal(rec.Body.String())
	}
}

func TestNotFoundAndErrorPages(t *testing.T) {
	a := testApp(Dev, nil)
	rec := get(t, a, "GET", "/nada", "", nil)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "404 Not Found") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "_trilha/events") {
		t.Fatal("dev script expected")
	}
	rec = get(t, a, "GET", "/blog/missing", "", nil)
	if rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	rec = get(t, a, "GET", "/blog/boom", "", nil)
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatalf("dev should show error: %d %s", rec.Code, rec.Body.String())
	}
	rec = get(t, a, "GET", "/blog/panic", "", nil)
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "ouch") || !strings.Contains(rec.Body.String(), "serve_test.go") {
		t.Fatalf("dev should show panic stack: %d", rec.Code)
	}
	prod := testApp(Prod, nil)
	rec = get(t, prod, "GET", "/blog/boom", "", nil)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatalf("prod must hide error: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCustomNotFoundUsesRootLayout(t *testing.T) {
	a := testApp(Prod, nil)
	a.SetNotFound(func(c *Ctx) (h.Node, error) { c.SetTitle("Perdido"); return h.P(h.Text("custom 404")), nil })
	a.SetErrorPage(func(c *Ctx, err error) (h.Node, error) { return h.P(h.Text("custom 500")), nil })
	rec := get(t, a, "GET", "/nada", "", nil)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), `<title>Perdido</title>`) || !strings.Contains(rec.Body.String(), `<div id="root"><p>custom 404</p></div>`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = get(t, a, "GET", "/blog/boom", "", nil)
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "custom 500") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestAPIRoutes(t *testing.T) {
	a := testApp(Prod, nil)
	rec := get(t, a, "GET", "/api/posts", "", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Body.String() != "[\"a\"]\n" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = get(t, a, "PUT", "/api/posts", "", nil)
	if rec.Code != 405 || rec.Header().Get("Allow") != "DELETE, GET, POST" || !strings.Contains(rec.Body.String(), `"status":405`) {
		t.Fatalf("%d allow=%q %s", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
	}
	rec = get(t, a, "POST", "/api/posts", `{"Title":""}`, nil)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), "title required") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = get(t, a, "POST", "/api/posts", `{bad`, nil)
	if rec.Code != 400 {
		t.Fatal(rec.Code)
	}
	rec = get(t, a, "DELETE", "/api/posts", "", nil)
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), `"title":"Not Found"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	rec = get(t, a, "GET", "/api/nada", "", nil)
	if rec.Code != 404 || !strings.HasPrefix(rec.Header().Get("Content-Type"), ProblemMediaType) {
		t.Fatalf("api 404 should be problem+json: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestBodyLimit(t *testing.T) {
	a := New(Config{Logger: quiet(), MaxBodyBytes: 16})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		var v map[string]any
		if err := c.BindJSON(&v); err != nil {
			return err
		}
		return c.JSON(200, v)
	}}})
	rec := get(t, a, "POST", "/api/x", `{"a":"`+strings.Repeat("x", 100)+`"}`, nil)
	if rec.Code != 413 {
		t.Fatal(rec.Code)
	}
}

func TestMethodNotAllowedOnPage(t *testing.T) {
	a := testApp(Prod, nil)
	rec := get(t, a, "DELETE", "/", "", nil)
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Allow"))
	}
	if !strings.Contains(rec.Body.String(), "405") || strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatal(rec.Body.String())
	}
}

func TestTrailingSlashRedirect(t *testing.T) {
	a := testApp(Prod, nil)
	rec := get(t, a, "GET", "/blog/novo/?x=1", "", nil)
	if rec.Code != 301 || rec.Header().Get("Location") != "/blog/novo?x=1" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(t, a, "GET", "/", "", nil); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
}

func TestMiddlewareChain(t *testing.T) {
	var order []string
	mw := func(name string, short bool) MiddlewareFunc {
		return func(c *Ctx, next Next) error {
			order = append(order, name+">")
			c.Set(name, true)
			if short {
				return c.Text(200, "short")
			}
			err := next()
			order = append(order, "<"+name)
			return err
		}
	}
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/x", Middlewares: []MiddlewareFunc{mw("root", false), mw("sub", false)}, Page: func(c *Ctx) (h.Node, error) {
		order = append(order, "page")
		if c.Get("root") != true || c.Get("sub") != true {
			t.Error("values not propagated")
		}
		return h.Text("ok"), nil
	}})
	a.Register(Route{Pattern: "/y", Middlewares: []MiddlewareFunc{mw("root", false), mw("guard", true)}, Page: func(c *Ctx) (h.Node, error) {
		t.Error("handler must not run")
		return nil, nil
	}})
	a.Register(Route{Pattern: "/z", Middlewares: []MiddlewareFunc{func(c *Ctx, next Next) error { return Redirect("/login") }}, Page: func(c *Ctx) (h.Node, error) {
		t.Error("handler must not run")
		return nil, nil
	}})
	get(t, a, "GET", "/x", "", nil)
	if got := strings.Join(order, " "); got != "root> sub> page <sub <root" {
		t.Fatal(got)
	}
	order = nil
	if rec := get(t, a, "GET", "/y", "", nil); rec.Body.String() != "short" || strings.Join(order, " ") != "root> guard> <root" {
		t.Fatalf("%s %v", rec.Body.String(), order)
	}
	if rec := get(t, a, "GET", "/z", "", nil); rec.Code != 303 || rec.Header().Get("Location") != "/login" {
		t.Fatal(rec.Code)
	}
}

func TestStaticFiles(t *testing.T) {
	pub := fstest.MapFS{
		"style.css":    {Data: []byte("body{}")},
		"img/logo.svg": {Data: []byte("<svg/>")},
	}
	a := testApp(Prod, pub)
	rec := get(t, a, "GET", "/style.css", "", nil)
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") || rec.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("%d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"))
	}
	if rec := get(t, a, "GET", "/img/logo.svg", "", nil); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if rec := get(t, a, "GET", "/img", "", nil); rec.Code != 404 {
		t.Fatalf("dirs must not be listed: %d", rec.Code)
	}
	if rec := get(t, a, "GET", "/", "", nil); !strings.Contains(rec.Body.String(), "Início") {
		t.Fatal("routes must win over static")
	}
	req := httptest.NewRequest("GET", "/../go.mod", nil)
	req.URL.Path = "/../go.mod"
	rec = httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code == 200 {
		t.Fatal("path traversal")
	}
	dev := testApp(Dev, pub)
	if rec := get(t, dev, "GET", "/style.css", "", nil); rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatal(rec.Header().Get("Cache-Control"))
	}
}

func TestNoLayoutGetsMinimalDocument(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { return h.P(h.Text("solo")), nil }})
	rec := get(t, a, "GET", "/", "", nil)
	if b := rec.Body.String(); !strings.HasPrefix(b, "<!doctype html><html><head>") || !strings.Contains(b, "<body><p>solo</p></body>") {
		t.Fatal(b)
	}
}

func TestStatusFromPage(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { c.Status(201); return h.Text("x"), nil }})
	if rec := get(t, a, "GET", "/", "", nil); rec.Code != 201 {
		t.Fatal(rec.Code)
	}
}

func TestHandlerWritesNothing(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return nil }}})
	if rec := get(t, a, "GET", "/api/x", "", nil); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
}

func TestDevEventsEndpoint(t *testing.T) {
	a := testApp(Prod, nil)
	if rec := get(t, a, "GET", "/_trilha/events", "", nil); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}

func TestRoutesListing(t *testing.T) {
	a := testApp(Prod, nil)
	r := a.Routes()
	if got := strings.Join(r["/api/posts"], ","); got != "DELETE,GET,POST" {
		t.Fatal(got)
	}
	if got := strings.Join(r["/blog/novo"], ","); got != "GET,POST" {
		t.Fatal(got)
	}
}

var _ = http.StatusOK

var tlsState = tls.ConnectionState{}
