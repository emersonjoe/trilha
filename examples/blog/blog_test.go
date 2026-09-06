package main

import (
	"encoding/json"
	"io"
	"log/slog"
	multipartlib "mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

type client struct {
	*trilha.TestClient
	app *trilha.App
}

func newClient(t *testing.T, env string) *client {
	t.Setenv("TRILHA_ENV", env)
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Os posts nascem no Setup, dentro do app: cada cliente de teste tem o
	// seu store, e nenhum teste depende da ordem em que os outros rodaram.
	a := newApp()
	return &client{trilha.NewTestClient(t, a), a}
}

// posts devolve o store deste app — o mesmo que as páginas recebem com
// trilha.Use, alcançado aqui pelo app em vez de por uma requisição.
func (c *client) posts() *posts.Store { return trilha.Use[*posts.Store](c.app) }

// postForm manda o formulário no formato cru dos testes daqui; o token do CSRF
// vai junto sozinho, como no navegador.
func (c *client) postForm(path, form string, opts ...trilha.TestOption) *trilha.TestResponse {
	return c.Request("POST", path, append([]trilha.TestOption{
		trilha.WithBody("application/x-www-form-urlencoded", form)}, opts...)...)
}

// #17 — o ícone mora em internal/icones e é servido em /icones/ por
// Config.Mounts; public/ continua servindo a raiz.
func TestMontagemServeArvoreForaDePublic(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/icones/favicon.svg")
	rec.WantStatus(200).WantContains("<svg")
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "image/svg") {
		t.Errorf("content-type %q", ct)
	}
	if rec := c.Get("/style.css"); rec.Code != 200 {
		t.Errorf("public/ continua na raiz: %d", rec.Code)
	}
	if rec := c.Get("/icones/nao-existe.svg"); rec.Code != 404 {
		t.Errorf("arquivo ausente na montagem: %d", rec.Code)
	}
}

// ---- US1: páginas por arquivo ---------------------------------------------

func TestUS1_HomeInsideRootLayout(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/")
	rec.WantStatus(200).WantContains("<!doctype html><html lang=\"pt-BR\">", "<title>Início · Trilha Blog</title>", `<h1 class="ui-h1">Trilha</h1>`, `href="/ui.css?v=`, `href="/style.css?v=`)
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" || rec.Header().Get("Server-Timing") == "" {
		t.Fatal(rec.Header())
	}
	if strings.Contains(rec.Body.String(), "_trilha/events") {
		t.Fatal("no dev script in prod")
	}
}

func TestUS1_DynamicSegmentWithNestedLayouts(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/blog/ola-trilha")
	rec.WantStatus(200).WantContains("<title>Olá, Trilha · Trilha Blog</title>", `<main id="conteudo"><div class="ui-container"><section class="blog"><aside>`, `<h1 class="ui-h1">Olá, Trilha</h1>`)
	c.Get("/blog").WantStatus(200).WantContains(`<ul class="posts">`, `href="/blog/layouts"`)
}

func TestUS1_CatchAll(t *testing.T) {
	c := newClient(t, "prod")
	c.Get("/docs/guia/rotas/dinamicas").WantStatus(200).WantContains("<li><code>guia</code></li><li><code>rotas</code></li><li><code>dinamicas</code></li>", "<title>Docs: dinamicas")
}

func TestUS1_NotFoundPageInsideLayout(t *testing.T) {
	c := newClient(t, "prod")
	c.Get("/nada").WantStatus(404).WantContains("<title>Não encontrado · Trilha Blog</title>", "Nada em /nada.")
	c.Get("/blog/inexistente").WantStatus(404).WantContains("<h1>404</h1>")
}

func TestUS1_ErrorPageStackOnlyInDev(t *testing.T) {
	dev := newClient(t, "dev")
	dev.posts().Create("boom", "x")
	dev.Get("/blog/boom").WantStatus(500).WantContains("<h1>Algo deu errado</h1>", "post explodiu", "_trilha/events")
	prod := newClient(t, "prod")
	prod.posts().Create("boom", "x")
	rec := prod.Get("/blog/boom")
	rec.WantStatus(500).WantContains("<h1>Algo deu errado</h1>")
	if strings.Contains(rec.Body.String(), "post explodiu") {
		t.Fatal("prod leaked the error")
	}
}

func TestUS1_StaticRouteBeatsDynamic(t *testing.T) {
	c := newClient(t, "prod")
	c.Get("/blog/novo").WantStatus(200).WantContains(`<h1 class="ui-card-title">Novo post</h1>`)
}

// ---- US2: API e formulários -----------------------------------------------

func TestUS2_APIGetPostDelete(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/api/posts")
	rec.WantStatus(200).WantContains(`"slug":"layouts"`)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatal(rec.Header().Get("Content-Type"))
	}
	rec = c.PostJSON("/api/posts", map[string]string{"title": "Via API", "body": "b"})
	rec.WantStatus(201).WantContains(`"slug":"via-api"`)
	if rec.Header().Get("Location") != "/api/posts/via-api" {
		t.Fatal(rec.Header().Get("Location"))
	}
	c.Get("/api/posts/via-api").WantStatus(200).WantContains(`"title":"Via API"`)
	c.Request("POST", "/api/posts", trilha.WithBody("", `{"title":""}`)).WantStatus(422).WantContains(`"title":"obrigatório"`)
	c.Request("POST", "/api/posts", trilha.WithBody("", `{"nope":1}`)).WantStatus(400).WantContains("invalid JSON")
	if rec := c.Request("DELETE", "/api/posts/via-api"); rec.Code != 204 {
		t.Fatal(rec.Code)
	}
	c.Request("DELETE", "/api/posts/via-api").WantStatus(404).WantContains(`"title":"Not Found"`)
	c.Get("/api/nada").WantStatus(404).WantContains(`"status":404`)
}

func TestUS2_MethodNotAllowed(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Request("PUT", "/api/posts")
	if rec.Code != 405 || rec.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestUS2_FormPostRedirectGetWithCSRF(t *testing.T) {
	c := newClient(t, "prod")
	if rec := c.postForm("/blog/novo", "titulo=Sem+token", trilha.WithoutCSRF()); rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	if _, ok := c.posts().Get("sem-token"); ok {
		t.Fatal("handler ran without CSRF")
	}
	c.Get("/blog/novo").WantStatus(200).WantContains(`name="_csrf"`)
	rec := c.postForm("/blog/novo", "titulo=Meu+Post&corpo=Texto")
	if rec.Code != 303 || rec.Header().Get("Location") != "/blog/meu-post" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	c.Get("/blog/meu-post").WantStatus(200).WantContains(`<h1 class="ui-h1">Meu Post</h1>`, "<p>Texto</p>")
	// Cookie de um lado, campo de outro: o par não bate e a rota recusa.
	rec = c.postForm("/blog/novo", "titulo=x&_csrf=errado",
		trilha.WithoutCSRF(), trilha.WithCookie(trilha.CSRFCookie, strings.Repeat("t", 32)))
	if rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	// Issue #27: o formulário volta com a mensagem ao lado do campo, no lugar
	// de um redirecionamento carregando o erro na URL.
	semTitulo := c.postForm("/blog/novo", "titulo=+&corpo=Texto")
	semTitulo.WantStatus(422).WantContains(`role="alert">obrigatório<`, `aria-invalid="true"`, `name="titulo"`)
	curto := c.postForm("/blog/novo", "titulo=ab&corpo=Texto")
	curto.WantStatus(422).WantContains("precisa ter ao menos 3 caracteres", `value="ab"`)
	if rec := c.postForm("/blog/meu-post", ""); rec.Code != 303 || rec.Header().Get("Location") != "/blog" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := c.Get("/blog/meu-post"); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}

func TestUS2_BodyTooLarge(t *testing.T) {
	c := newClient(t, "prod")
	big := `{"title":"` + strings.Repeat("x", 2<<20) + `"}`
	if rec := c.Request("POST", "/api/posts", trilha.WithBody("", big)); rec.Code != 413 {
		t.Fatal(rec.Code)
	}
}

// ---- US3: middleware --------------------------------------------------------

func TestUS3_AdminGuard(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/admin")
	if rec.Code != 302 || rec.Header().Get("Location") != "/login?next=/admin" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("Server-Timing") == "" {
		t.Fatal("root middleware should still run")
	}
	c.Get("/login").WantStatus(200).WantContains(`name="_csrf"`)
	if rec := c.postForm("/login", "usuario=admin&senha=errada"); rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	rec = c.postForm("/login", "usuario=admin&senha=trilha&next=/admin")
	if rec.Code != 303 || rec.Header().Get("Location") != "/admin" || rec.Cookie("sessao") == nil {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(rec.Cookie("sessao").Value, "|") {
		t.Fatalf("sessão sem assinatura: %q", rec.Cookie("sessao").Value)
	}
	c.Get("/admin").WantStatus(200).WantContains(`<h1 class="ui-h1">Olá, admin</h1>`, "2 posts publicados.")
	if rec := c.postForm("/login", "sair=1"); rec.Code != 303 {
		t.Fatal(rec.Code)
	}
	if rec := c.Get("/admin"); rec.Code != 302 {
		t.Fatal("logout should revoke access")
	}
}

func TestSetupSeededValues(t *testing.T) {
	c := newClient(t, "prod")
	var list []posts.Post
	_ = json.Unmarshal(c.Get("/api/posts").Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatal(len(list))
	}
}

func TestPublicAndTraversal(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/style.css")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatal("Config() in setup.go not applied:", cc)
	}
	// Um caminho sujo não passa por httptest.NewRequest: ele é montado aqui e
	// entregue direto ao handler.
	req := httptest.NewRequest("GET", "/x", nil)
	req.URL.Path = "/../go.mod"
	cru := httptest.NewRecorder()
	c.app.Handler().ServeHTTP(cru, req)
	if cru.Code == 200 {
		t.Fatal("traversal")
	}
}

func TestTrailingSlash(t *testing.T) {
	c := newClient(t, "prod")
	if rec := c.Get("/blog/"); rec.Code != 301 || rec.Header().Get("Location") != "/blog" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Location"))
	}
}

// ---- 002: grupos de rota ---------------------------------------------------

func TestGroups_LayoutWithoutURLSegment(t *testing.T) {
	c := newClient(t, "prod")
	c.Get("/precos").WantStatus(200).WantContains(`<main id="conteudo"><div class="ui-container"><section class="marketing"><nav class="sub ui-nav">`, `<h1 class="ui-h1">Preços</h1>`, "<title>Preços · Trilha Blog</title>")
	c.Get("/sobre").WantStatus(200).WantContains(`<section class="marketing">`, `<h1 class="ui-h1">Sobre</h1>`)
	if rec := c.Get("/marketing-/precos"); rec.Code != 404 {
		t.Fatalf("group name must not be a URL segment: %d", rec.Code)
	}
}

func TestGroups_MiddlewareOrder(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/painel")
	rec.WantStatus(200).WantContains(`<section class="app" data-area="painel" data-trilha-nav="conteudo">`, `<h1 class="ui-h1">Painel</h1>`)
	if rec.Header().Get("X-Area") != "painel" || rec.Header().Get("Server-Timing") == "" {
		t.Fatalf("root and group middlewares must both run: %v", rec.Header())
	}
	if rec := c.Get("/precos"); rec.Header().Get("X-Area") != "" {
		t.Fatal("painel middleware leaked into marketing group")
	}
}

// Issue #23: a área do app navega no cliente, mas o servidor continua
// respondendo a página inteira — o link é um link, e o endereço é o mesmo.
func TestNavegacaoNoClienteDegradaSemScript(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/painel")
	rec.WantStatus(200).WantContains(`data-trilha-nav="conteudo"`, `<script src="/ui.nav.js`, `<a href="/relatorio">Relatório</a>`)
	// O mesmo endereço, pedido direto, devolve o documento inteiro: recarregar
	// ou abrir em outra aba dá a mesma página.
	direct := c.Get("/relatorio")
	direct.WantStatus(200).WantContains("<!doctype html>", `id="conteudo"`, "<title>Relatório · Trilha Blog</title>")
	// O arquivo do comportamento é servido como qualquer estático do projeto.
	if js := c.Get("/ui.nav.js"); js.Code != 200 || !strings.Contains(js.Body.String(), "data-trilha-nav") {
		t.Fatalf("ui.nav.js: %d", js.Code)
	}
}

// Issue #24: o anexo passa do limite global de 1 MiB porque a rota levantou o
// dela, e a mesma rota responde fragmento ou página conforme quem pergunta.
func TestUploadComProgressoDegradaSemScript(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/anexos")
	rec.WantStatus(200).WantContains(`data-trilha-upload="anexos"`, "<progress", `id="anexos"`, `id="lista"`, `<script src="/ui.upload.js`)

	// 2 MiB: acima do MaxBodyBytes padrão, dentro do que a rota permitiu.
	body, ct := multipart(t, "planilha.csv", strings.Repeat("a", 2<<20))
	frag := c.Request("POST", "/anexos", trilha.WithBody(ct, body), fragmento)
	frag.WantStatus(200).WantContains(`id="anexos"`, "planilha.csv", "2,0 MB", "text/plain")
	if strings.Contains(frag.Body.String(), "<!doctype") {
		t.Fatal("o fragmento não pode vir com o documento inteiro")
	}

	// O mesmo envio sem JavaScript (sem o cabeçalho) volta para a página.
	body, ct = multipart(t, "sem-js.txt", "oi")
	plain := c.Request("POST", "/anexos", trilha.WithBody(ct, body))
	if plain.Code != 303 || plain.Header().Get("Location") != "/anexos" {
		t.Fatalf("envio normal deve redirecionar: %d %s", plain.Code, plain.Header().Get("Location"))
	}
	c.Get("/anexos").WantStatus(200).WantContains("sem-js.txt")

	// O limite continua valendo para o resto do app.
	huge := c.postForm("/blog/novo", strings.Repeat("x", 2<<20))
	if huge.Code == 200 {
		t.Fatal("o limite global tem de continuar de pé fora da rota de anexos")
	}
}

// Issue #28: o arquivo grande demais e o tipo mentido param no c.File, com a
// mensagem no campo e em português — não em 500 nem na lista.
func TestUploadRecusaTamanhoETipo(t *testing.T) {
	c := newClient(t, "prod")
	c.Get("/anexos")

	body, ct := multipart(t, "grande.txt", strings.Repeat("a", 5<<20))
	grande := c.Request("POST", "/anexos", trilha.WithBody(ct, body), fragmento)
	grande.WantStatus(422).WantContains("no máximo 4 MB", `aria-invalid="true"`, `id="anexos"`)

	// Um binário qualquer com nome de imagem: o tipo sai do conteúdo.
	body, ct = multipart(t, "foto.png", "\x00\x01\x02\x03rmnop\x00\xff")
	mentido := c.Request("POST", "/anexos", trilha.WithBody(ct, body), fragmento)
	mentido.WantStatus(422).WantContains("tipo de arquivo não permitido", `aria-invalid="true"`)

	if strings.Contains(c.Get("/anexos").Body.String(), "grande.txt") {
		t.Fatal("arquivo recusado não pode entrar na lista")
	}
}

// fragmento pede só o pedaço da página, como o script do upload faz.
var fragmento = trilha.WithHeader("Trilha-Fragment", "anexos")

// multipart monta um corpo multipart/form-data com um arquivo dentro. O token
// do CSRF vai no cabeçalho, posto pelo cliente.
func multipart(t *testing.T, name, content string) (body, contentType string) {
	t.Helper()
	var sb strings.Builder
	w := multipartlib.NewWriter(&sb)
	f, err := w.CreateFormFile("arquivo", name)
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(f, content)
	w.Close()
	return sb.String(), w.FormDataContentType()
}

// ---- 002: html/template ---------------------------------------------------

func TestTemplatePageInsideLayouts(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/relatorio")
	rec.WantStatus(200).WantContains(`<section class="app" data-area="painel" data-trilha-nav="conteudo">`, "<h1>Relatório &lt;de posts&gt;</h1>", `<a href="/blog/layouts">Layouts aninhados</a>`, "<title>Relatório · Trilha Blog</title>")
}

func TestTemplateErrorIs500(t *testing.T) {
	c := newClient(t, "dev")
	c.Get("/relatorio?t=nao-existe").WantStatus(500).WantContains("<h1>Algo deu errado</h1>", "nao-existe")
}

// ---- 004: segurança -------------------------------------------------------

func TestSecurityHeadersOnExample(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/")
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "img-src 'self' data: https:") || rec.Header().Get("Permissions-Policy") == "" {
		t.Fatalf("csp=%q", csp)
	}
}

func TestSignedSessionCannotBeForged(t *testing.T) {
	c := newClient(t, "prod")
	forjada := trilha.WithCookie("sessao", "admin|9999999999|assinatura-falsa")
	if rec := c.Get("/admin", forjada); rec.Code != 302 {
		t.Fatalf("forged session accepted: %d", rec.Code)
	}
}

func TestAPIRateLimit(t *testing.T) {
	c := newClient(t, "prod")
	var last int
	for i := 0; i < 25; i++ {
		last = c.Get("/api/posts").Code
	}
	if last != 429 {
		t.Fatalf("expected 429 after burst, got %d", last)
	}
}

// Spec 014: as sondas e o endereço de métricas no app de exemplo.
func TestHealthProbesOnExample(t *testing.T) {
	c := newClient(t, "prod")
	live := c.Get("/_trilha/health/live")
	if live.Code != 200 || live.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %v", live.Code, live.Header())
	}
	ready := c.Get("/_trilha/health/ready")
	if ready.Code != 200 || !strings.Contains(ready.Body.String(), `"status":"pass"`) {
		t.Fatalf("%d %s", ready.Code, ready.Body.String())
	}
	// Anônimo em produção não vê o nome da verificação.
	if strings.Contains(ready.Body.String(), "posts") {
		t.Fatal("detalhe vazou: " + ready.Body.String())
	}
	// Sem posts, a prontidão falha e o balanceador tira o processo da roda.
	c.posts().Delete("ola-trilha")
	c.posts().Delete("layouts")
	time.Sleep(2100 * time.Millisecond) // o resultado fica em cache por 2s
	if rec := c.Get("/_trilha/health/ready"); rec.Code != 503 {
		t.Fatalf("prontidão deveria falhar sem posts: %d %s", rec.Code, rec.Body.String())
	}
	if rec := c.Get("/_trilha/health/live"); rec.Code != 200 {
		t.Fatal("vivacidade não pode cair junto com uma dependência")
	}
}

func TestMetricsOnExample(t *testing.T) {
	t.Setenv("TRILHA_METRICS", "/_trilha/metrics")
	t.Setenv("TRILHA_OBS_TOKEN", "0123456789abcdef0123456789abcdef")
	c := newClient(t, "prod")
	c.Get("/")
	c.Get("/blog")
	c.posts().Create("Métrica de domínio", "corpo")
	if rec := c.Get("/_trilha/metrics"); rec.Code != 401 {
		t.Fatalf("raspagem anônima: %d", rec.Code)
	}
	rec := c.Get("/_trilha/metrics", trilha.WithHeader("Authorization", "Bearer 0123456789abcdef0123456789abcdef"))
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	for _, want := range []string{
		`trilha_requests_total{method="GET",route="/",status="200"}`,
		`trilha_requests_total{method="GET",route="/blog",status="200"}`,
		"blog_posts_total 1",
		"trilha_request_duration_seconds_bucket",
		"go_goroutines",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %q", want)
		}
	}
}

// Ilha (#22): a página manda o formulário pronto e o módulo da ilha ao lado.
// O que prova a convenção é o que sobra sem script: o textarea, com nome e
// tudo, servido pelo servidor.
func TestIlhaDoEditorDegradaSemScript(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/blog/novo")
	body := rec.Body.String()
	for _, want := range []string{
		`data-trilha-island="/ilha-editor.js`,
		`data-trilha-props="{&#34;palavrasPorMinuto&#34;:200}"`,
		`<textarea class="ui-textarea" id="corpo" name="corpo" rows="6"></textarea>`,
		`data-info=""`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("falta %s na página:\n%s", want, body)
		}
	}
	// O carregador é inline e precisa do nonce da requisição, senão a CSP
	// padrão o recusa.
	nonce := rec.Header().Get("Content-Security-Policy")
	i := strings.Index(nonce, "'nonce-")
	if i < 0 {
		t.Fatal("sem nonce na CSP")
	}
	n := nonce[i+7 : i+7+strings.Index(nonce[i+7:], "'")]
	if !strings.Contains(body, `<script nonce="`+n+`">`) {
		t.Fatal("o carregador da ilha não levou o nonce da requisição")
	}
}

// Issue #25: a lista de posts vem do cache, e publicar não pode atrasar. O
// que o cache resolve (a segunda visita não vai ao banco) e o que ele não pode
// quebrar (quem publicou vê o post) cabem no mesmo teste.
func TestCacheDaListaDePosts(t *testing.T) {
	t.Setenv("TRILHA_METRICS", "/_trilha/metrics")
	t.Setenv("TRILHA_OBS_TOKEN", "0123456789abcdef0123456789abcdef")
	c := newClient(t, "prod")
	c.Get("/blog").WantStatus(200).WantContains(`href="/blog/layouts"`, "2 posts")
	c.Get("/blog").WantStatus(200).WantContains(`href="/blog/layouts"`)

	c.Get("/blog/novo")
	if rec := c.postForm("/blog/novo", "titulo=Cache+quente&corpo=x"); rec.Code != 303 {
		t.Fatalf("publicar: %d", rec.Code)
	}
	c.Get("/blog").WantStatus(200).WantContains(`href="/blog/cache-quente"`, "3 posts")

	rec := c.Get("/_trilha/metrics", trilha.WithHeader("Authorization", "Bearer 0123456789abcdef0123456789abcdef"))
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`trilha_cache_hits_total{cache="posts"} 1`,
		`trilha_cache_misses_total{cache="posts"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("falta %q em\n%s", want, body)
		}
	}
}

// Issue #26: a página do post carrega a versão do dado, então um F5 devolve
// 304 e o corpo não viaja de novo. Republicar muda a versão e a página volta.
func TestPostRespondeComETag(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/blog/ola-trilha")
	tag := rec.Header().Get("ETag")
	if rec.Code != 200 || tag == "" {
		t.Fatalf("%d %q", rec.Code, tag)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "private") {
		t.Fatalf("página de post sem private: %q", cc)
	}

	again := c.Get("/blog/ola-trilha", trilha.WithHeader("If-None-Match", tag))
	if again.Code != 304 || again.Body.Len() != 0 {
		t.Fatalf("revalidação: %d, %d bytes", again.Code, again.Body.Len())
	}

	c.posts().Create("Ola Trilha", "outro corpo")
	novo := c.Get("/blog/ola-trilha", trilha.WithHeader("If-None-Match", tag))
	if novo.Code != 200 || !strings.Contains(novo.Body.String(), "outro corpo") {
		t.Fatalf("depois de republicar: %d", novo.Code)
	}
}

// #29 — o preflight do painel de outra origem é respondido pelo framework, e
// quem não está na lista não passa dele.
func TestCORSDoPainel(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Request("OPTIONS", "/api/posts",
		trilha.WithHeader("Origin", "https://painel.exemplo.com"),
		trilha.WithHeader("Access-Control-Request-Method", "POST"))
	if rec.Code != 204 {
		t.Fatalf("preflight = %d, quero 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://painel.exemplo.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("max-age = %q", got)
	}
	rec = c.Request("OPTIONS", "/api/posts",
		trilha.WithHeader("Origin", "https://atacante.net"),
		trilha.WithHeader("Access-Control-Request-Method", "POST"))
	if rec.Code != 403 || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("origem de fora: %d %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	// A leitura simples do painel continua funcionando, agora com o cabeçalho.
	rec = c.Get("/api/posts", trilha.WithHeader("Origin", "https://painel.exemplo.com"))
	if rec.Code != 200 || rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Errorf("GET do painel: %d sem allow-origin", rec.Code)
	}
}

// #30 — o erro da API é problem+json, com o type e o membro de extensão que o
// handler escolheu.
func TestProblemJSONDaAPI(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.PostJSON("/api/posts", map[string]string{"title": "Repetido", "body": "b"})
	if rec.Code != 201 {
		t.Fatalf("primeiro POST = %d", rec.Code)
	}
	rec = c.PostJSON("/api/posts", map[string]string{"title": "Repetido", "body": "b"})
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), trilha.ProblemMediaType) {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	rec.WantStatus(409).WantContains(`"type":"https://trilha.dev/probs/slug-em-uso"`, `"slug":"repetido"`, `"instance":"/api/posts"`)
	// A validação continua respondendo fields, no mesmo corpo.
	c.Request("POST", "/api/posts", trilha.WithBody("", `{"title":""}`)).WantStatus(422).WantContains(`"fields":{"title":"obrigatório"}`)
}

// #56 e #53 — a rota de papel misto do painel. A leitura é de quem chegou, a
// escrita é de quem entrou, e a regra da escrita mora em middleware.go, não na
// primeira linha do handler. A recusa sai com a cara do app.
func TestPainelPOSTGuardadoPorMetodo(t *testing.T) {
	c := newClient(t, "prod")
	// O middleware do POST não toca no GET: a leitura segue aberta.
	c.Get("/painel").WantStatus(200).WantContains(`<h1 class="ui-h1">Painel</h1>`, `name="meta"`)
	// Sem sessão, o POST para na regra do grupo e o 403 é a página do app.
	rec := c.postForm("/painel", "meta=20")
	rec.WantStatus(403).WantContains("<h1>Sem acesso</h1>", "<title>Sem acesso · Trilha Blog</title>",
		"só quem entrou pode mudar a meta do mês", `href="/login?next=/painel"`)
	// Depois de entrar, o mesmo POST passa e o painel mostra a meta nova.
	if rec := c.postForm("/login", "usuario=admin&senha=trilha"); rec.Code != 303 {
		t.Fatalf("login: %d", rec.Code)
	}
	if rec := c.postForm("/painel", "meta=20"); rec.Code != 303 {
		t.Fatalf("POST autorizado: %d %s", rec.Code, rec.Body.String())
	}
	c.Get("/painel").WantStatus(200).WantContains("de 20")
}

// Spec 046 (#55): dois apps no mesmo processo não enxergam as dependências um
// do outro. É o que o estado de pacote impedia — e é o formato de qualquer
// suíte que sobe um servidor por teste.
func TestDoisAppsNoMesmoProcesso(t *testing.T) {
	um := newClient(t, "prod")
	dois := newClient(t, "prod")
	um.posts().Create("Só no primeiro", "corpo")

	if _, ok := dois.posts().Get("so-no-primeiro"); ok {
		t.Fatal("o segundo app enxergou o post do primeiro")
	}
	if body := dois.Get("/blog").Body.String(); strings.Contains(body, "Só no primeiro") {
		t.Fatal("a lista do segundo app trouxe o post do primeiro")
	}
	if body := um.Get("/blog").Body.String(); !strings.Contains(body, "Só no primeiro") {
		t.Fatal("o primeiro app perdeu o próprio post")
	}
}

// #75 — /.well-known/ é a exceção do ponto no começo do nome: a pasta vira
// rota como qualquer outra, e a URL que a RFC 9116 exige responde de verdade.
func TestWellKnownSecurityTxt(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Get("/.well-known/security.txt")
	rec.WantStatus(200).WantContains("Contact: mailto:")
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("content-type %q", ct)
	}
}

// #76/#78 — o documento do /.well-known/ é buscado de outra origem, e a
// política é dele: o preflight responde aqui sem que as outras 14 rotas do blog
// deixem de ser de mesma origem.
func TestWellKnownRespondePreflight(t *testing.T) {
	c := newClient(t, "prod")
	rec := c.Request("OPTIONS", "/.well-known/security.txt",
		trilha.WithHeader("Origin", "https://cliente.exemplo.com"),
		trilha.WithHeader("Access-Control-Request-Method", "GET"))
	rec.WantStatus(204).WantHeader("Access-Control-Allow-Origin", "*")

	doc := c.Get("/.well-known/security.txt", trilha.WithHeader("Origin", "https://cliente.exemplo.com"))
	doc.WantStatus(200).WantHeader("Access-Control-Allow-Origin", "*")

	// A rota vizinha continua fechada, que é a razão de a política ser por rota.
	if got := c.Get("/api/posts", trilha.WithHeader("Origin", "https://cliente.exemplo.com")).
		Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("a API do blog ganhou a política do documento: %q", got)
	}
}
