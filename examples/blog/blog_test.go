package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

type client struct {
	t   *testing.T
	h   http.Handler
	jar map[string]string
}

func newClient(t *testing.T, env string) *client {
	t.Setenv("TRILHA_ENV", env)
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	posts.Seed()
	return &client{t: t, h: newApp().Handler(), jar: map[string]string{}}
}

func (c *client) do(method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	for k, v := range c.jar {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.MaxAge < 0 {
			delete(c.jar, ck.Name)
		} else {
			c.jar[ck.Name] = ck.Value
		}
	}
	return rec
}

func (c *client) get(path string) *httptest.ResponseRecorder { return c.do("GET", path, "", nil) }

func (c *client) postForm(path, form string) *httptest.ResponseRecorder {
	c.t.Helper()
	if tok, ok := c.jar[trilha.CSRFCookie]; ok {
		form += "&_csrf=" + tok
	}
	return c.do("POST", path, form, map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
}

func wantContains(t *testing.T, rec *httptest.ResponseRecorder, code int, parts ...string) {
	t.Helper()
	if rec.Code != code {
		t.Fatalf("status %d want %d\n%s", rec.Code, code, rec.Body.String())
	}
	for _, p := range parts {
		if !strings.Contains(rec.Body.String(), p) {
			t.Fatalf("body lacks %q:\n%s", p, rec.Body.String())
		}
	}
}

// ---- US1: páginas por arquivo ---------------------------------------------

func TestUS1_HomeInsideRootLayout(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/")
	wantContains(t, rec, 200, "<!doctype html><html lang=\"pt-BR\">", "<title>Início · Trilha Blog</title>", `<main id="conteudo"><h1>Trilha</h1>`, `href="/style.css"`)
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" || rec.Header().Get("Server-Timing") == "" {
		t.Fatal(rec.Header())
	}
	if strings.Contains(rec.Body.String(), "_trilha/events") {
		t.Fatal("no dev script in prod")
	}
}

func TestUS1_DynamicSegmentWithNestedLayouts(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/blog/ola-trilha")
	wantContains(t, rec, 200, "<title>Olá, Trilha · Trilha Blog</title>", `<main id="conteudo"><section class="blog"><aside>`, `<article><h1>Olá, Trilha</h1>`)
	wantContains(t, c.get("/blog"), 200, `<ul class="posts">`, `href="/blog/layouts"`)
}

func TestUS1_CatchAll(t *testing.T) {
	c := newClient(t, "prod")
	wantContains(t, c.get("/docs/guia/rotas/dinamicas"), 200, "<li><code>guia</code></li><li><code>rotas</code></li><li><code>dinamicas</code></li>", "<title>Docs: dinamicas")
}

func TestUS1_NotFoundPageInsideLayout(t *testing.T) {
	c := newClient(t, "prod")
	wantContains(t, c.get("/nada"), 404, "<title>Não encontrado · Trilha Blog</title>", "Nada em /nada.")
	wantContains(t, c.get("/blog/inexistente"), 404, "<h1>404</h1>")
}

func TestUS1_ErrorPageStackOnlyInDev(t *testing.T) {
	dev := newClient(t, "dev")
	posts.Create("boom", "x")
	wantContains(t, dev.get("/blog/boom"), 500, "<h1>Algo deu errado</h1>", "post explodiu", "_trilha/events")
	prod := newClient(t, "prod")
	posts.Create("boom", "x")
	rec := prod.get("/blog/boom")
	wantContains(t, rec, 500, "<h1>Algo deu errado</h1>")
	if strings.Contains(rec.Body.String(), "post explodiu") {
		t.Fatal("prod leaked the error")
	}
}

func TestUS1_StaticRouteBeatsDynamic(t *testing.T) {
	c := newClient(t, "prod")
	wantContains(t, c.get("/blog/novo"), 200, "<h1>Novo post</h1>")
}

// ---- US2: API e formulários -----------------------------------------------

func TestUS2_APIGetPostDelete(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/api/posts")
	wantContains(t, rec, 200, `"slug":"layouts"`)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatal(rec.Header().Get("Content-Type"))
	}
	rec = c.do("POST", "/api/posts", `{"title":"Via API","body":"b"}`, map[string]string{"Content-Type": "application/json"})
	wantContains(t, rec, 201, `"slug":"via-api"`)
	if rec.Header().Get("Location") != "/api/posts/via-api" {
		t.Fatal(rec.Header().Get("Location"))
	}
	wantContains(t, c.get("/api/posts/via-api"), 200, `"title":"Via API"`)
	wantContains(t, c.do("POST", "/api/posts", `{"title":""}`, nil), 422, "title is required")
	wantContains(t, c.do("POST", "/api/posts", `{"nope":1}`, nil), 400, "invalid JSON")
	if rec := c.do("DELETE", "/api/posts/via-api", "", nil); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	wantContains(t, c.do("DELETE", "/api/posts/via-api", "", nil), 404, `"error":"Not Found"`)
	wantContains(t, c.get("/api/nada"), 404, `"status":404`)
}

func TestUS2_MethodNotAllowed(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.do("PUT", "/api/posts", "", nil)
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestUS2_FormPostRedirectGetWithCSRF(t *testing.T) {
	c := newClient(t, "prod")
	if rec := c.postForm("/blog/novo", "titulo=Sem+token"); rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	if _, ok := posts.Get("sem-token"); ok {
		t.Fatal("handler ran without CSRF")
	}
	wantContains(t, c.get("/blog/novo"), 200, `name="_csrf"`)
	rec := c.postForm("/blog/novo", "titulo=Meu+Post&corpo=Texto")
	if rec.Code != 303 || rec.Header().Get("Location") != "/blog/meu-post" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	wantContains(t, c.get("/blog/meu-post"), 200, "<h1>Meu Post</h1>", "<p>Texto</p>")
	rec = c.do("POST", "/blog/novo", "titulo=x&_csrf=errado", map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	if rec := c.postForm("/blog/novo", "titulo=+"); rec.Code != 303 || !strings.Contains(rec.Header().Get("Location"), "erro=") {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := c.postForm("/blog/meu-post", ""); rec.Code != 303 || rec.Header().Get("Location") != "/blog" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := c.get("/blog/meu-post"); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}

func TestUS2_BodyTooLarge(t *testing.T) {
	c := newClient(t, "prod")
	big := `{"title":"` + strings.Repeat("x", 2<<20) + `"}`
	if rec := c.do("POST", "/api/posts", big, nil); rec.Code != 413 {
		t.Fatal(rec.Code)
	}
}

// ---- US3: middleware --------------------------------------------------------

func TestUS3_AdminGuard(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/admin")
	if rec.Code != 302 || rec.Header().Get("Location") != "/login?next=/admin" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("Server-Timing") == "" {
		t.Fatal("root middleware should still run")
	}
	wantContains(t, c.get("/login"), 200, `name="_csrf"`)
	if rec := c.postForm("/login", "usuario=admin&senha=errada"); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	rec = c.postForm("/login", "usuario=admin&senha=trilha&next=/admin")
	if rec.Code != 303 || rec.Header().Get("Location") != "/admin" || !strings.Contains(c.jar["sessao"], "|") {
		t.Fatalf("%d %s %v", rec.Code, rec.Header().Get("Location"), c.jar)
	}
	wantContains(t, c.get("/admin"), 200, "<h1>Olá, admin</h1>", "2 posts publicados.")
	if rec := c.postForm("/login", "sair=1"); rec.Code != 303 {
		t.Fatal(rec.Code)
	}
	if rec := c.get("/admin"); rec.Code != 302 {
		t.Fatal("logout should revoke access")
	}
}

func TestSetupSeededValues(t *testing.T) {
	c := newClient(t, "prod")
	var list []posts.Post
	_ = json.Unmarshal(c.get("/api/posts").Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatal(len(list))
	}
}

func TestPublicAndTraversal(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/style.css")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatal("Config() in setup.go not applied:", cc)
	}
	req := httptest.NewRequest("GET", "/x", nil)
	req.URL.Path = "/../go.mod"
	rec = httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	if rec.Code == 200 {
		t.Fatal("traversal")
	}
}

func TestTrailingSlash(t *testing.T) {
	c := newClient(t, "prod")
	if rec := c.get("/blog/"); rec.Code != 301 || rec.Header().Get("Location") != "/blog" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
}

// ---- 002: grupos de rota ---------------------------------------------------

func TestGroups_LayoutWithoutURLSegment(t *testing.T) {
	c := newClient(t, "prod")
	wantContains(t, c.get("/precos"), 200, `<main id="conteudo"><section class="marketing"><nav class="sub">`, "<h1>Preços</h1>", "<title>Preços · Trilha Blog</title>")
	wantContains(t, c.get("/sobre"), 200, `<section class="marketing">`, "<h1>Sobre</h1>")
	if rec := c.get("/marketing-/precos"); rec.Code != 404 {
		t.Fatalf("group name must not be a URL segment: %d", rec.Code)
	}
}

func TestGroups_MiddlewareOrder(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/painel")
	wantContains(t, rec, 200, `<section class="app" data-area="painel">`, "<h1>Painel</h1>")
	if rec.Header().Get("X-Area") != "painel" || rec.Header().Get("Server-Timing") == "" {
		t.Fatalf("root and group middlewares must both run: %v", rec.Header())
	}
	if rec := c.get("/precos"); rec.Header().Get("X-Area") != "" {
		t.Fatal("painel middleware leaked into marketing group")
	}
}

// ---- 002: html/template ---------------------------------------------------

func TestTemplatePageInsideLayouts(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/relatorio")
	wantContains(t, rec, 200, `<section class="app" data-area="painel">`, "<h1>Relatório &lt;de posts&gt;</h1>", `<a href="/blog/layouts">Layouts aninhados</a>`, "<title>Relatório · Trilha Blog</title>")
}

func TestTemplateErrorIs500(t *testing.T) {
	c := newClient(t, "dev")
	wantContains(t, c.get("/relatorio?t=nao-existe"), 500, "<h1>Algo deu errado</h1>", "nao-existe")
}

// ---- 004: segurança -------------------------------------------------------

func TestSecurityHeadersOnExample(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.get("/")
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "img-src 'self' data: https:") || rec.Header().Get("Permissions-Policy") == "" {
		t.Fatalf("csp=%q", csp)
	}
}

func TestSignedSessionCannotBeForged(t *testing.T) {
	c := newClient(t, "prod")
	c.jar["sessao"] = "admin|9999999999|assinatura-falsa"
	if rec := c.get("/admin"); rec.Code != 302 {
		t.Fatalf("forged session accepted: %d", rec.Code)
	}
}

func TestAPIRateLimit(t *testing.T) {
	c := newClient(t, "prod")
	var last int
	for i := 0; i < 25; i++ {
		last = c.get("/api/posts").Code
	}
	if last != 429 {
		t.Fatalf("expected 429 after burst, got %d", last)
	}
}
