package recipes

// tenantRecipe is the column that must not be missing, and the screen where
// somebody picks which organisation they are in.
//
// One column is the most common shape of multi-tenant, and forgetting that
// column in one query is its most common bug: the report that shows another
// customer's rows, found by the customer.
func tenantRecipe() Recipe {
	return Recipe{
		Name: "tenant",
		Summary: map[string]string{
			"en": "organisations: choosing one, switching, and where the column goes",
			"pt": "organizações: escolher uma, trocar, e onde a coluna entra",
		},
		Doc:   "/reference/auth",
		Needs: []Need{{Recipe: "login", File: "internal/sessao/sessao.go"}},
		Files: []File{
			{Rel: "internal/organizacoes/organizacoes.go", Go: true, Body: tenantStore},
			{Rel: "internal/organizacoes/organizacoes_test.go", Go: true, Body: tenantStoreTest},
			{Rel: "{{.At}}organizacoes/page.go", Go: true, Body: tenantPage},
			{Rel: "{{.At}}organizacoes/middleware.go", Go: true, Body: tenantMiddleware},
			{Rel: "organizacoes_test.go", Go: true, Body: tenantTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add tenant",
			Line:   "\ttrilha.Provide(a, organizacoes.Novo())\n",
		}},
		Imports: []string{"{{.Module}}/internal/organizacoes"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}organizacoes. Then two things: put " +
				"sessao.Flow.RequireTenant() in the middleware.go of everything that is per-organisation, " +
				"and put `WHERE tenant_id = ?` with auth.Tenant(c) in every query that reads rows. The " +
				"clause is yours — one this framework generated would be one nobody reads in a review.",
			"pt": "Rode `trilha dev` e abra {{.URL}}organizacoes. Depois, duas coisas: ponha " +
				"sessao.Flow.RequireTenant() no middleware.go de tudo que é por organização, e ponha " +
				"`WHERE tenant_id = ?` com auth.Tenant(c) em toda consulta que lê linhas. A cláusula é " +
				"sua — uma que este framework gerasse seria uma que ninguém lê na revisão.",
		},
	}
}

const tenantStore = `// Package organizacoes is who exists and who belongs where.
//
// The framework carries the tenant in the session and refuses a session
// without one; what it cannot know is whether a given person may enter a given
// organisation. That is a membership, it is this application's, and it is the
// table below — auth.SwitchTenant says so in its own documentation, and a
// check that looked like a guarantee and was not would be worse than none.
package organizacoes

import (
	"sort"
	"sync"
)

// Organizacao is one row.
type Organizacao struct {
	ID   string
	Nome string
}

// Store is the table, plus who belongs to what.
type Store struct {
	mu      sync.RWMutex
	rows    map[string]Organizacao
	membros map[string]map[string]bool // subject -> organisation -> true
}

// Novo seeds two organisations, which is the smallest number that makes
// switching mean anything.
func Novo() *Store {
	s := &Store{rows: map[string]Organizacao{}, membros: map[string]map[string]bool{}}
	s.Add("acme", "ACME")
	s.Add("globex", "Globex")
	return s
}

// Add records an organisation.
func (s *Store) Add(id, nome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id] = Organizacao{ID: id, Nome: nome}
}

// Entrar records that somebody belongs to an organisation.
func (s *Store) Entrar(subject, org string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.membros[subject] == nil {
		s.membros[subject] = map[string]bool{}
	}
	s.membros[subject][org] = true
}

// Pode is the check auth.SwitchTenant does not do, and the one that decides
// everything: without it, the switch is a URL that moves anybody into any
// organisation.
func (s *Store) Pode(subject, org string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.rows[org]; !ok {
		return false
	}
	return s.membros[subject][org]
}

// De is what somebody may choose from.
func (s *Store) De(subject string) []Organizacao {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Organizacao, 0, len(s.membros[subject]))
	for id := range s.membros[subject] {
		if o, ok := s.rows[id]; ok {
			out = append(out, o)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out
}
`

const tenantStoreTest = `package organizacoes

import "testing"

// Pertencer é o que decide: sem isso, a troca é uma URL que move qualquer um
// para qualquer organização.
func TestPodeSoOndeEntrou(t *testing.T) {
	s := Novo()
	s.Entrar("u-1", "acme")

	if !s.Pode("u-1", "acme") {
		t.Error("quem entrou não pode")
	}
	if s.Pode("u-1", "globex") {
		t.Error("entrou numa organização de que não participa")
	}
	// E uma organização que não existe não é um caso especial: não pode.
	if s.Pode("u-1", "inventada") {
		t.Error("uma organização inventada foi aceita")
	}
	if orgs := s.De("u-1"); len(orgs) != 1 || orgs[0].ID != "acme" {
		t.Fatalf("de = %+v", orgs)
	}
}
`

const tenantPage = `// Package organizacoes is the screen where somebody picks which organisation
// they are in, and switches.
//
// It asks for a session and nothing else — see middleware.go. Guarding it with
// RequireTenant would be guarding the screen where the tenant is chosen with
// the rule that there has to be one already: a loop with no way out.
package organizacoes

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/organizacoes"
	"{{.Module}}/internal/sessao"
)

// Page renders GET {{.URL}}organizacoes.
func Page(c *trilha.Ctx) (h.Node, error) {
	u := sessao.Atual(c)
	if u == nil {
		return nil, trilha.Errorf(http.StatusUnauthorized, "%s", "{{.T.tenant_gone}}")
	}
	store := trilha.Use[*organizacoes.Store](c)
	c.SetTitle("{{.T.tenant_title}}")

	atual := auth.Tenant(c)
	minhas := store.De(u.Subject)
	linhas := make([]h.Node, 0, len(minhas))
	for _, o := range minhas {
		rotulo := o.Nome
		if o.ID == atual {
			rotulo += " · {{.T.tenant_current}}"
		}
		linhas = append(linhas, h.Li(h.Form(h.Method("post"), h.Action("{{.URL}}organizacoes"),
			h.Class("ui-inline-form"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("org"), h.Value(o.ID)),
			ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text(rotulo)),
		)))
	}
	if len(linhas) == 0 {
		return ui.Stack(
			ui.PageHeader("{{.T.tenant_title}}"),
			ui.Empty(ui.EmptyOpts{Icon: "info", Title: "{{.T.tenant_none}}"}),
		), nil
	}
	return ui.Stack(
		ui.PageHeader("{{.T.tenant_title}}"),
		ui.Muted(h.Text("{{.T.tenant_desc}}")),
		h.Ul(linhas...),
	), nil
}

// POST switches organisation, after checking that the person belongs to it.
//
// The check is here and not in the framework on purpose: auth.SwitchTenant
// does not know what a membership is in this application, and pretending to
// would be a check that looks like a guarantee and is not one. Without this
// line the screen is a URL that moves anybody into any organisation.
func POST(c *trilha.Ctx) error {
	u := sessao.Atual(c)
	if u == nil {
		return trilha.Errorf(http.StatusUnauthorized, "%s", "{{.T.tenant_gone}}")
	}
	org := c.Form("org")
	if !trilha.Use[*organizacoes.Store](c).Pode(u.Subject, org) {
		return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.tenant_denied}}")
	}
	// The switch is audited on both sides — from where, to where. An
	// investigation that starts at "they saw the wrong rows" begins by asking
	// when they changed.
	if err := sessao.Flow.SwitchTenant(c, org); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "{{.T.tenant_switched}}")
	return c.Redirect("{{.URL}}organizacoes")
}
`

const tenantMiddleware = `package organizacoes

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige asks for a session and nothing more.
//
// Not RequireTenant: this is the screen where the organisation is chosen, and
// guarding it with the rule that one must already be chosen is a loop with no
// way out — the same shape as sending somebody who is signed in back to the
// login.
var exige = sessao.Flow.Require()

// Middleware guards this folder.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const tenantTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/organizacoes"
	"{{.Module}}/internal/sessao"
)

// Trocar de organização é trocar o que a pessoa vê: só vale onde ela entrou.
func TestTrocarDeOrganizacao(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()

	// Duas rotas de teste: uma que registra o pertencimento (que é dado do
	// app, e não do framework) e outra que diz qual organização a sessão
	// carrega agora.
	a.Register(trilha.Route{Pattern: "/_teste/entrar-na-org", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error {
				u := sessao.Atual(c)
				if u == nil {
					return trilha.Errorf(http.StatusUnauthorized, "sem sessão")
				}
				trilha.Use[*organizacoes.Store](c).Entrar(u.Subject, c.Form("org"))
				return c.Text(http.StatusOK, "ok")
			},
		}})
	// auth.Tenant lê o usuário que o middleware pôs na requisição, e esta rota
	// de teste não tem middleware nenhum: aqui a sessão é lida direto.
	a.Register(trilha.Route{Pattern: "/_teste/org-atual", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				u := sessao.Atual(c)
				if u == nil {
					return trilha.Errorf(http.StatusUnauthorized, "sem sessão")
				}
				return c.Text(http.StatusOK, u.Tenant)
			},
		}})

	c := trilha.NewTestClient(t, a)
	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)

	// Sem organização nenhuma, a tela responde: é onde se escolhe.
	c.Get("{{.URL}}organizacoes").WantStatus(http.StatusOK)

	// Uma organização de que a pessoa não participa é recusada — e é esta
	// linha que o framework não escreve por você.
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"acme"}}).WantStatus(http.StatusForbidden)

	// Depois de entrar nela, a troca vale, e a sessão passa a carregá-la.
	c.PostForm("/_teste/entrar-na-org", url.Values{"org": {"acme"}}).WantStatus(http.StatusOK)
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"acme"}}).WantStatus(http.StatusSeeOther)
	if got := c.Get("/_teste/org-atual").WantStatus(http.StatusOK).Body.String(); got != "acme" {
		t.Fatalf("a sessão carrega %q", got)
	}
}
`
