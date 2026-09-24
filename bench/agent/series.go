package main

// The savings series (spec 159): four scenarios the plan adds and the four v1
// ones that have an honest stdlib twin. Each pairs the task as Trilha asks it
// with the same task in Go puro — that pairing is what a saving is measured
// against — and every prompt of both sides is frozen under prompts/.

// vendasGo is the data package both sides of s7-tela start from: the screen is
// the task, not the data.
const vendasGo = `// Package vendas holds this month's sales. It is memory here, which is the
// honest starting point: the rows last as long as the process, and a real
// store arrives behind the same function.
package vendas

// Venda is one row: what was sold, to whom, in which state, for how much.
type Venda struct {
	ID      string
	Cliente string
	Estado  string
	Total   float64
}

// Todas returns every sale of the month, oldest first.
func Todas() []Venda {
	return []Venda{
		{"v-01", "Ana Souza", "SP", 120.50},
		{"v-02", "Ana Souza", "SP", 89.90},
		{"v-03", "Ana Souza", "SP", 310.00},
		{"v-04", "Ana Souza", "SP", 74.20},
		{"v-05", "Bruno Lima", "RJ", 205.10},
		{"v-06", "Bruno Lima", "RJ", 42.00},
		{"v-07", "Bruno Lima", "RJ", 178.35},
		{"v-08", "Bruno Lima", "RJ", 99.99},
		{"v-09", "Carla Dias", "MG", 260.40},
		{"v-10", "Carla Dias", "MG", 55.75},
		{"v-11", "Carla Dias", "MG", 133.10},
		{"v-12", "Carla Dias", "MG", 88.88},
	}
}
`

var s5Login = Scenario{
	Name:             "s5-login",
	Title:            "instalar login funcional num app vazio",
	Example:          "bench/agent/apps/s5-login",
	AppDir:           "bench/agent/apps/s5-login",
	ID:               "s5-login",
	PromptMD:         "prompts/s5-login.md",
	Prompt:           mustPrompt("prompts/s5-login.md"),
	Gate:             []string{"go vet ./...", "go test ./..."},
	BaseDir:          "bench/agent/baseline/s5-login",
	BaselinePromptMD: "prompts/s5-login.baseline.md",
	BaselineGate:     []string{"go vet ./...", "go test ./..."},
	BaselineTests:    map[string]string{"zz_bench_test.go": baselineS5LoginTest},
	Tests:            map[string]string{"zz_bench_test.go": s5LoginTest},
}

var s6CRUD = Scenario{
	Name:             "s6-crud",
	Title:            "recurso CRUD completo: API JSON e página com validação",
	Example:          "bench/agent/apps/s6-crud",
	AppDir:           "bench/agent/apps/s6-crud",
	ID:               "s6-crud",
	PromptMD:         "prompts/s6-crud.md",
	Prompt:           mustPrompt("prompts/s6-crud.md"),
	Gate:             []string{"go vet ./...", "go test ./..."},
	BaseDir:          "bench/agent/baseline/s6-crud",
	BaselinePromptMD: "prompts/s6-crud.baseline.md",
	BaselineGate:     []string{"go vet ./...", "go test ./..."},
	BaselineTests:    map[string]string{"zz_bench_test.go": baselineS6CRUDTest},
	Tests:            map[string]string{"zz_bench_test.go": s6CRUDTest},
}

var s7Tela = Scenario{
	Name:             "s7-tela",
	Title:            "tela de listagem com filtro, paginação e gráfico",
	Example:          "bench/agent/apps/s7-tela",
	AppDir:           "bench/agent/apps/s7-tela",
	ID:               "s7-tela",
	PromptMD:         "prompts/s7-tela.md",
	Prompt:           mustPrompt("prompts/s7-tela.md"),
	Gate:             []string{"go vet ./...", "go test ./..."},
	BaseDir:          "bench/agent/baseline/s7-tela",
	BaselinePromptMD: "prompts/s7-tela.baseline.md",
	BaselineGate:     []string{"go vet ./...", "go test ./..."},
	BaselineTests:    map[string]string{"zz_bench_test.go": baselineS7TelaTest},
	Tests:            map[string]string{"zz_bench_test.go": s7TelaTest},
}

// s8Conserto has its three defects committed, and the fixture's own tests are
// red on purpose: the gate is `trilha check` going green, and the ruler counts
// the rounds it took. There is no hidden test to copy in — the app carries the
// proof that the fixes are the real ones.
var s8Conserto = Scenario{
	Name:             "s8-conserto",
	Title:            "consertar três erros guiado pelo check",
	Example:          "bench/agent/apps/s8-conserto",
	AppDir:           "bench/agent/apps/s8-conserto",
	ID:               "s8-conserto",
	PromptMD:         "prompts/s8-conserto.md",
	Prompt:           mustPrompt("prompts/s8-conserto.md"),
	Gate:             []string{"trilha check", "go test ./..."},
	MaxRounds:        40,
	BaseDir:          "bench/agent/baseline/s8-conserto",
	BaselinePromptMD: "prompts/s8-conserto.baseline.md",
	BaselineGate:     []string{"go vet ./...", "go test ./..."},
}

// The hidden tests of the trilha side. They run in the fixture's package main,
// against the app the generated file builds, exactly like the v1 ones.

const s5LoginTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/emersonjoe/trilha"
)

func TestBenchS5Login(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@exemplo.com")
	t.Setenv("ADMIN_PASSWORD", "senha-trilha-2026")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := trilha.NewTestClient(t, newApp())

	// The screen exists and invites.
	c.Get("/entrar", trilha.WithHeader("Accept", "text/html")).WantStatus(http.StatusOK)

	// A wrong password is refused with the form, and no session is opened.
	rec := c.PostForm("/entrar", url.Values{"email": {"admin@exemplo.com"}, "password": {"errada"}}).
		WantStatus(http.StatusUnprocessableEntity)
	if rec.Cookie("trilha_session") != nil {
		t.Fatal("a wrong password opened a session")
	}

	// The right one opens it: a redirect, and the session on the client.
	c.PostForm("/entrar", url.Values{"email": {"admin@exemplo.com"}, "password": {"senha-trilha-2026"}}).
		WantStatus(http.StatusSeeOther)

	// And signing out closes it.
	c.PostForm("/sair", url.Values{}).WantStatus(http.StatusSeeOther)
}
`

const s6CRUDTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/emersonjoe/trilha"
)

func TestBenchS6CRUD(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()

	// The JSON API: create, refuse, list.
	trilha.TestRequest(t, a, "POST", "/api/produtos",
		trilha.WithJSON(map[string]any{"nome": "Teclado", "preco": 129.9})).
		WantStatus(http.StatusCreated)
	trilha.TestRequest(t, a, "POST", "/api/produtos",
		trilha.WithJSON(map[string]any{"nome": "", "preco": -1})).
		WantStatus(http.StatusUnprocessableEntity)
	var list []map[string]any
	trilha.TestRequest(t, a, "GET", "/api/produtos").WantStatus(http.StatusOK).JSON(&list)
	found := false
	for _, p := range list {
		if p["nome"] == "Teclado" {
			found = true
		}
	}
	if !found {
		t.Fatalf("GET /api/produtos = %v, want the Teclado in it", list)
	}

	// The screen: a form, per-field errors on invalid, a redirect on valid.
	c := trilha.NewTestClient(t, a)
	c.Get("/produtos").WantStatus(http.StatusOK).WantContains("<form")
	c.PostForm("/produtos", url.Values{"nome": {""}, "preco": {"-1"}}).
		WantStatus(http.StatusUnprocessableEntity)
	c.PostForm("/produtos", url.Values{"nome": {"Monitor"}, "preco": {"899.00"}}).
		WantStatus(http.StatusSeeOther)
	c.Get("/produtos").WantStatus(http.StatusOK).WantContains("Monitor")
}
`

const s7TelaTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// linhas counts the table rows: one cell of the body carries the column's
// data-label, so it appears once per row.
func linhas(body string) int { return strings.Count(body, ` + "`" + `data-label="Cliente"` + "`" + `) }

func TestBenchS7Tela(t *testing.T) {
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "segredo-de-teste-com-mais-de-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()

	// The columns the task named, and the chart drawing the totals.
	body := trilha.TestRequest(t, a, "GET", "/relatorios").WantStatus(http.StatusOK).Body.String()
	for _, quero := range []string{"Cliente", "Estado", "Total", "SP", "RJ", "MG"} {
		if !strings.Contains(body, quero) {
			t.Fatalf("a tela não mostra %q", quero)
		}
	}
	if !strings.Contains(body, "<svg") || !strings.Contains(body, "ui-chart-label") {
		t.Fatalf("sem gráfico de barras na tela:\n%s", trechoS7(body))
	}

	// Twelve sales, five per page: page 1 lists 5 and links on; page 2 links
	// both ways; page 3 lists the last 2.
	if n := linhas(body); n != 5 {
		t.Fatalf("página 1 lista %d linhas, queria 5", n)
	}
	if !strings.Contains(body, "page=2") {
		t.Fatal("a página 1 não aponta para a 2")
	}
	p2 := trilha.TestRequest(t, a, "GET", "/relatorios?page=2").WantStatus(http.StatusOK).Body.String()
	if n := linhas(p2); n != 5 {
		t.Fatalf("página 2 lista %d linhas, queria 5", n)
	}
	if !strings.Contains(p2, "page=1") || !strings.Contains(p2, "page=3") {
		t.Fatal("a página 2 perdeu os vizinhos")
	}
	p3 := trilha.TestRequest(t, a, "GET", "/relatorios?page=3").WantStatus(http.StatusOK).Body.String()
	if n := linhas(p3); n != 2 {
		t.Fatalf("página 3 lista %d linhas, queria 2", n)
	}

	// The search reads the URL and reaches the table: Bruno's rows stay,
	// Ana's do not survive the filter.
	filtrada := trilha.TestRequest(t, a, "GET", "/relatorios?q=Bruno").WantStatus(http.StatusOK).Body.String()
	if strings.Contains(filtrada, "Ana Souza") {
		t.Fatal("o filtro ?q deixou passar o que não era dele")
	}
	if !strings.Contains(filtrada, "Bruno Lima") {
		t.Fatal("o filtro escondeu o que devia mostrar")
	}
}

func trechoS7(s string) string {
	if len(s) > 800 {
		return s[:800]
	}
	return s
}
`
