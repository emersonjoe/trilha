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
			"en": "organisations: create, switch, activate or not, members, per-organisation settings",
			"pt": "organizações: criar, trocar, ativar ou não, membros, configuração por organização",
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
			"en": "Run `trilha dev` and open {{.URL}}organizacoes: create one, switch, deactivate, " +
				"and save its settings. Then two things: put " +
				"sessao.Flow.RequireTenant() in the middleware.go of everything that is per-organisation, " +
				"and put `WHERE tenant_id = ?` with auth.Tenant(c) in every query that reads rows. The " +
				"clause is yours — one this framework generated would be one nobody reads in a review.",
			"pt": "Rode `trilha dev` e abra {{.URL}}organizacoes: crie uma, troque, desative, " +
				"e salve a configuração dela. Depois, duas coisas: ponha " +
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
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/emersonjoe/trilha"
)

// Organizacao is one row. An inactive one keeps its rows and its members and
// lets nobody in: deactivating is what happens to a customer who stopped
// paying, and deleting is what happens to one who asked for it in writing.
type Organizacao struct {
	ID    string
	Nome  string
	Ativa bool
}

// Configuracao is what one organisation may set for itself, apart from the
// application's own settings. The struct is the screen, as in every
// trilha.Settings: labels from the tags, controls from the types.
type Configuracao struct {
	Fuso   string ` + "`form:\"fuso\" label:\"{{.T.tenant_cfg_tz}}\" validate:\"required,max=40\"`" + `
	Limite int    ` + "`form:\"limite\" label:\"{{.T.tenant_cfg_limit}}\" validate:\"required,min=1,max=1000\"`" + `
}

var (
	// ErrNomeVazio is a create with nothing in the name.
	ErrNomeVazio = errors.New("organizacoes: the name is empty")
	// ErrJaExiste is a create whose name maps to an id that is taken.
	ErrJaExiste = errors.New("organizacoes: an organisation with that id already exists")
	// ErrNaoExiste is an id nobody has.
	ErrNaoExiste = errors.New("organizacoes: no such organisation")
)

// Store is the table, plus who belongs to what, plus each organisation's
// settings section.
type Store struct {
	// Configs is where each organisation's settings are saved. Nil keeps them
	// in memory, which is what a fresh project has; give it the same store
	// the application's own settings use once there is one.
	Configs trilha.SettingsStore

	mu      sync.RWMutex
	rows    map[string]Organizacao
	membros map[string]map[string]bool // subject -> organisation -> true
	configs map[string]*trilha.Settings[Configuracao]
}

// Novo seeds two organisations, which is the smallest number that makes
// switching mean anything.
func Novo() *Store {
	s := &Store{
		rows:    map[string]Organizacao{},
		membros: map[string]map[string]bool{},
		configs: map[string]*trilha.Settings[Configuracao]{},
	}
	s.Add("acme", "ACME")
	s.Add("globex", "Globex")
	return s
}

// Add records an organisation with the id given, active.
func (s *Store) Add(id, nome string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id] = Organizacao{ID: id, Nome: nome, Ativa: true}
}

// Criar records a new organisation from its name; the id is the name as a
// slug. A name that slugs to an id already taken is refused rather than
// renumbered: two "ACME" is a mistake somebody should see.
func (s *Store) Criar(nome string) (Organizacao, error) {
	nome = strings.TrimSpace(nome)
	id := slug(nome)
	if id == "" {
		return Organizacao{}, ErrNomeVazio
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[id]; ok {
		return Organizacao{}, ErrJaExiste
	}
	o := Organizacao{ID: id, Nome: nome, Ativa: true}
	s.rows[id] = o
	return o, nil
}

// Ativar lets people in again.
func (s *Store) Ativar(id string) error { return s.ativa(id, true) }

// Desativar keeps everything and lets nobody switch into it. Whoever is in it
// now stays until the session ends: a switch is the moment the check runs.
func (s *Store) Desativar(id string) error { return s.ativa(id, false) }

func (s *Store) ativa(id string, v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.rows[id]
	if !ok {
		return ErrNaoExiste
	}
	o.Ativa = v
	s.rows[id] = o
	return nil
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
// organisation. An inactive organisation says no to everybody.
func (s *Store) Pode(subject, org string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.rows[org]
	if !ok || !o.Ativa {
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

// Todas is every organisation, active or not, by name.
func (s *Store) Todas() []Organizacao {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Organizacao, 0, len(s.rows))
	for _, o := range s.rows {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out
}

// Membros is who belongs to an organisation, by subject.
func (s *Store) Membros(org string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for subject, orgs := range s.membros {
		if orgs[org] {
			out = append(out, subject)
		}
	}
	sort.Strings(out)
	return out
}

// Config is the settings section of one organisation, bound the first time it
// is asked for. One section per organisation, under its own key, is what keeps
// ACME's time zone from becoming everybody's.
func (s *Store) Config(org string) *trilha.Settings[Configuracao] {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg, ok := s.configs[org]; ok {
		return cfg
	}
	cfg := trilha.NewSettings("org:"+org, Configuracao{Fuso: "America/Sao_Paulo", Limite: 50})
	// Bind without an app: the section is a value of this store, not a
	// singleton the app owns, and nothing it needs from the app is used here.
	_ = cfg.Bind(nil, s.Configs)
	s.configs[org] = cfg
	return cfg
}

// slug is the id a name becomes: lower case, letters and digits, one dash
// between words. Accents are dropped as letters, not translated — an id is
// something typed in a URL, and "ação" and "acao" would otherwise be two.
func slug(nome string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(nome) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
`

const tenantStoreTest = `package organizacoes

import (
	"errors"
	"testing"
)

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
	if m := s.Membros("acme"); len(m) != 1 || m[0] != "u-1" {
		t.Fatalf("membros = %v", m)
	}
}

// Criar dá o id pelo nome e recusa o repetido; desativar guarda tudo e não
// deixa ninguém entrar.
func TestCriarEDesativar(t *testing.T) {
	s := Novo()
	o, err := s.Criar("  Ação & Cia  ")
	if err != nil || o.ID != "a-o-cia" || !o.Ativa {
		t.Fatalf("criar = %+v, %v", o, err)
	}
	if _, err := s.Criar("ACME"); !errors.Is(err, ErrJaExiste) {
		t.Fatalf("repetida: %v", err)
	}
	if _, err := s.Criar("   "); !errors.Is(err, ErrNomeVazio) {
		t.Fatalf("vazia: %v", err)
	}
	if err := s.Desativar("ninguem"); !errors.Is(err, ErrNaoExiste) {
		t.Fatalf("inexistente: %v", err)
	}

	s.Entrar("u-1", o.ID)
	if err := s.Desativar(o.ID); err != nil {
		t.Fatal(err)
	}
	if s.Pode("u-1", o.ID) {
		t.Error("entrou numa organização desativada")
	}
	if n := len(s.Todas()); n != 3 {
		t.Errorf("desativar apagou: %d organizações", n)
	}
	if err := s.Ativar(o.ID); err != nil || !s.Pode("u-1", o.ID) {
		t.Errorf("reativar não devolveu a entrada: %v", err)
	}
}

// A configuração é por organização: a de uma não vaza para a outra.
func TestConfigPorOrganizacao(t *testing.T) {
	s := Novo()
	if err := s.Config("acme").Set(Configuracao{Fuso: "UTC", Limite: 10}); err != nil {
		t.Fatal(err)
	}
	if got := s.Config("acme").Get().Fuso; got != "UTC" {
		t.Errorf("acme = %q", got)
	}
	if got := s.Config("globex").Get().Fuso; got == "UTC" {
		t.Error("a configuração da ACME apareceu na Globex")
	}
}
`

const tenantPage = `// Package organizacoes is the screen where somebody picks which organisation
// they are in, and switches — and, since it is the one screen about
// organisations, where one is created, deactivated, and configured.
//
// It asks for a session and nothing else — see middleware.go. Guarding it with
// RequireTenant would be guarding the screen where the tenant is chosen with
// the rule that there has to be one already: a loop with no way out.
//
// Everybody signed in can create and deactivate here. That is right for a
// project of one team and wrong for a product: when there is a permissions
// recipe, the two forms below are where RequirePolicy goes.
package organizacoes

import (
	"errors"
	"fmt"
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
	linhas := make([]h.Node, 0)
	for _, o := range store.Todas() {
		linhas = append(linhas, linha(c, store, u.Subject, o, atual))
	}
	partes := []h.Node{
		ui.PageHeader("{{.T.tenant_title}}"),
		ui.Muted(h.Text("{{.T.tenant_desc}}")),
		ui.Table(
			h.Thead(h.Tr(h.Th(h.Text("{{.T.tenant_name}}")), h.Th(h.Text("{{.T.tenant_members}}")), h.Th(h.Text("")), h.Th(h.Text("")))),
			h.Tbody(linhas...)),
		criarForm(c),
	}
	if atual != "" {
		partes = append(partes, configForm(c, store, atual))
	}
	return ui.Stack(partes...), nil
}

func linha(c *trilha.Ctx, store *organizacoes.Store, subject string, o organizacoes.Organizacao, atual string) h.Node {
	nome := []h.Node{h.Text(o.Nome)}
	if !o.Ativa {
		nome = append(nome, h.Text(" "), ui.Badge(ui.Outline(), h.Text("{{.T.tenant_inactive}}")))
	}
	if o.ID == atual {
		nome = append(nome, h.Text(" "), ui.Badge(h.Text("{{.T.tenant_current}}")))
	}
	var trocar h.Node
	if store.Pode(subject, o.ID) && o.ID != atual {
		trocar = acaoForm(c, "switch", o.ID, ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.tenant_switch}}")))
	}
	var alternar h.Node
	if o.Ativa {
		alternar = acaoForm(c, "deactivate", o.ID, ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.tenant_deactivate}}"),
			ui.Confirm("{{.T.tenant_deactivate_ask}}", "{{.T.tenant_deactivate_desc}}")))
	} else {
		alternar = acaoForm(c, "activate", o.ID, ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.tenant_activate}}")))
	}
	return h.Tr(
		h.Td(nome...),
		h.Td(h.Text(fmt.Sprint(len(store.Membros(o.ID))))),
		h.Td(trocar),
		h.Td(alternar),
	)
}

// acaoForm is one button: a POST to this same page with what to do and to
// what. One route, four actions, because the page that owns the list is the
// one that owns its changes.
func acaoForm(c *trilha.Ctx, acao, org string, botao h.Node) h.Node {
	return h.Form(h.Method("post"), h.Action("{{.URL}}organizacoes"), h.Class("ui-inline-form"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("_action"), h.Value(acao)),
		h.Input(h.Type("hidden"), h.Name("org"), h.Value(org)),
		botao)
}

func criarForm(c *trilha.Ctx) h.Node {
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.tenant_create}}")),
		ui.CardContent(h.Form(h.Method("post"), h.Action("{{.URL}}organizacoes"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("_action"), h.Value("create")),
			ui.Field("nome", "{{.T.tenant_name}}", ui.Input(h.ID("nome"), h.Name("nome"), h.Required())),
			ui.Submit(h.Text("{{.T.tenant_create}}")),
		)))
}

// configForm is the settings of the organisation the session is in, drawn
// from the struct as every settings screen is. It posts to this page with its
// own _action; the submit is passed in so the hidden field goes with it.
func configForm(c *trilha.Ctx, store *organizacoes.Store, org string) h.Node {
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.tenant_config}}")),
		ui.CardContent(ui.SettingsForm(c, store.Config(org), nil,
			h.Input(h.Type("hidden"), h.Name("_action"), h.Value("config")),
			ui.Submit(h.Text("{{.T.tenant_save}}")),
		)))
}

// POST is every button of the page: create, activate, deactivate, save the
// settings, and switch — which is the default, so a form that posts just
// "org" still switches.
//
// The membership check on the switch is here and not in the framework on
// purpose: auth.SwitchTenant does not know what a membership is in this
// application, and pretending to would be a check that looks like a guarantee
// and is not one. Without this line the screen is a URL that moves anybody
// into any organisation.
func POST(c *trilha.Ctx) error {
	u := sessao.Atual(c)
	if u == nil {
		return trilha.Errorf(http.StatusUnauthorized, "%s", "{{.T.tenant_gone}}")
	}
	store := trilha.Use[*organizacoes.Store](c)
	org := c.Form("org")
	switch c.Form("_action") {
	case "create":
		o, err := store.Criar(c.Form("nome"))
		switch {
		case errors.Is(err, organizacoes.ErrNomeVazio):
			return trilha.FieldErrors{"nome": "{{.T.tenant_name_empty}}"}
		case errors.Is(err, organizacoes.ErrJaExiste):
			return trilha.FieldErrors{"nome": "{{.T.tenant_exists}}"}
		case err != nil:
			return err
		}
		// Whoever creates one is in it: an organisation nobody can enter is
		// a row and not a place.
		store.Entrar(u.Subject, o.ID)
		c.Audit("organizacao.create", o.ID, trilha.Fields{"nome": o.Nome})
		c.Flash(ui.FlashSuccess, "{{.T.tenant_created}}")
	case "activate", "deactivate":
		ativar := c.Form("_action") == "activate"
		var err error
		if ativar {
			err = store.Ativar(org)
		} else {
			err = store.Desativar(org)
		}
		if errors.Is(err, organizacoes.ErrNaoExiste) {
			return trilha.ErrNotFound
		} else if err != nil {
			return err
		}
		c.Audit("organizacao."+c.Form("_action"), org, nil)
		c.Flash(ui.FlashSuccess, "{{.T.tenant_toggled}}")
	case "config":
		// Only the organisation the session is in, and only if the person
		// still belongs to it: the tenant in the session is the one whose
		// settings the form showed.
		atual := auth.Tenant(c)
		if atual == "" || !store.Pode(u.Subject, atual) {
			return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.tenant_denied}}")
		}
		return store.Config(atual).Update(c)
	default:
		if !store.Pode(u.Subject, org) {
			return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.tenant_denied}}")
		}
		// The switch is audited on both sides — from where, to where. An
		// investigation that starts at "they saw the wrong rows" begins by
		// asking when they changed.
		if err := sessao.Flow.SwitchTenant(c, org); err != nil {
			return err
		}
		c.Flash(ui.FlashSuccess, "{{.T.tenant_switched}}")
	}
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
	"fmt"
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

	a.Register(trilha.Route{Pattern: "/_teste/config", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				cfg := trilha.Use[*organizacoes.Store](c).Config(c.Query("org")).Get()
				return c.Text(http.StatusOK, fmt.Sprintf("%s %d", cfg.Fuso, cfg.Limite))
			},
		}})

	c := trilha.NewTestClient(t, a)
	c.PostForm(sessao.Flow.LoginPath(), url.Values{
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

	// Com uma organização na sessão, a tela mostra a configuração dela, e
	// salvar vale só para ela.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"config"}, "fuso": {"UTC"}, "limite": {"5"}}).WantStatus(http.StatusSeeOther)
	if got := c.Get("/_teste/config?org=acme").WantStatus(http.StatusOK).Body.String(); got != "UTC 5" {
		t.Fatalf("config da acme = %q", got)
	}
	if got := c.Get("/_teste/config?org=globex").WantStatus(http.StatusOK).Body.String(); got == "UTC 5" {
		t.Fatal("a configuração da ACME vazou para a Globex")
	}
	// Um valor fora das regras do struct volta 422, e nada é salvo.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"config"}, "fuso": {"UTC"}, "limite": {"5000"}}).WantStatus(http.StatusUnprocessableEntity)

	// Criar: quem cria já está dentro; nome repetido é recusado.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"create"}, "nome": {"Nova Org"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"create"}, "nome": {"nova org"}}).WantStatus(http.StatusUnprocessableEntity)
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"nova-org"}}).WantStatus(http.StatusSeeOther)

	// Desativada, ninguém troca para ela; reativada, sim.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"deactivate"}, "org": {"acme"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"acme"}}).WantStatus(http.StatusForbidden)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"activate"}, "org": {"acme"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"acme"}}).WantStatus(http.StatusSeeOther)
}
`
