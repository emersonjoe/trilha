package dev

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/scan"
)

func inspetor(t *testing.T, caminho string) string {
	t.Helper()
	raiz := filepath.Join("..", "..", "testdata", "apps", "full")
	b, err := renderInspector(raiz, "example.com/full", caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func rotasDe(t *testing.T) []scan.Route {
	t.Helper()
	res, err := scan.Scan(filepath.Join("..", "..", "testdata", "apps", "full"), "example.com/full")
	if err != nil {
		t.Fatal(err)
	}
	return res.Routes
}

func TestInspetorLista(t *testing.T) {
	html := inspetor(t, "")
	// Cada rota, com o que só o scanner sabe: origem, layouts e middlewares.
	for _, s := range []string{
		"/blog/{slug}", "/docs/{path...}", "/api/posts/{id}", "/admin",
		"app/blog/slug_/page.go", "app/api/posts/id_/route.go",
		"app/layout.go", "app/blog/layout.go",
		"app/middleware.go", "app/admin/middleware.go",
		"app/not_found.go", "app/error.go", "app/setup.go",
		"page", "api", "GET",
	} {
		if !strings.Contains(html, s) {
			t.Errorf("a página não menciona %q", s)
		}
	}
	// Precedência: a literal aparece antes da dinâmica que ela sombreia.
	if strings.Index(html, "/blog/novo") > strings.Index(html, "/blog/{slug}") {
		t.Error("a rota literal tem de vir antes da dinâmica")
	}
	// Nada de fora, e nada de script: a página é HTML e CSS embutidos.
	for _, s := range []string{"<script", "http://", "https://"} {
		if strings.Contains(html, s) {
			t.Errorf("a página carrega algo de fora: %q", s)
		}
	}
}

func TestInspetorCasamento(t *testing.T) {
	rotas := rotasDe(t)
	casos := []struct {
		caminho string
		padrao  string
		params  string
	}{
		{"/blog/novo", "/blog/novo", ""},
		{"/blog/ola-mundo", "/blog/{slug}", "slug=ola-mundo"},
		{"/docs/a/b", "/docs/{path...}", "path=a/b"},
		{"/api/posts/7", "/api/posts/{id}", "id=7"},
		{"/", "/", ""},
		{"/nao-existe", "", ""},
	}
	for _, c := range casos {
		got := matchPath(rotas, c.caminho)
		if got.Pattern != c.padrao {
			t.Errorf("%s: padrão = %q, queria %q", c.caminho, got.Pattern, c.padrao)
		}
		var ps []string
		for _, p := range got.Params {
			ps = append(ps, p.Name+"="+p.Value)
		}
		if strings.Join(ps, ",") != c.params {
			t.Errorf("%s: params = %v, queria %q", c.caminho, ps, c.params)
		}
	}
}

func TestInspetorEscapa(t *testing.T) {
	html := inspetor(t, "/<script>alert(1)</script>")
	if strings.Contains(html, "<script>") {
		t.Fatal("o caminho da URL voltou sem escapar")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("o caminho pedido tem de aparecer na resposta, escapado")
	}
}
