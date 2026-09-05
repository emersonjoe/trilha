// Package bench measures Trilha's overhead over the standard library. Run:
//
//	cd bench && go test -bench . -benchmem
//
// Numbers are per request, in-process (httptest.NewRecorder), so they show the
// framework cost only; a real app is dominated by I/O.
package bench

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

type post struct {
	Title, Body string
}

var posts = func() []post {
	out := make([]post, 20)
	for i := range out {
		out[i] = post{fmt.Sprintf("Post %d", i), strings.Repeat("Lorem ipsum dolor sit amet. ", 5)}
	}
	return out
}()

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// ---- page: h + layout vs html/template ------------------------------------

func layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Html(h.Lang("pt-BR"), h.Head(h.Meta(h.Charset("utf-8")), h.Title(h.Text(c.Title()))),
		h.Body(h.Header(h.Nav(h.A(h.Href("/"), h.Text("Início")), h.A(h.Href("/blog"), h.Text("Blog")))), h.Main(children))), nil
}

func page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Blog")
	return h.Fragment(h.H1(h.Text("Blog")), h.Ul(h.Map(posts, func(p post) h.Node {
		return h.Li(h.Article(h.H2(h.Text(p.Title)), h.P(h.Text(p.Body))))
	}))), nil
}

var tpl = template.Must(template.New("page").Parse(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>{{.Title}}</title></head><body><header><nav><a href="/">Início</a><a href="/blog">Blog</a></nav></header><main><h1>Blog</h1><ul>{{range .Posts}}<li><article><h2>{{.Title}}</h2><p>{{.Body}}</p></article></li>{{end}}</ul></main></body></html>`))

func trilhaApp(routes int) *trilha.App {
	a := trilha.New(trilha.Config{Logger: quiet(), Env: trilha.Prod, Public: fstest.MapFS{"app.css": {Data: []byte(strings.Repeat("body{margin:0}", 100))}}})
	a.Register(trilha.Route{Pattern: "/blog", Page: page, Layouts: []trilha.LayoutFunc{layout}})
	a.Register(trilha.Route{Pattern: "/api/posts", Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.JSON(200, posts) }}})
	a.Register(trilha.Route{Pattern: "/api/posts/{id}", Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.Text(200, c.Param("id")) }}})
	for i := 0; i < routes; i++ {
		a.Register(trilha.Route{Pattern: fmt.Sprintf("/r%d/{id}", i), Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.Text(200, c.Param("id")) }}})
	}
	mw := func(c *trilha.Ctx, next trilha.Next) error { c.Set("k", 1); return next() }
	a.Register(trilha.Route{Pattern: "/mw", Middlewares: []trilha.MiddlewareFunc{mw, mw, mw, mw, mw}, Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.Text(200, "ok") }}})
	return a
}

func stdlibMux(routes int) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.Execute(w, map[string]any{"Title": "Blog", "Posts": posts})
	})
	mux.HandleFunc("GET /api/posts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(posts)
	})
	mux.HandleFunc("GET /api/posts/{id}", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, r.PathValue("id")) })
	for i := 0; i < routes; i++ {
		mux.HandleFunc(fmt.Sprintf("GET /r%d/{id}", i), func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, r.PathValue("id")) })
	}
	mux.Handle("GET /app.css", http.FileServerFS(fstest.MapFS{"app.css": {Data: []byte(strings.Repeat("body{margin:0}", 100))}}))
	h := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	for i := 0; i < 5; i++ {
		next := h
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
	}
	mux.Handle("GET /mw", h)
	return mux
}

func run(b *testing.B, hnd http.Handler, path string) {
	b.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	hnd.ServeHTTP(rec, req)
	if rec.Code != 200 {
		b.Fatalf("%s: %d", path, rec.Code)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		hnd.ServeHTTP(rec, req)
	}
}

func BenchmarkPage_Trilha(b *testing.B)   { run(b, trilhaApp(0).Handler(), "/blog") }
func BenchmarkPage_Stdlib(b *testing.B)   { run(b, stdlibMux(0), "/blog") }
func BenchmarkJSON_Trilha(b *testing.B)   { run(b, trilhaApp(0).Handler(), "/api/posts") }
func BenchmarkJSON_Stdlib(b *testing.B)   { run(b, stdlibMux(0), "/api/posts") }
func BenchmarkStatic_Trilha(b *testing.B) { run(b, trilhaApp(0).Handler(), "/app.css") }
func BenchmarkStatic_Stdlib(b *testing.B) { run(b, stdlibMux(0), "/app.css") }
func BenchmarkRoute200_Trilha(b *testing.B) {
	run(b, trilhaApp(200).Handler(), "/r150/abc")
}
func BenchmarkRoute200_Stdlib(b *testing.B) { run(b, stdlibMux(200), "/r150/abc") }
func BenchmarkMiddleware5_Trilha(b *testing.B) {
	run(b, trilhaApp(0).Handler(), "/mw")
}
func BenchmarkMiddleware5_Stdlib(b *testing.B) { run(b, stdlibMux(0), "/mw") }

func TestBenchmarksRespond(t *testing.T) {
	for _, p := range []string{"/blog", "/api/posts", "/app.css", "/r5/x", "/mw"} {
		for name, hnd := range map[string]http.Handler{"trilha": trilhaApp(10).Handler(), "stdlib": stdlibMux(10)} {
			rec := httptest.NewRecorder()
			hnd.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
			if rec.Code != 200 {
				t.Fatal(name, p, rec.Code)
			}
		}
	}
}
