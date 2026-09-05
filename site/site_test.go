package main

import (
	"github.com/emersonjoe/trilha/site/internal/ui"
	"io"
	"log/slog"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/site/internal/docs"
)

func get(t *testing.T, path string) (int, string) {
	t.Helper()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	newApp().Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec.Code, rec.Body.String()
}

func TestEveryPageResponds(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	paths := append([]string{"/"}, pagePaths()...)
	for _, p := range paths {
		code, body := get(t, p)
		if code != 200 {
			t.Errorf("%s → %d", p, code)
		}
		if !strings.Contains(body, "<title>") || !strings.Contains(body, `href="/site.css"`) {
			t.Errorf("%s: missing shell", p)
		}
	}
	if code, _ := get(t, "/aprender/nao-existe"); code != 404 {
		t.Errorf("unknown chapter → %d", code)
	}
}

func pagePaths() []string {
	var out []string
	for _, p := range docs.All() {
		out = append(out, p.Path())
	}
	return out
}

func TestChaptersHaveChallengeAndSolution(t *testing.T) {
	for _, p := range docs.All() {
		if p.Section != "aprender" || p.Slug == "problemas-comuns" {
			continue
		}
		_, body := get(t, p.Path())
		if !strings.Contains(body, `id="desafio"`) || !strings.Contains(body, `<details class="solucao">`) {
			t.Errorf("%s: chapter without Desafio + solução", p.Path())
		}
	}
}

func TestInternalLinksResolve(t *testing.T) {
	known := map[string]bool{"/": true}
	for _, p := range pagePaths() {
		known[p] = true
	}
	re := regexp.MustCompile(`href="(/[^"#]*)`)
	for _, p := range append([]string{"/"}, pagePaths()...) {
		_, body := get(t, p)
		for _, m := range re.FindAllStringSubmatch(body, -1) {
			href := m[1]
			if strings.HasSuffix(href, ".css") || strings.HasSuffix(href, ".js") || strings.HasSuffix(href, ".svg") {
				continue
			}
			if !known[href] {
				t.Errorf("%s links to unknown %s", p, href)
			}
		}
	}
}

func TestBasePathPrefixesLinks(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "/trilha")
	_, body := get(t, "/aprender")
	if !strings.Contains(body, `href="/trilha/aprender/paginas-e-rotas"`) || !strings.Contains(body, `href="/trilha/site.css"`) {
		t.Fatal("links must carry the base path")
	}
	if strings.Contains(body, `href="/aprender/`) {
		t.Fatal("unprefixed link found")
	}
}

func TestExportPathsCoverEveryPage(t *testing.T) {
	t.Setenv("TRILHA_BASE_PATH", "")
	a := newApp()
	got := strings.Join(a.ExportPaths(), " ")
	for _, p := range pagePaths() {
		if !strings.Contains(got+" ", p+" ") {
			t.Errorf("export misses %s", p)
		}
	}
}

func TestNoDevScriptInProd(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	if _, body := get(t, "/"); strings.Contains(body, "_trilha/events") {
		t.Fatal("dev script leaked")
	}
}

func TestAnalyticsOnlyWhenConfigured(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("SITE_ANALYTICS", "")
	rec := httptest.NewRecorder()
	newApp().Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(rec.Body.String(), "goatcounter") {
		t.Fatal("analytics must be off by default")
	}
	t.Setenv("SITE_ANALYTICS", "goatcounter:trilha")
	rec = httptest.NewRecorder()
	newApp().Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `<script data-goatcounter="https://trilha.goatcounter.com/count" async src="https://gc.zgo.to/count.js">`) || !strings.Contains(body, "sem cookies") {
		t.Fatal(body)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'nonce-") || !strings.Contains(csp, "https://gc.zgo.to") || !strings.Contains(csp, "connect-src 'self' https://trilha.goatcounter.com") {
		t.Fatal(csp)
	}
	for _, bad := range []string{"google:UA-1", "goatcounter:", "goatcounter:Bad Code", "x"} {
		if _, err := ui.ParseAnalytics(bad); err == nil {
			t.Fatal("must reject", bad)
		}
	}
}
