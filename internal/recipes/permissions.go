package recipes

// permissionsRecipe is the matrix — a role per row, a module per column — and
// the screen that edits it.
//
// It is the recipe that removes the most code from an application: with a
// matrix, the `if role == "admin"` spread over eighteen files stops being
// written. auth.Policy and ui.PolicyGrid have existed since phase 5; what did
// not exist is the path for somebody who has a project and does not know they
// are there.
func permissionsRecipe() Recipe {
	return Recipe{
		Name: "permissions",
		Summary: map[string]string{
			"en": "the permission matrix as data, the screen that edits it, roles created and removed, who has what",
			"pt": "a matriz de permissões como dado, a tela que a edita, papéis criados e removidos, quem tem o quê",
		},
		Doc:   "/reference/auth",
		Needs: []Need{{Recipe: "login", File: "internal/sessao/sessao.go"}},
		Files: []File{
			{Rel: "internal/acesso/acesso.go", Go: true, Body: permsPolicy},
			{Rel: "internal/acesso/acesso_test.go", Go: true, Body: permsPolicyTest},
			{Rel: "{{.At}}permissoes/page.go", Go: true, Body: permsPage},
			{Rel: "{{.At}}permissoes/middleware.go", Go: true, Body: permsMiddleware},
			{Rel: "permissoes_test.go", Go: true, Body: permsTest},
		},
		// With the users recipe there, its role list is this matrix's: one
		// place declares a role, and the invite form offers it. The line is
		// the same one the users recipe carries, each conditioned on the
		// other's file, so the tie happens whichever is added second.
		Setup: []Insert{{
			Marker:  "// trilha:link users-permissions",
			Line:    "\tusuarios.Papeis = acesso.Papeis\n",
			If:      "internal/usuarios/papeis.go",
			Imports: []string{"{{.Module}}/internal/acesso", "{{.Module}}/internal/usuarios"},
		}},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}permissoes. Then replace the `if role == …` you have " +
				"with acesso.Exige(module, level) in the middleware.go of each folder — the matrix is only " +
				"worth something when it is the one deciding.",
			"pt": "Rode `trilha dev` e abra {{.URL}}permissoes. Depois troque os `if papel == …` que você " +
				"tem por acesso.Exige(modulo, nivel) no middleware.go de cada pasta — a matriz só vale " +
				"alguma coisa quando é ela quem decide.",
		},
	}
}

const permsPolicy = `// Package acesso is who may do what, declared as data in one place.
//
// A matrix is worth more than the rules it replaces: the same three lines
// answer the middleware, the button and the screen that edits it, instead of
// each one growing its own if. What is here is memory — a real one hands
// auth.PolicyStore a table and calls auth.PolicyFrom after saving — and no
// screen changes when that arrives.
package acesso

import (
	"errors"
	"sync"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
)

var (
	// ErrPapelInvalido is a name that is not lower-case letters, digits and
	// dashes: a role is an identifier that ends up in a session and a URL.
	ErrPapelInvalido = errors.New("acesso: a role is lower-case letters, digits and dashes")
	// ErrPapelExiste is a create with a name the matrix already has.
	ErrPapelExiste = errors.New("acesso: that role already exists")
	// ErrPapelEmUso is a delete of a role somebody still has: taking it away
	// would leave them with a role that grants nothing, silently.
	ErrPapelEmUso = errors.New("acesso: somebody still has that role")
)

// Policy is the matrix. Modules are what the application has to protect;
// Levels are ordered, from least to most, and a level implies the ones below.
//
// Edit this to match your application: these three modules are a starting
// point, not a guess about your domain.
var Policy = auth.Policy{
	Modules: []string{"relatorios", "usuarios"},
	Levels:  auth.Levels{"ver", "editar", "administrar"},
	Roles: map[string]auth.Grants{
		"admin":  auth.All("administrar"),
		"editor": {"relatorios": "editar"},
		"leitor": {"relatorios": "ver"},
	},
}

// Rotulos is what each module is called on the screen: the code says "docs",
// the person reading says "Documentos".
var Rotulos = map[string]string{
	"relatorios": "Relatórios",
	"usuarios":   "Usuários",
}

var mu sync.Mutex

// Pode is what a page asks before it draws a button. Hiding is cosmetic: the
// rule that holds is the middleware, because a hidden button is still an
// address somebody can type.
func Pode(u *auth.User, modulo, nivel string) bool {
	mu.Lock()
	defer mu.Unlock()
	return Policy.Can(u, modulo, nivel)
}

// Bind reads what the grid posted.
func Bind(c *trilha.Ctx) (map[string]auth.Grants, error) {
	return auth.BindPolicy(c, Policy)
}

// Salvar swaps the roles of the matrix. The snapshot is deliberate: one
// request must not answer twice — allowed at the middleware, denied at the
// button — so what changes is the whole set of roles, at once.
func Salvar(papeis map[string]auth.Grants) {
	mu.Lock()
	defer mu.Unlock()
	Policy.Roles = papeis
}

// Papeis is the roles the matrix has, in a stable order. It is what a form
// that assigns a role offers, so that the list of roles lives here and
// nowhere else — app/setup.go hands it to the users screen when there is one.
func Papeis() []string {
	mu.Lock()
	defer mu.Unlock()
	return Policy.RolesSorted()
}

// Criar adds a role born with what "leitor" has — the least a role grants
// here — so that a new role is safe before anybody edits its row.
func Criar(nome string) error {
	if !nomeValido(nome) {
		return ErrPapelInvalido
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := Policy.Roles[nome]; ok {
		return ErrPapelExiste
	}
	g := auth.Grants{}
	for m, n := range Policy.Roles["leitor"] {
		g[m] = n
	}
	roles := make(map[string]auth.Grants, len(Policy.Roles)+1)
	for k, v := range Policy.Roles {
		roles[k] = v
	}
	roles[nome] = g
	Policy.Roles = roles
	return nil
}

// Apagar removes a role nobody has. Who has it is this application's table
// and not this package's, so the check comes in as a function: the screen
// passes one that asks the users store.
func Apagar(nome string, emUso func(papel string) bool) error {
	if emUso != nil && emUso(nome) {
		return ErrPapelEmUso
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := Policy.Roles[nome]; !ok {
		return trilha.ErrNotFound
	}
	roles := make(map[string]auth.Grants, len(Policy.Roles))
	for k, v := range Policy.Roles {
		if k != nome {
			roles[k] = v
		}
	}
	Policy.Roles = roles
	return nil
}

func nomeValido(nome string) bool {
	if nome == "" || len(nome) > 30 {
		return false
	}
	for _, r := range nome {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}
`

const permsPolicyTest = `package acesso

import (
	"errors"
	"testing"

	"github.com/emersonjoe/trilha/auth"
)

// Um nível implica os de baixo: quem administra vê, e quem vê não administra.
func TestNivelImplicaOsDeBaixo(t *testing.T) {
	admin := &auth.User{Roles: []string{"admin"}}
	leitor := &auth.User{Roles: []string{"leitor"}}
	if !Pode(admin, "relatorios", "ver") {
		t.Error("quem administra não viu")
	}
	if Pode(leitor, "relatorios", "editar") {
		t.Error("quem só vê editou")
	}
	// E um papel que não está na matriz não tem nada.
	if Pode(&auth.User{Roles: []string{"visitante"}}, "relatorios", "ver") {
		t.Error("um papel que ninguém declarou tem acesso")
	}
}

// Salvar troca a matriz inteira, e o pedido seguinte já obedece.
func TestSalvarTrocaAMatriz(t *testing.T) {
	antes := Policy.Roles
	t.Cleanup(func() { Salvar(antes) })

	Salvar(map[string]auth.Grants{"leitor": {"relatorios": "editar"}})
	if !Pode(&auth.User{Roles: []string{"leitor"}}, "relatorios", "editar") {
		t.Error("a matriz nova não valeu")
	}
	if Pode(&auth.User{Roles: []string{"admin"}}, "usuarios", "administrar") {
		t.Error("um papel que saiu da matriz continuou valendo")
	}
}

// Um papel novo nasce com o que o leitor tem; apagar só quando ninguém o tem.
func TestCriarEApagarPapel(t *testing.T) {
	antes := Policy.Roles
	t.Cleanup(func() { Salvar(antes) })

	if err := Criar("Gerente"); !errors.Is(err, ErrPapelInvalido) {
		t.Fatalf("maiúscula aceita: %v", err)
	}
	if err := Criar("gerente"); err != nil {
		t.Fatal(err)
	}
	if err := Criar("gerente"); !errors.Is(err, ErrPapelExiste) {
		t.Fatalf("repetido: %v", err)
	}
	if !Pode(&auth.User{Roles: []string{"gerente"}}, "relatorios", "ver") {
		t.Error("o papel novo não nasceu com o que o leitor tem")
	}
	if got := Papeis(); len(got) != 4 || got[2] != "gerente" {
		t.Fatalf("papeis = %v", got)
	}

	temGente := func(p string) bool { return p == "gerente" }
	if err := Apagar("gerente", temGente); !errors.Is(err, ErrPapelEmUso) {
		t.Fatalf("apagou um papel em uso: %v", err)
	}
	if err := Apagar("gerente", nil); err != nil {
		t.Fatal(err)
	}
	if Pode(&auth.User{Roles: []string{"gerente"}}, "relatorios", "ver") {
		t.Error("o papel apagado continuou valendo")
	}
}
`

const permsPage = `// Package permissoes is the screen that edits the matrix: a role per row, a
// module per column, a level in each cell — and, below it, the roles
// themselves (who has each, create, remove) and what the person looking at
// it may do.
//
// It is the most valuable screen in the application — whoever changes it
// changes what everybody else may do — which is why it sits behind the module
// that administers users, in middleware.go beside this file.
package permissoes

import (
	"errors"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/acesso"
	"{{.Module}}/internal/sessao"
	"{{.Module}}/internal/usuarios"
)

// Page draws the grid at GET {{.URL}}permissoes.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.perms_title}}")
	return ui.Stack(grade(c), papeis(c), oQueEuPosso(c)), nil
}

// POST is every button of the page: create a role, remove one, and — the
// default, what the grid posts — save the matrix.
func POST(c *trilha.Ctx) error {
	switch c.Form("_action") {
	case "create":
		nome := strings.TrimSpace(c.Form("nome"))
		err := acesso.Criar(nome)
		switch {
		case errors.Is(err, acesso.ErrPapelInvalido):
			return trilha.FieldErrors{"nome": "{{.T.perms_invalid}}"}
		case errors.Is(err, acesso.ErrPapelExiste):
			return trilha.FieldErrors{"nome": "{{.T.perms_exists}}"}
		case err != nil:
			return err
		}
		c.Audit("permissao.papel_criado", nome, nil)
		c.Flash(ui.FlashSuccess, "{{.T.perms_created}}")
	case "delete":
		nome := c.Form("papel")
		store := trilha.Use[*usuarios.Store](c)
		err := acesso.Apagar(nome, func(p string) bool { return len(comPapel(store, p)) > 0 })
		if errors.Is(err, acesso.ErrPapelEmUso) {
			c.Flash(ui.FlashError, "{{.T.perms_in_use}}")
			return c.Redirect("{{.URL}}permissoes")
		} else if err != nil {
			return err
		}
		c.Audit("permissao.papel_apagado", nome, nil)
		c.Flash(ui.FlashSuccess, "{{.T.perms_deleted}}")
	default:
		papeis, err := acesso.Bind(c)
		if err != nil {
			return err
		}
		acesso.Salvar(papeis)
		// One line, and the trail knows who did it, from where and on which
		// route. Changing who may do what is exactly the action somebody
		// asks about later.
		c.Audit("permissao.alterou", "matriz", trilha.Fields{"papeis": len(papeis)})
		c.Flash(ui.FlashSuccess, "{{.T.perms_saved}}")
	}
	return c.Redirect("{{.URL}}permissoes")
}

func grade(c *trilha.Ctx) h.Node {
	return ui.Card(
		ui.CardHeader(
			ui.CardTitle("{{.T.perms_title}}"),
			ui.CardDescription("{{.T.perms_desc}}"),
		),
		ui.CardContent(ui.PolicyGrid(acesso.Policy, ui.PolicyGridOpts{
			Action: "{{.URL}}permissoes",
			CSRF:   trilha.CSRFInput(c),
			Labels: acesso.Rotulos,
			None:   "{{.T.perms_none}}",
			Submit: "{{.T.perms_save}}",
		})),
	)
}

// papeis is one row per role: who has it, and the button to remove it when
// nobody does. The button is absent, not disabled, while somebody has the
// role — the rule that holds is acesso.Apagar, and a hidden button is only
// what the screen says about it.
func papeis(c *trilha.Ctx) h.Node {
	store := trilha.Use[*usuarios.Store](c)
	linhas := make([]h.Node, 0)
	for _, p := range acesso.Papeis() {
		gente := comPapel(store, p)
		quem := h.Text("{{.T.perms_nobody}}")
		var apagar h.Node
		if len(gente) > 0 {
			quem = h.Text(strings.Join(gente, ", "))
		} else {
			apagar = h.Form(h.Method("post"), h.Action("{{.URL}}permissoes"), h.Class("ui-inline-form"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("_action"), h.Value("delete")),
				h.Input(h.Type("hidden"), h.Name("papel"), h.Value(p)),
				ui.Button(ui.Outline(), ui.Sm(), h.Type("submit"), h.Text("{{.T.perms_delete}}"),
					ui.Confirm("{{.T.perms_delete_ask}}", "{{.T.perms_delete_desc}}")))
		}
		linhas = append(linhas, h.Tr(h.Td(ui.Code(p)), h.Td(quem), h.Td(apagar)))
	}
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.perms_roles}}")),
		ui.CardContent(ui.Stack(
			ui.Table(
				h.Thead(h.Tr(h.Th(h.Text("{{.T.perms_role}}")), h.Th(h.Text("{{.T.perms_who}}")), h.Th(h.Text("")))),
				h.Tbody(linhas...)),
			h.Form(h.Method("post"), h.Action("{{.URL}}permissoes"), h.Class("ui-stack"),
				trilha.CSRFInput(c),
				h.Input(h.Type("hidden"), h.Name("_action"), h.Value("create")),
				ui.Field("nome", "{{.T.perms_new_role}}", ui.Input(h.ID("nome"), h.Name("nome"), h.Required(), h.Placeholder("gerente"))),
				ui.Submit(h.Text("{{.T.perms_create}}")),
			),
		)),
	)
}

// comPapel is who has a role, by e-mail, from the login recipe's table.
func comPapel(store *usuarios.Store, papel string) []string {
	var out []string
	for _, u := range store.All() {
		if u.Papel == papel {
			out = append(out, u.Email)
		}
	}
	return out
}

// oQueEuPosso is the matrix from the other side: not what a role grants, but
// what the person looking at the screen has, module by module. It is the
// answer to "why can't I" before it becomes a ticket.
func oQueEuPosso(c *trilha.Ctx) h.Node {
	u := sessao.Atual(c)
	linhas := make([]h.Node, 0, len(acesso.Policy.Modules))
	for _, m := range acesso.Policy.Modules {
		rotulo := acesso.Rotulos[m]
		if rotulo == "" {
			rotulo = m
		}
		nivel := acesso.Policy.Level(u, m)
		if nivel == "" {
			nivel = "{{.T.perms_none}}"
		}
		linhas = append(linhas, h.Tr(h.Td(h.Text(rotulo)), h.Td(h.Text(nivel))))
	}
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.T.perms_mine}}"), ui.CardDescription("{{.T.perms_mine_desc}}")),
		ui.CardContent(ui.Table(
			h.Thead(h.Tr(h.Th(h.Text("{{.T.perms_module}}")), h.Th(h.Text("{{.T.perms_level}}")))),
			h.Tbody(linhas...))),
	)
}
`

const permsMiddleware = `package permissoes

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/acesso"
	"{{.Module}}/internal/sessao"
)

// exige is the guard the matrix builds; Middleware is what the scanner asks
// for — a function with a fixed signature, and a var of the right type is not
// one.
//
// The screen that edits the matrix is guarded by the matrix itself, and not by
// a role written here: a permissions screen behind an if on the role is a
// matrix with one exception living outside it.
var exige = sessao.Flow.RequirePolicy(acesso.Policy, "usuarios", "administrar")

// Middleware guards this folder. Somebody signed in without the level gets 403
// and not a redirect to the login: they are known, just not permitted.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const permsTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// The matrix decides, and the screen that edits it is decided by the matrix.
func TestMatrizDecide(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	c := trilha.NewTestClient(t, a)

	// Closed to whoever is not signed in.
	c.Get("{{.URL}}permissoes").WantStatus(http.StatusUnauthorized)

	c.PostForm(sessao.Flow.LoginPath(), url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}permissoes").WantStatus(http.StatusOK)

	// What the grid posts changes the matrix: the field names are
	// grant.<role>.<module>, which is what auth.BindPolicy reads.
	c.PostForm("{{.URL}}permissoes", url.Values{
		"grant.admin.usuarios":     {"administrar"},
		"grant.admin.relatorios":   {"administrar"},
		"grant.leitor.relatorios":  {"editar"},
	}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}permissoes").WantStatus(http.StatusOK).WantContains("leitor")

	// A role is created here and shows up in the grid; the one the
	// administrator has cannot be removed, and one nobody has can.
	c.PostForm("{{.URL}}permissoes", url.Values{"_action": {"create"}, "nome": {"gerente"}}).WantStatus(http.StatusSeeOther)
	c.PostForm("{{.URL}}permissoes", url.Values{"_action": {"create"}, "nome": {"Gerente!"}}).WantStatus(http.StatusUnprocessableEntity)
	c.Get("{{.URL}}permissoes").WantStatus(http.StatusOK).WantContains("grant.gerente.relatorios", "admin@example.com")
	c.PostForm("{{.URL}}permissoes", url.Values{"_action": {"delete"}, "papel": {"admin"}}).WantStatus(http.StatusSeeOther)
	c.Get("{{.URL}}permissoes").WantStatus(http.StatusOK).WantContains("grant.admin.usuarios")
	c.PostForm("{{.URL}}permissoes", url.Values{"_action": {"delete"}, "papel": {"gerente"}}).WantStatus(http.StatusSeeOther)
	if body := c.Get("{{.URL}}permissoes").WantStatus(http.StatusOK).Body.String(); strings.Contains(body, "grant.gerente.") {
		t.Fatal("the role stayed after being removed")
	}
}
`
