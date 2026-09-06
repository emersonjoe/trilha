package trilha

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

func TestExport(t *testing.T) {
	pub := fstest.MapFS{"style.css": {Data: []byte("body{}")}, "img/a.svg": {Data: []byte("<svg/>")}}
	a := testApp(Dev, pub) // Dev on purpose: export must not inject the dev script
	a.SetNotFound(func(c *Ctx) (h.Node, error) { return h.P(h.Text("perdido")), nil })
	a.AddExportPath("/blog/ola", "/docs/guia/x")
	dir := filepath.Join(t.TempDir(), "out")
	if err := a.Export(dir); err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		return string(b)
	}
	if got := read("index.html"); !strings.Contains(got, "Início") || strings.Contains(got, "_trilha/events") {
		t.Fatal(got)
	}
	if !strings.Contains(read("blog/novo/index.html"), "static wins") {
		t.Fatal("static route missing")
	}
	if !strings.Contains(read("blog/ola/index.html"), "post ola") {
		t.Fatal("extra path missing")
	}
	if !strings.Contains(read("docs/guia/x/index.html"), "docs:guia/x") {
		t.Fatal("catch-all extra path missing")
	}
	if !strings.Contains(read("404.html"), "perdido") {
		t.Fatal("404.html")
	}
	if read("style.css") != "body{}" || read("img/a.svg") != "<svg/>" {
		t.Fatal("public not copied")
	}
	if _, err := os.Stat(filepath.Join(dir, "api")); err == nil {
		t.Fatal("api routes must not be exported")
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "blog")); len(entries) != 2 { // novo, ola
		t.Fatalf("dynamic pattern must be skipped: %v", entries)
	}
	if got := strings.Join(a.ExportPaths(), " "); got != "/ /blog/novo /blog/ola /docs/guia/x" {
		t.Fatal(got)
	}
	// Re-export into the same dir works (marker present) and removes stale files.
	_ = os.WriteFile(filepath.Join(dir, "velho.html"), []byte("x"), 0o644)
	if err := a.Export(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "velho.html")); err == nil {
		t.Fatal("stale file should be removed")
	}
}

func TestExportRefusesForeignDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "importante.txt"), []byte("x"), 0o644)
	if err := testApp(Prod, nil).Export(dir); err == nil || !strings.Contains(err.Error(), "was not created by trilha") {
		t.Fatal(err)
	}
}

func TestExportFailsOnErrorRoute(t *testing.T) {
	a := testApp(Prod, nil)
	a.AddExportPath("/blog/boom")
	if err := a.Export(filepath.Join(t.TempDir(), "o")); err == nil || !strings.Contains(err.Error(), "/blog/boom") {
		t.Fatal(err)
	}
	a = testApp(Prod, nil)
	a.AddExportPath("/blog/novo/")
	if err := a.Export(filepath.Join(t.TempDir(), "o")); err == nil || !strings.Contains(err.Error(), "redirects to") {
		t.Fatal(err)
	}
}

func TestBasePath(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "/docs/")
	cfg := ConfigFromEnv()
	if cfg.BasePath != "/docs" {
		t.Fatal(cfg.BasePath)
	}
	a := New(cfg)
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) { return h.A(h.Href(c.Base() + "/x")), nil }})
	if rec := get(t, a, "GET", "/", "", nil); !strings.Contains(rec.Body.String(), `href="/docs/x"`) {
		t.Fatal(rec.Body.String())
	}
}

// Spec 015: an exported path that redirects inside the site becomes a small
// HTML stub (meta refresh + canonical); redirects to another origin still fail.
func TestExportRedirectStub(t *testing.T) {
	a := testApp(Prod, nil)
	a.Register(Route{Pattern: "/antigo", Page: func(c *Ctx) (h.Node, error) {
		return nil, RedirectCode("/blog/novo", 301)
	}})
	dir := filepath.Join(t.TempDir(), "o")
	if err := a.Export(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "antigo", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`<meta http-equiv="refresh" content="0; url=/blog/novo">`, `<link rel="canonical" href="/blog/novo">`, `<a href="/blog/novo">`} {
		if !strings.Contains(got, want) {
			t.Errorf("stub misses %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "<script") {
		t.Fatal("stub must not use JavaScript")
	}

	a = testApp(Prod, nil)
	a.Register(Route{Pattern: "/fora", Page: func(c *Ctx) (h.Node, error) {
		return nil, RedirectCode("https://example.com/", 301)
	}})
	if err := a.Export(filepath.Join(t.TempDir(), "o")); err == nil || !strings.Contains(err.Error(), "/fora") {
		t.Fatalf("external redirect must fail the export: %v", err)
	}
}

// Issue #26: uma rota que declara ETag continua exportável. O export não manda
// If-None-Match, então nada pode responder 304 — e um 304 no export escreveria
// um arquivo vazio no lugar da página.
func TestExportWithETag(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet()})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		if c.ETag("v1") {
			return nil, nil
		}
		return h.P(h.Text("conteúdo exportado")), nil
	}})
	dir := filepath.Join(t.TempDir(), "out")
	if err := a.Export(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "conteúdo exportado") {
		t.Fatalf("página vazia: %q", string(b))
	}
}
