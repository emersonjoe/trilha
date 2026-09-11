package main

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scenario is one task the agent gets, in one sentence, and the hidden test
// that decides whether it was done. The prompt is part of the contract: it
// does not change without a version, or the before and the after would be
// two different rulers.
type Scenario struct {
	Name    string
	Title   string
	Example string // examples/blog or examples/sso, relative to the repo
	Prompt  string
	// Prepare edits the copied example before the agent sees it. Nil keeps
	// the example as it is.
	Prepare func(dir string) error
	// Tests are the hidden tests, path relative to the copy -> source.
	Tests map[string]string
	// Check is an extra assertion on the source after the tests passed.
	Check func(dir string) error
	// Serve brings up what the project talks to — a service that is somewhere
	// else — and returns the environment saying where it landed, plus the way
	// to take it down. Nil is a scenario that talks to nobody.
	//
	// It runs twice: once around the agent, so it can try what it writes, and
	// once around the verification. Two runs, two addresses, one at a time:
	// what the hidden test reads is the API that answered it.
	Serve func() (env []string, stop func())
}

// Scenarios in the order the table shows them.
func Scenarios() []Scenario {
	return []Scenario{comments, contactForm, cognito, pagination, portListing, apiCall}
}

// ScenarioByName finds one; "" is not a name.
func ScenarioByName(name string) (Scenario, bool) {
	for _, s := range Scenarios() {
		if s.Name == name {
			return s, true
		}
	}
	return Scenario{}, false
}

var comments = Scenario{
	Name:    "comments",
	Title:   "rota de API com Bind, validação e 404",
	Example: "examples/blog",
	Prompt:  `Adicione comentários à API deste blog: POST /api/posts/{id}/comments recebe JSON {"author": "...", "body": "..."} (os dois obrigatórios, body com no máximo 500 caracteres), responde 201 com o comentário criado em JSON (campos author, body, created), 422 quando o corpo é inválido e 404 quando o post não existe; GET /api/posts/{id}/comments lista os comentários do post em JSON (array). Guarde em memória, como os posts. Escreva um teste da rota com os auxiliares de teste do próprio Trilha e deixe go vet ./... e go test ./... verdes.`,
	Tests: map[string]string{"zz_bench_test.go": `package main

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/emersonjoe/trilha"
)

func TestBenchComments(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	ok := map[string]string{"author": "Ana", "body": "Primeiro!"}
	trilha.TestRequest(t, a, "POST", "/api/posts/ola-trilha/comments", trilha.WithJSON(ok)).
		WantStatus(http.StatusCreated).WantContains(` + "`\"author\"`, `\"Ana\"`, `\"body\"`" + `)
	trilha.TestRequest(t, a, "POST", "/api/posts/ola-trilha/comments", trilha.WithJSON(map[string]string{"author": "", "body": ""})).
		WantStatus(http.StatusUnprocessableEntity)
	trilha.TestRequest(t, a, "POST", "/api/posts/nao-existe/comments", trilha.WithJSON(ok)).
		WantStatus(http.StatusNotFound)
	var list []map[string]any
	trilha.TestRequest(t, a, "GET", "/api/posts/ola-trilha/comments").WantStatus(http.StatusOK).JSON(&list)
	// The store is shared by every test in the package, so the list may
	// hold more than this test wrote: ours must be in it, that is all.
	found := false
	for _, c := range list {
		if c["author"] == "Ana" && c["body"] == "Primeiro!" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GET after POST = %v, want the comment by Ana in it", list)
	}
}
`},
}

var contactForm = Scenario{
	Name:    "contact-form",
	Title:   "página com formulário do kit ui no layout raiz",
	Example: "examples/blog",
	Prompt:  `Adicione a página /contato a este blog, dentro do layout raiz que já existe, com um formulário de contato feito com o kit ui do Trilha: campos nome, email e mensagem (todos obrigatórios, email válido). O POST vai para a própria página: com erro, a página volta com as mensagens nos campos e status 422; válido, mostra um agradecimento. Deixe go vet ./... e go test ./... verdes.`,
	Tests: map[string]string{"zz_bench_test.go": `package main

import (
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/emersonjoe/trilha"
)

func TestBenchContato(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())
	// "Preços" is a link of the root layout: the page must be inside it.
	c.Get("/contato").WantStatus(200).WantContains("<form", "Preços")
	res := c.PostForm("/contato", url.Values{"nome": {"Ana"}, "email": {"ana@example.com"}, "mensagem": {"Olá"}})
	if res.Code != 200 && res.Code != 303 {
		t.Fatalf("valid POST /contato = %d, want 200 or 303\n%s", res.Code, res.Body)
	}
	c.PostForm("/contato", url.Values{"nome": {""}, "email": {"nao-e-email"}, "mensagem": {""}}).WantStatus(422)
}
`},
}

var cognito = Scenario{
	Name:    "cognito",
	Title:   "trocar o provedor de login de Keycloak para Cognito",
	Example: "examples/sso",
	Prompt:  `Este app faz login com Keycloak. Troque o provedor para AWS Cognito usando o pacote auth do Trilha: a região vem de SSO_REGION, o user pool de SSO_USER_POOL_ID e o domínio de logout de SSO_LOGOUT_DOMAIN (as variáveis SSO_URL e SSO_REALM deixam de existir). Atualize a mensagem que explica o que falta configurar e o README. Deixe go vet ./... e go test ./... verdes.`,
	Prepare: keycloakOnly,
	Tests: map[string]string{"internal/sso/zz_bench_test.go": `package sso

import (
	"strings"
	"testing"
)

func TestBenchCognito(t *testing.T) {
	for k, v := range map[string]string{
		"SSO_CLIENT_ID": "cid", "SSO_CLIENT_SECRET": "sec", "SSO_REDIRECT_URL": "https://app.example/entrar/retorno",
		"SSO_REGION": "us-east-1", "SSO_USER_POOL_ID": "us-east-1_AbC123", "SSO_LOGOUT_DOMAIN": "https://auth.example.com",
		"SSO_URL": "", "SSO_REALM": "",
	} {
		t.Setenv(k, v)
	}
	Configure()
	if !Configurado() {
		t.Fatalf("Configure with the Cognito variables set: %s", Motivo())
	}
	if d := Descricao(); !strings.Contains(d, "cognito-idp.us-east-1.amazonaws.com/us-east-1_AbC123") {
		t.Fatalf("issuer = %s, want the Cognito one", d)
	}
	if got := flow.Provider().LogoutDomain; got != "https://auth.example.com" {
		t.Fatalf("LogoutDomain = %q, want SSO_LOGOUT_DOMAIN", got)
	}
}
`},
	Check: func(dir string) error {
		b, err := os.ReadFile(filepath.Join(dir, "internal", "sso", "sso.go"))
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "auth.Keycloak(") {
			return errors.New("internal/sso/sso.go still calls auth.Keycloak")
		}
		return nil
	},
}

// keycloakOnly collapses the provider switch of examples/sso to Keycloak,
// so that "switch to Cognito" is work and not a variable.
func keycloakOnly(dir string) error {
	path := filepath.Join(dir, "internal", "sso", "sso.go")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(b)
	start := strings.Index(src, "\tvar p *auth.Provider\n")
	end := strings.Index(src, "\tflow = auth.New(")
	if start < 0 || end < 0 || end < start {
		return fmt.Errorf("%s: provider switch not where expected; update keycloakOnly", path)
	}
	src = src[:start] + `	base, realm := os.Getenv("SSO_URL"), os.Getenv("SSO_REALM")
	if base == "" || realm == "" {
		motivo = "defina SSO_URL e SSO_REALM (o Keycloak e o realm)"
		return
	}
	p := auth.Keycloak(base, realm, id, secret, redirect)
` + src[end:]
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		return err
	}
	readme := `# Exemplo: login com Keycloak

Variáveis de ambiente:

- SSO_CLIENT_ID, SSO_CLIENT_SECRET, SSO_REDIRECT_URL — o cliente OIDC.
- SSO_URL, SSO_REALM — o Keycloak e o realm.
- SSO_ADMIN_ROLE — papel exigido em /painel/relatorio (padrão admin).
- SSO_ROLE_CLAIMS — claims extras onde procurar papéis, separadas por vírgula.

Rode com trilha dev; sem configuração o app sobe e a página inicial diz o que falta.
`
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}
	// The example's own suite drives /entrar against a fake IdP, which only
	// works for a provider whose issuer is configurable. Cognito's is not, so
	// the fixture keeps just the test that needs no provider at all; the hidden
	// test is what checks the switch.
	return os.WriteFile(filepath.Join(dir, "sso_test.go"), []byte(ssoTestNoProvider), 0o644)
}

const ssoTestNoProvider = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/emersonjoe/trilha"
)

var navegador = trilha.WithHeader("Accept", "text/html")

type cliente struct{ *trilha.TestClient }

func novo(t *testing.T) *cliente {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "0123456789abcdef0123456789abcdef")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &cliente{trilha.NewTestClient(t, newApp())}
}

func (c *cliente) get(path string) *trilha.TestResponse { return c.Get(path, navegador) }

func (c *cliente) api(path string) *trilha.TestResponse {
	return c.Get(path, trilha.WithHeader("Accept", "application/json"))
}

// Sem provedor o app sobe e explica o que falta, em vez de quebrar.
func TestSemConfiguracaoOAppExplica(t *testing.T) {
	c := novo(t)
	c.get("/").WantStatus(200).WantContains("Login indisponível")
	c.get("/painel").WantStatus(http.StatusSeeOther).WantHeader("Location", "/")
	c.api("/api/eu").WantStatus(http.StatusServiceUnavailable)
}
`

var pagination = Scenario{
	Name:    "pagination",
	Title:   "paginar a lista de posts",
	Example: "examples/blog",
	Prompt:  `A página /blog lista todos os posts de uma vez. Faça-a mostrar 5 posts por página: ?page=N escolhe a página (1 por padrão), e abaixo da lista aparecem os links para a página anterior e a próxima quando existem, com a página atual indicada, usando o componente de paginação do kit ui ou a receita do cookbook do Trilha. Deixe go vet ./... e go test ./... verdes.`,
	Tests: map[string]string{"zz_bench_test.go": `package main

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"testing"

	"github.com/emersonjoe/trilha"
	"MODULE/internal/posts"
)

// postLinks counts links to posts; /blog/novo is the "new post" button.
var postLinks = regexp.MustCompile(` + "`" + `href="/blog/[^"]+"` + "`" + `)

func countPosts(body string) int {
	n := 0
	for _, m := range postLinks.FindAllString(body, -1) {
		if m != ` + "`" + `href="/blog/novo"` + "`" + ` {
			n++
		}
	}
	return n
}

func TestBenchPaginacao(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	store := trilha.Use[*posts.Store](a)
	for i := 1; i <= 12; i++ { // 2 seeded + 12 = 14 posts: 5, 5, 4
		store.Create(fmt.Sprintf("Post %02d", i), "corpo")
	}
	p1 := trilha.TestRequest(t, a, "GET", "/blog").WantStatus(200)
	if n := countPosts(p1.Body.String()); n != 5 {
		t.Fatalf("page 1 lists %d posts, want 5", n)
	}
	trilha.TestRequest(t, a, "GET", "/blog?page=2").WantStatus(200).WantContains("page=1", "page=3")
	p3 := trilha.TestRequest(t, a, "GET", "/blog?page=3").WantStatus(200)
	if n := countPosts(p3.Body.String()); n != 4 {
		t.Fatalf("page 3 lists %d posts, want 4", n)
	}
}
`},
}

//go:embed fixtures/documentos.tsx fixtures/MIGRATION.snippet.md fixtures/acervo.tsx fixtures/MIGRATION.acervo.md
var fixtures embed.FS

// portListing is the scenario Phase 7 asked for and never got: take a listing
// that exists as a React page and write it in Trilha.
//
// It is the one screen that touches most of what that phase shipped —
// ListParams, ui.DataTable, ui.Poll, the fragment — and a ruler that does not
// measure the thing you changed is a ruler that agrees with any result.
//
// The example already has this screen, which is what makes the scenario
// honest in both directions: Prepare puts a stub in its place, so the agent
// has to write it, and the screen that was replaced is the proof that the
// hidden test is passable — which TestPortListingEhAtingivel runs.
var portListing = Scenario{
	Name:    "port-listing",
	Title:   "portar uma listagem .tsx para o Trilha",
	Example: "examples/blog",
	Prompt: `A tela de documentos deste projeto existia em Next.js e o arquivo original está em ` +
		`app/documentos/page.tsx.txt, com a linha correspondente do MIGRATION.md ao lado. ` +
		`Escreva o equivalente em app/documentos/page.go, contra o pacote internal/documentos ` +
		`que já existe, mantendo o filtro por busca e por tipo, a ordenação por coluna vinda da ` +
		`URL, a paginação e a atualização automática da tabela. Deixe go vet ./... e ` +
		`go test ./... verdes.`,
	Prepare: portListingPrepare,
	Tests:   map[string]string{"zz_bench_test.go": portListingTest},
}

// portListingPrepare replaces the page with a stub and leaves the source
// beside it, with the migration note.
//
// Only the page: the folder's other routes read the same store and keep
// compiling, so what fails afterwards is the screen and not the project.
func portListingPrepare(dir string) error {
	page := filepath.Join(dir, "app", "documentos", "page.go")
	if _, err := os.Stat(page); err != nil {
		return fmt.Errorf("%s: the screen this scenario asks for is not where it was; update portListingPrepare", page)
	}
	// A stub and not an empty folder: a folder with no Go file is a package
	// that stops existing, and trilha_gen.go still imports it — the fixture
	// would fail to build for a reason that has nothing to do with the task.
	// It is also the fairer starting point, because it is how a port actually
	// begins: the route is there and the screen is not.
	if err := os.WriteFile(page, []byte(portListingStub), 0o644); err != nil {
		return err
	}
	// .tsx.txt and not .tsx: a stray .tsx in the tree is a file some tool
	// tries to build, and this one is here to be read.
	for name, dst := range map[string]string{
		"fixtures/documentos.tsx":       filepath.Join(dir, "app", "documentos", "page.tsx.txt"),
		"fixtures/MIGRATION.snippet.md": filepath.Join(dir, "MIGRATION.md"),
	} {
		b, err := fixtures.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

const portListingTest = `package main

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// posicao is where a name shows up in the body, or -1. Comparing two of them
// is how the test reads the order of the rows without parsing HTML.
func posicao(body, s string) int { return strings.Index(body, s) }

func TestBenchPortListing(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()

	// The columns the .tsx had.
	body := trilha.TestRequest(t, a, "GET", "/documentos").WantStatus(200).Body.String()
	for _, quero := range []string{"Documento", "Tipo", "Tamanho", "Status"} {
		if !strings.Contains(body, quero) {
			t.Fatalf("a coluna %q sumiu na porta", quero)
		}
	}

	// The ordering is the URL's, and it says so out loud: a table ordered for
	// whoever sees it and not for whoever listens is half a table.
	desc := trilha.TestRequest(t, a, "GET", "/documentos?sort=tamanho&dir=desc").WantStatus(200).Body.String()
	if !strings.Contains(desc, "aria-sort=\"descending\"") {
		t.Fatalf("sem aria-sort na coluna ordenada:\n%s", primeiroTrecho(desc))
	}
	asc := trilha.TestRequest(t, a, "GET", "/documentos?sort=tamanho&dir=asc").WantStatus(200).Body.String()
	if desc == asc {
		t.Fatal("dir=asc e dir=desc devolveram a mesma ordem: a ordenação não veio da URL")
	}

	// The pagination carries the filter. Losing it is the classic port of a
	// useSearchParams, and what makes page two show something else.
	comFiltro := trilha.TestRequest(t, a, "GET", "/documentos?q=a&limit=5").WantStatus(200).Body.String()
	if strings.Contains(comFiltro, "offset=") && !strings.Contains(comFiltro, "q=a") {
		t.Fatalf("os links de página perderam o filtro:\n%s", primeiroTrecho(comFiltro))
	}

	// The automatic refresh exists, and it is a fragment: a poll that answers
	// the whole page puts the page inside itself.
	if !strings.Contains(body, "data-trilha-poll") && !strings.Contains(body, "data-trilha-live") {
		t.Fatalf("a tela não se atualiza sozinha:\n%s", primeiroTrecho(body))
	}
	if id := fragmentoDe(body); id != "" {
		// O fragmento se pede pelo cabeçalho, não pela URL: a mesma rota
		// responde a página ou o pedaço conforme quem pergunta.
		frag := trilha.TestRequest(t, a, "GET", "/documentos",
			trilha.WithHeader("Trilha-Fragment", id)).WantStatus(200).Body.String()
		if strings.Contains(strings.ToLower(frag), "<html") {
			t.Fatalf("o fragmento %q devolveu a página inteira", id)
		}
	}

	// A 'use client' ported to Go that brings the JavaScript along was not
	// ported. What counts is what the page loads: an island, or a script of
	// the project's own. The kit's files are the exception — they are what
	// make the poll and the sortable header work with no JavaScript here.
	//
	// Inline scripts are left out on purpose: the shell writes one itself, and
	// telling it from somebody else's by reading the source is a test that
	// guesses. The order and the refresh are already proven server-side above.
	for _, trecho := range []string{body, desc} {
		if strings.Contains(trecho, "data-trilha-island") {
			t.Error("a tela carregou uma ilha: esta listagem cabe inteira no servidor")
		}
		for _, src := range scriptsDe(trecho) {
			if !strings.Contains(src, "/ui.") {
				t.Errorf("apareceu JavaScript próprio na página: %s", src)
			}
		}
	}
}

// scriptsDe returns the src of every script the page loads. A src into the
// kit is the kit; anything else is JavaScript somebody wrote.
func scriptsDe(body string) []string {
	var out []string
	resto := body
	for {
		i := strings.Index(resto, "<script")
		if i < 0 {
			return out
		}
		resto = resto[i+7:]
		fim := strings.IndexByte(resto, '>')
		if fim < 0 {
			return out
		}
		tag := resto[:fim]
		resto = resto[fim:]
		k := strings.Index(tag, "src=\"")
		if k < 0 {
			continue // inline: não é o que este teste mede
		}
		src := tag[k+5:]
		if f := strings.IndexByte(src, '"'); f >= 0 {
			src = src[:f]
		}
		out = append(out, src)
	}
}

// fragmentoDe reads the id the poll asks for, which is the id of the element
// that carries it.
func fragmentoDe(body string) string {
	i := strings.Index(body, "data-trilha-poll")
	if i < 0 {
		return ""
	}
	antes := body[:i]
	j := strings.LastIndex(antes, "id=\"")
	if j < 0 {
		return ""
	}
	resto := antes[j+4:]
	fim := strings.IndexByte(resto, '"')
	if fim < 0 {
		return ""
	}
	return resto[:fim]
}

func primeiroTrecho(s string) string {
	if len(s) > 800 {
		return s[:800]
	}
	return s
}
`

// portListingStub is what the agent finds where the screen was: the route
// answers, and there is nothing on it.
const portListingStub = `// Package documentos is the listing this project used to have in Next.js.
// The original is in page.tsx.txt, and MIGRATION.md says what each piece of it
// becomes here.
package documentos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page answers GET /documentos.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Documentos")
	return h.Div(h.H1(h.Text("Documentos"))), nil
}
`

// apiCall is the other half of #94: port-listing measures the screen, this one
// measures the path the call takes to get to the API.
//
// The app is examples/local-login, which is the one that already has an API
// somewhere else — Config.Upstreams and a session carrying the credential the
// API wants. The screen being ported ran in the browser and read the token out
// of localStorage, which is where a migration usually leaves it: the port is
// done when the credential comes from the session and the browser never sees
// it.
//
// It is the only scenario with a service of its own up during the run, and
// that is what it costs to measure a call instead of a page.
var apiCall = Scenario{
	Name:    "api-call",
	Title:   "portar a chamada à API que ficou onde estava",
	Example: "examples/local-login",
	Prompt: `A tela de documentos deste app existia em Next.js e rodava no browser: o arquivo ` +
		`original está em app/painel/documentos/page.tsx.txt, com a linha correspondente do ` +
		`MIGRATION.md ao lado. Escreva o equivalente em app/painel/documentos/page.go, ` +
		`renderizado no servidor, contra a API que continua onde estava: a base dela está na ` +
		`variável API_URL e o documento OpenAPI que ela publica é o openapi.json na raiz do ` +
		`projeto. A tela lista os documentos com o filtro de busca que vem da URL, e a ` +
		`credencial da chamada é a da sessão de quem está logado — nunca uma que o browser ` +
		`mande. Deixe go vet ./... e go test ./... verdes.`,
	Prepare: apiCallPrepare,
	Tests:   map[string]string{"zz_bench_test.go": apiCallTest},
	Serve:   serveAcervo,
	Check:   clienteGerado,
}

// apiCallPrepare puts the screen back to a stub, takes the generated client
// away and leaves what a migration actually starts from: the page as it was in
// React, the migration note, and the document the API publishes.
//
// The client goes because it is the answer: with internal/acervo committed in
// the tree there is nothing to find out, and what this scenario asks is
// whether the tool that writes it — `trilha client`, one command over the
// openapi.json that stays — gets reached for at all.
func apiCallPrepare(dir string) error {
	page := filepath.Join(dir, "app", "painel", "documentos", "page.go")
	if _, err := os.Stat(page); err != nil {
		return fmt.Errorf("%s: the screen this scenario asks for is not where it was; update apiCallPrepare", page)
	}
	if _, err := os.Stat(filepath.Join(dir, "openapi.json")); err != nil {
		return fmt.Errorf("%s: the API document the task points at is gone; update apiCallPrepare", dir)
	}
	if err := os.WriteFile(page, []byte(apiCallStub), 0o644); err != nil {
		return err
	}
	// The generated client, and the example's own test of the screen, which
	// names the package that just stopped existing.
	if err := os.RemoveAll(filepath.Join(dir, "internal", "acervo")); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, "documentos_test.go")); err != nil {
		return err
	}
	// .tsx.txt and not .tsx: a stray .tsx in the tree is a file some tool
	// tries to build, and this one is here to be read.
	for name, dst := range map[string]string{
		"fixtures/acervo.tsx":          filepath.Join(dir, "app", "painel", "documentos", "page.tsx.txt"),
		"fixtures/MIGRATION.acervo.md": filepath.Join(dir, "MIGRATION.md"),
	} {
		b, err := fixtures.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// clienteGerado is the half of the acceptance that no request can show: the
// call may carry the right credential and still have been written by hand,
// forty lines of http.NewRequest and json.Decode over a document that was one
// command away from being types.
//
// The other way the issue allows — Config.Upstreams — is not checked here
// because this app already declares one: a screen rendered on the server does
// not go through the app's own proxy, so the upstream that is in setup.go says
// nothing about what the agent did.
func clienteGerado(dir string) error {
	found := false
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(b, []byte("Code generated by trilha client")) {
			found = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !found {
		return errors.New("no file generated by `trilha client`: the API publishes an openapi.json and the call was written by hand")
	}
	return nil
}

// apiCallStub is what the agent finds where the screen was.
const apiCallStub = `// Package documentos is the listing this app used to have in Next.js, running
// in the browser. The original is in page.tsx.txt, MIGRATION.md says what each
// piece of it becomes here, and openapi.json in the project root is what the
// API publishes.
package documentos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page answers GET /painel/documentos.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Documentos")
	return h.Div(h.Class("cartao"), h.H1(h.Text("Documentos"))), nil
}
`

// apiCallTest is the hidden test. It reads the call from the far side: the API
// the scenario brought up says what arrived, which is the only place the
// credential is visible.
const apiCallTest = `package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// recebidaBench is one request as the API saw it.
type recebidaBench struct {
	Method string ` + "`json:\"method\"`" + `
	Path   string ` + "`json:\"path\"`" + `
	Query  string ` + "`json:\"query\"`" + `
	Auth   string ` + "`json:\"auth\"`" + `
}

// baseDoAcervo is where the scenario put the API. Without it there is nothing
// to measure, and saying so is better than a page that is empty for a reason
// nobody can see.
func baseDoAcervo(t *testing.T) string {
	t.Helper()
	base := os.Getenv("API_URL")
	if base == "" {
		t.Fatal("API_URL vazia: a API do cenário não subiu, e sem ela este teste não mede nada")
	}
	return base
}

// recebidasBench asks the API what came in. The log is the whole run's, so
// every assertion below takes what appeared since it started looking.
func recebidasBench(t *testing.T, base string) []recebidaBench {
	t.Helper()
	res, err := http.Get(base + "/__bench/received")
	if err != nil {
		t.Fatalf("a API do cenário não respondeu: %v", err)
	}
	defer res.Body.Close()
	var out []recebidaBench
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("o log da API não é JSON: %v", err)
	}
	return out
}

func desde(t *testing.T, base string, n int) []recebidaBench {
	t.Helper()
	todas := recebidasBench(t, base)
	if len(todas) < n {
		t.Fatalf("o log da API encolheu: %d agora, %d antes", len(todas), n)
	}
	return todas[n:]
}

func clienteBench(t *testing.T) *trilha.TestClient {
	t.Helper()
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return trilha.NewTestClient(t, newApp())
}

func entrarBench(t *testing.T, c *trilha.TestClient, email, senha string) {
	t.Helper()
	c.Request("POST", "/entrar", trilha.WithBody("application/x-www-form-urlencoded",
		"email="+email+"&senha="+senha)).WantStatus(http.StatusSeeOther)
}

func TestBenchChamadaComACredencialDaSessao(t *testing.T) {
	base := baseDoAcervo(t)
	c := clienteBench(t)

	// Sem sessão não há tela, e a API não fica sabendo que alguém tentou: a
	// pasta é guardada antes de a página existir.
	antes := len(recebidasBench(t, base))
	c.Get("/painel/documentos", trilha.WithHeader("Accept", "text/html"))
	if novas := desde(t, base, antes); len(novas) > 0 {
		t.Fatalf("a API foi chamada por quem não tem sessão: %+v", novas)
	}

	entrarBench(t, c, "ana@exemplo.com", "segredo-da-ana")

	// A tela mostra o que só a API sabe: estes nomes não existem no projeto.
	antes = len(recebidasBench(t, base))
	body := c.Get("/painel/documentos").WantStatus(200).Body.String()
	for _, quero := range []string{"contrato-2026.pdf", "nota-fiscal-9.pdf"} {
		if !strings.Contains(body, quero) {
			t.Fatalf("a tela não traz %q, que só a API tem:\n%s", quero, trecho(body))
		}
	}
	novas := desde(t, base, antes)
	if len(novas) == 0 {
		t.Fatal("a tela respondeu sem chamar a API")
	}
	// A credencial é a da sessão de quem está logado. É esta linha que separa
	// a porta feita da porta que compila.
	for _, r := range novas {
		if r.Auth != "Bearer jwt-da-ana" {
			t.Fatalf("a API recebeu Authorization %q, queria o token da sessão (Bearer jwt-da-ana)", r.Auth)
		}
	}
	// E ela não passa pela página: um token no HTML é o localStorage de novo,
	// com outro nome.
	if strings.Contains(body, "jwt-da-ana") {
		t.Fatalf("o token da sessão apareceu no HTML:\n%s", trecho(body))
	}

	// O filtro da URL é o parâmetro da API, e não uma fatia filtrada em Go
	// depois de pedir tudo.
	antes = len(recebidasBench(t, base))
	filtrada := c.Get("/painel/documentos?q=contrato").WantStatus(200).Body.String()
	novas = desde(t, base, antes)
	comQ := false
	for _, r := range novas {
		if strings.Contains(r.Query, "q=contrato") {
			comQ = true
		}
	}
	if !comQ {
		t.Fatalf("a busca da URL não chegou à API: %+v", novas)
	}
	if strings.Contains(filtrada, "nota-fiscal-9.pdf") {
		t.Fatalf("a tela filtrada ainda lista o que a API deixou de mandar:\n%s", trecho(filtrada))
	}

	// Um Authorization mandado pelo browser não é credencial deste app, e não
	// é ele que chega à API.
	antes = len(recebidasBench(t, base))
	c.Get("/painel/documentos", trilha.WithHeader("Authorization", "Bearer roubado")).WantStatus(200)
	for _, r := range desde(t, base, antes) {
		if strings.Contains(r.Auth, "roubado") {
			t.Fatalf("o token que o browser mandou chegou à API: %q", r.Auth)
		}
	}

	// Uma 'use client' portada que trouxe o JavaScript junto não foi portada.
	// Os arquivos do kit são a exceção: são eles que fazem o resto funcionar
	// sem JavaScript escrito aqui.
	if strings.Contains(body, "data-trilha-island") {
		t.Error("a tela carregou uma ilha: esta listagem cabe inteira no servidor")
	}
	for _, src := range scriptsBench(body) {
		if !strings.Contains(src, "/ui.") {
			t.Errorf("apareceu JavaScript próprio na página: %s", src)
		}
	}
}

// scriptsBench returns the src of every script the page loads.
func scriptsBench(body string) []string {
	var out []string
	resto := body
	for {
		i := strings.Index(resto, "<script")
		if i < 0 {
			return out
		}
		resto = resto[i+7:]
		fim := strings.IndexByte(resto, '>')
		if fim < 0 {
			return out
		}
		tag := resto[:fim]
		resto = resto[fim:]
		k := strings.Index(tag, "src=\"")
		if k < 0 {
			continue // inline: não é o que este teste mede
		}
		src := tag[k+5:]
		if f := strings.IndexByte(src, '"'); f >= 0 {
			src = src[:f]
		}
		out = append(out, src)
	}
}

func trecho(s string) string {
	if len(s) > 800 {
		return s[:800]
	}
	return s
}
`
