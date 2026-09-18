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

// Unidade is one node of the organisation's own hierarchy: a secretariat, a
// department, a sector. Caminho is the whole path from the top —
// "sec-adm/protocolo" — because everything asked below the organisation is
// asked about a path: who is in it, what is under it, who sees which.
type Unidade struct {
	Caminho string
	Nome    string
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
	// unidades is the hierarchy of each organisation, by path, and lotacoes is
	// who works in which of them. The session carries the paths; this is where
	// they come from.
	unidades map[string]map[string]Unidade // organisation -> path -> unit
	lotacoes map[string]map[string][]string // subject -> organisation -> paths
}

// Novo seeds two organisations, which is the smallest number that makes
// switching mean anything.
func Novo() *Store {
	s := &Store{
		rows:     map[string]Organizacao{},
		membros:  map[string]map[string]bool{},
		configs:  map[string]*trilha.Settings[Configuracao]{},
		unidades: map[string]map[string]Unidade{},
		lotacoes: map[string]map[string][]string{},
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

// Unidades is the hierarchy of one organisation, by path, so a parent always
// comes before its children — which is what lets a screen build the tree in
// one pass.
func (s *Store) Unidades(org string) []Unidade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Unidade, 0, len(s.unidades[org]))
	for _, u := range s.unidades[org] {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Caminho < out[j].Caminho })
	return out
}

// CriarUnidade records a unit under pai — the empty path being the top of the
// organisation. The last segment is the name as a slug, so the path is
// something a person can read in an audit line and type in a URL.
func (s *Store) CriarUnidade(org, pai, nome string) (Unidade, error) {
	nome = strings.TrimSpace(nome)
	seg := slug(nome)
	if seg == "" {
		return Unidade{}, ErrNomeVazio
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[org]; !ok {
		return Unidade{}, ErrNaoExiste
	}
	pai = strings.Trim(strings.TrimSpace(pai), "/")
	if pai != "" {
		if _, ok := s.unidades[org][pai]; !ok {
			return Unidade{}, ErrNaoExiste
		}
	}
	caminho := seg
	if pai != "" {
		caminho = pai + "/" + seg
	}
	if s.unidades[org] == nil {
		s.unidades[org] = map[string]Unidade{}
	}
	if _, ok := s.unidades[org][caminho]; ok {
		return Unidade{}, ErrJaExiste
	}
	u := Unidade{Caminho: caminho, Nome: nome}
	s.unidades[org][caminho] = u
	return u, nil
}

// RemoverUnidade takes a unit and everything under it, and takes those paths
// off whoever was in them. A child left behind would be a unit nobody can
// reach and a filter nobody can explain.
func (s *Store) RemoverUnidade(org, caminho string) error {
	caminho = strings.Trim(strings.TrimSpace(caminho), "/")
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.unidades[org][caminho]; !ok {
		return ErrNaoExiste
	}
	for p := range s.unidades[org] {
		if p == caminho || strings.HasPrefix(p, caminho+"/") {
			delete(s.unidades[org], p)
		}
	}
	for subject, porOrg := range s.lotacoes {
		var ficam []string
		for _, p := range porOrg[org] {
			if p != caminho && !strings.HasPrefix(p, caminho+"/") {
				ficam = append(ficam, p)
			}
		}
		s.lotacoes[subject][org] = ficam
	}
	return nil
}

// Lotar is who works where: the paths that go into the session as
// auth.User.Units. An unknown path is refused rather than stored — a unit
// filter is only worth something if the path on the left exists.
func (s *Store) Lotar(subject, org string, caminhos ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range caminhos {
		if _, ok := s.unidades[org][strings.Trim(c, "/")]; !ok {
			return ErrNaoExiste
		}
	}
	if s.lotacoes[subject] == nil {
		s.lotacoes[subject] = map[string][]string{}
	}
	limpos := make([]string, 0, len(caminhos))
	for _, c := range caminhos {
		limpos = append(limpos, strings.Trim(c, "/"))
	}
	sort.Strings(limpos)
	s.lotacoes[subject][org] = limpos
	return nil
}

// Lotacao is the units somebody works in, inside one organisation.
func (s *Store) Lotacao(subject, org string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.lotacoes[subject][org]...)
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

// A árvore da organização: criar uma filha, lotar alguém nela, e remover um
// galho leva tudo que está abaixo — inclusive a lotação de quem estava lá.
func TestUnidadesDaOrganizacao(t *testing.T) {
	s := Novo()
	sec, err := s.CriarUnidade("acme", "", "Secretaria de Administracao")
	if err != nil || sec.Caminho != "secretaria-de-administracao" {
		t.Fatalf("criar raiz = %+v, %v", sec, err)
	}
	filha, err := s.CriarUnidade("acme", sec.Caminho, "Protocolo")
	if err != nil || filha.Caminho != sec.Caminho+"/protocolo" {
		t.Fatalf("criar filha = %+v, %v", filha, err)
	}
	if _, err := s.CriarUnidade("acme", sec.Caminho, "protocolo"); !errors.Is(err, ErrJaExiste) {
		t.Fatalf("repetida: %v", err)
	}
	if _, err := s.CriarUnidade("acme", "inventada", "X"); !errors.Is(err, ErrNaoExiste) {
		t.Fatalf("pai inexistente: %v", err)
	}
	// O pai vem antes da filha, que é o que deixa a tela montar a árvore numa
	// passada só.
	us := s.Unidades("acme")
	if len(us) != 2 || us[0].Caminho != sec.Caminho || us[1].Caminho != filha.Caminho {
		t.Fatalf("unidades = %+v", us)
	}
	if len(s.Unidades("globex")) != 0 {
		t.Error("a árvore de uma organização apareceu na outra")
	}

	if err := s.Lotar("u-1", "acme", filha.Caminho); err != nil {
		t.Fatal(err)
	}
	if got := s.Lotacao("u-1", "acme"); len(got) != 1 || got[0] != filha.Caminho {
		t.Fatalf("lotação = %v", got)
	}
	if err := s.Lotar("u-1", "acme", "nao-existe"); !errors.Is(err, ErrNaoExiste) {
		t.Fatalf("lotar em unidade inexistente: %v", err)
	}

	if err := s.RemoverUnidade("acme", sec.Caminho); err != nil {
		t.Fatal(err)
	}
	if len(s.Unidades("acme")) != 0 {
		t.Error("remover o galho deixou a filha para trás")
	}
	if got := s.Lotacao("u-1", "acme"); len(got) != 0 {
		t.Errorf("a lotação sobreviveu à unidade: %v", got)
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
	"strings"

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
		partes = append(partes, unidadesCard(c, store, atual), configForm(c, store, atual))
	}
	// The tree browses with no JavaScript at all — each node is a <details> —
	// and the script only adds the keyboard.
	partes = append(partes, ui.TreeScript(c))
	return ui.Stack(partes...), nil
}

// unidadesCard is the organisation's own hierarchy: secretariats, departments,
// sectors. The organisation is one column; this is what is asked below it —
// "this analyst sees their own sector", "the secretary sees everything under
// them" — and the answer is a path, which is why a unit is one.
//
// The paths go into the session as auth.User.Units, and the policy compares
// them with auth.ScopeUnit and auth.ScopeUnitTree. Nothing here writes a
// WHERE: the query is the application's, like the tenant column above it.
func unidadesCard(c *trilha.Ctx, store *organizacoes.Store, org string) h.Node {
	unidades := store.Unidades(org)
	nodes := arvore(unidades)
	corpo := []h.Node{
		ui.Muted(h.Text("{{.T.unit_desc}}")),
	}
	if len(nodes) == 0 {
		corpo = append(corpo, ui.Muted(h.Text("{{.T.unit_none}}")))
	} else {
		corpo = append(corpo, ui.Tree(ui.TreeOpts{Nodes: nodes, Label: "{{.T.unit_title}}"}), removerLista(c, unidades))
	}
	corpo = append(corpo, criarUnidadeForm(c, nodes), lotarForm(c, store, org, nodes))
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.unit_title}}")),
		ui.CardContent(ui.Stack(corpo...)))
}

// arvore turns the flat paths into the tree the kit draws. The store hands
// them back with every parent before its children, so one pass is enough.
func arvore(us []organizacoes.Unidade) []ui.TreeNode {
	filhas := map[string][]organizacoes.Unidade{}
	for _, u := range us {
		pai := ""
		if i := strings.LastIndex(u.Caminho, "/"); i >= 0 {
			pai = u.Caminho[:i]
		}
		filhas[pai] = append(filhas[pai], u)
	}
	var monta func(string) []ui.TreeNode
	monta = func(pai string) []ui.TreeNode {
		var out []ui.TreeNode
		for _, u := range filhas[pai] {
			kids := monta(u.Caminho)
			out = append(out, ui.TreeNode{
				Value:    u.Caminho,
				Label:    u.Nome,
				Leaf:     len(kids) == 0,
				Children: kids,
				Open:     true,
			})
		}
		return out
	}
	return monta("")
}

// removerLista is one button per unit. Removing takes the branch with it, and
// the confirmation says so: a sector that disappears with its department is a
// surprise worth one sentence.
func removerLista(c *trilha.Ctx, us []organizacoes.Unidade) h.Node {
	linhas := make([]h.Node, 0, len(us))
	for _, u := range us {
		linhas = append(linhas, h.Tr(
			h.Td(h.Code(h.Text(u.Caminho))),
			h.Td(unidadeForm(c, "unidade-remover", u.Caminho,
				ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.unit_remove}}"),
					ui.Confirm("{{.T.unit_remove_ask}}", "{{.T.unit_remove_desc}}")))),
		))
	}
	return ui.Table(h.Tbody(linhas...))
}

// unidadeForm is one button about one unit, the same shape as acaoForm above.
func unidadeForm(c *trilha.Ctx, acao, caminho string, botao h.Node) h.Node {
	return h.Form(h.Method("post"), h.Action("{{.URL}}organizacoes"), h.Class("ui-inline-form"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("_action"), h.Value(acao)),
		h.Input(h.Type("hidden"), h.Name("unidade"), h.Value(caminho)),
		botao)
}

// criarUnidadeForm adds a child under whatever is picked — nothing picked is
// the top of the organisation.
func criarUnidadeForm(c *trilha.Ctx, nodes []ui.TreeNode) h.Node {
	campos := []h.Node{
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("_action"), h.Value("unidade-criar")),
	}
	if len(nodes) > 0 {
		campos = append(campos, ui.Field("pai", "{{.T.unit_parent}}", ui.TreePicker(ui.TreePickerOpts{
			Name:  "pai",
			Nodes: nodes,
			Label: "{{.T.unit_parent}}",
		})))
	}
	campos = append(campos,
		ui.Field("unidade-nome", "{{.T.tenant_name}}", ui.Input(h.ID("unidade-nome"), h.Name("nome"), h.Required())),
		ui.Submit(h.Text("{{.T.unit_create}}")))
	return h.Form(append([]h.Node{h.Method("post"), h.Action("{{.URL}}organizacoes"), h.Class("ui-stack")}, campos...)...)
}

// lotarForm is who works where. The units of somebody else's session only
// change when they sign in again — a session is a copy, and this screen has no
// way to reach into it — so the one it can update on the spot is the reader's
// own, which is what it does.
func lotarForm(c *trilha.Ctx, store *organizacoes.Store, org string, nodes []ui.TreeNode) h.Node {
	if len(nodes) == 0 {
		return h.Fragment()
	}
	u := sessao.Atual(c)
	opcoes := make([]ui.Option, 0)
	for _, m := range store.Membros(org) {
		rotulo := m
		if u != nil && m == u.Subject {
			rotulo = m + " ({{.T.unit_member_me}})"
		}
		opcoes = append(opcoes, ui.Option{Value: m, Label: rotulo})
	}
	atual := ""
	if u != nil {
		if lot := store.Lotacao(u.Subject, org); len(lot) > 0 {
			atual = lot[0]
		}
	}
	return h.Form(h.Method("post"), h.Action("{{.URL}}organizacoes"), h.Class("ui-stack"),
		trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("_action"), h.Value("lotar")),
		ui.Muted(h.Text("{{.T.unit_member_desc}}")),
		ui.Field("membro", "{{.T.unit_member_who}}", ui.Select(h.ID("membro"), h.Name("membro"),
			ui.SelectOptions(opcoes, ""))),
		ui.Field("unidade", "{{.T.unit_title}}", ui.TreePicker(ui.TreePickerOpts{
			Name:  "unidade",
			Value: atual,
			Nodes: nodes,
			Label: "{{.T.unit_title}}",
		})),
		ui.Submit(h.Text("{{.T.unit_assign}}")))
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
	case "unidade-criar", "unidade-remover", "lotar":
		// Everything about the hierarchy is about the organisation the session
		// is in, and only for somebody who belongs to it. Without this line
		// the screen is a URL that redraws another organisation's tree.
		atual := auth.Tenant(c)
		if atual == "" || !store.Pode(u.Subject, atual) {
			return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.tenant_denied}}")
		}
		return unidades(c, store, u.Subject, atual)
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

// unidades is the tree's half of POST: create a child, remove a branch, put
// somebody in a unit. It is a function of its own because the switch above was
// already the page's whole story and a hierarchy is a second one.
func unidades(c *trilha.Ctx, store *organizacoes.Store, subject, org string) error {
	switch c.Form("_action") {
	case "unidade-criar":
		un, err := store.CriarUnidade(org, c.Form("pai"), c.Form("nome"))
		switch {
		case errors.Is(err, organizacoes.ErrNomeVazio):
			return trilha.FieldErrors{"nome": "{{.T.tenant_name_empty}}"}
		case errors.Is(err, organizacoes.ErrJaExiste):
			return trilha.FieldErrors{"nome": "{{.T.unit_exists}}"}
		case errors.Is(err, organizacoes.ErrNaoExiste):
			return trilha.ErrNotFound
		case err != nil:
			return err
		}
		c.Audit("unidade.create", un.Caminho, trilha.Fields{"nome": un.Nome})
		c.Flash(ui.FlashSuccess, "{{.T.unit_created}}")
	case "unidade-remover":
		if err := store.RemoverUnidade(org, c.Form("unidade")); errors.Is(err, organizacoes.ErrNaoExiste) {
			return trilha.ErrNotFound
		} else if err != nil {
			return err
		}
		c.Audit("unidade.remove", c.Form("unidade"), nil)
		c.Flash(ui.FlashSuccess, "{{.T.unit_removed}}")
	case "lotar":
		caminho := c.Form("unidade")
		if caminho == "" {
			return trilha.FieldErrors{"unidade": "{{.T.unit_pick}}"}
		}
		membro := c.Form("membro")
		if membro == "" {
			membro = subject
		}
		if !store.Pode(membro, org) {
			return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.tenant_denied}}")
		}
		if err := store.Lotar(membro, org, caminho); errors.Is(err, organizacoes.ErrNaoExiste) {
			return trilha.ErrNotFound
		} else if err != nil {
			return err
		}
		// A session is a copy: somebody else's units change when they sign in
		// again, and the reader's own change now — which is what makes the
		// unit filter answer on the next screen instead of the next login.
		if membro == subject {
			if err := sessao.Flow.Update(c, func(s *auth.User) {
				s.Units = store.Lotacao(membro, org)
			}); err != nil {
				return err
			}
		}
		c.Audit("unidade.lotou", membro, trilha.Fields{"unidade": caminho})
		c.Flash(ui.FlashSuccess, "{{.T.unit_assigned}}")
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
	"strings"
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

	// A unidade da sessão: é o que a política compara com auth.ScopeUnit.
	a.Register(trilha.Route{Pattern: "/_teste/minhas-unidades", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				u := sessao.Atual(c)
				if u == nil {
					return trilha.Errorf(http.StatusUnauthorized, "sem sessão")
				}
				return c.Text(http.StatusOK, strings.Join(u.Units, ","))
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

	// A árvore da organização: uma secretaria, um setor dentro dela, e quem
	// está na sessão lotado no setor — que é o que a política lê depois.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"unidade-criar"}, "nome": {"Administracao"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"unidade-criar"}, "pai": {"administracao"}, "nome": {"Protocolo"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"unidade-criar"}, "pai": {"administracao"}, "nome": {"Protocolo"}}).WantStatus(http.StatusUnprocessableEntity)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"lotar"}, "unidade": {"administracao/protocolo"}}).WantStatus(http.StatusSeeOther)
	if got := c.Get("/_teste/minhas-unidades").WantStatus(http.StatusOK).Body.String(); got != "administracao/protocolo" {
		t.Fatalf("a sessão carrega as unidades %q", got)
	}
	// Remover a secretaria leva o setor e a lotação junto.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"unidade-remover"}, "unidade": {"administracao"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"lotar"}, "unidade": {"administracao/protocolo"}}).WantStatus(http.StatusNotFound)

	// Desativada, ninguém troca para ela; reativada, sim.
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"deactivate"}, "org": {"acme"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"acme"}}).WantStatus(http.StatusForbidden)
	c.PostForm("{{.URL}}organizacoes", url.Values{"_action": {"activate"}, "org": {"acme"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}organizacoes", url.Values{"org": {"acme"}}).WantStatus(http.StatusSeeOther)
}
`
