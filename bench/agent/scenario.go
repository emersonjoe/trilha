package main

import (
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
}

// Scenarios in the order the table shows them.
func Scenarios() []Scenario {
	return []Scenario{comments, contactForm, cognito, pagination}
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
	"MODULE/internal/posts"
)

func TestBenchComments(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	posts.Seed()
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
	"MODULE/internal/posts"
)

func TestBenchContato(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	posts.Seed()
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
	posts.Seed()
	a := newApp()
	for i := 1; i <= 12; i++ { // 2 seeded + 12 = 14 posts: 5, 5, 4
		posts.Create(fmt.Sprintf("Post %02d", i), "corpo")
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
