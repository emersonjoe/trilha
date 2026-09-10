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
			"en": "the permission matrix as data, and the screen that edits it",
			"pt": "a matriz de permissões como dado, e a tela que a edita",
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
	"sync"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/auth"
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
`

const permsPolicyTest = `package acesso

import (
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
`

const permsPage = `// Package permissoes is the screen that edits the matrix: a role per row, a
// module per column, a level in each cell.
//
// It is the most valuable screen in the application — whoever changes it
// changes what everybody else may do — which is why it sits behind the module
// that administers users, in middleware.go beside this file.
package permissoes

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"{{.Module}}/internal/acesso"
)

// Page draws the grid at GET {{.URL}}permissoes.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.perms_title}}")
	return tela(c), nil
}

// POST saves what the grid posted and comes back to it.
func POST(c *trilha.Ctx) error {
	papeis, err := acesso.Bind(c)
	if err != nil {
		return err
	}
	acesso.Salvar(papeis)
	// One line, and the trail knows who did it, from where and on which route.
	// Changing who may do what is exactly the action somebody asks about later.
	c.Audit("permissao.alterou", "matriz", trilha.Fields{"papeis": len(papeis)})
	c.Flash(ui.FlashSuccess, "{{.T.perms_saved}}")
	return c.Redirect("{{.URL}}permissoes")
}

func tela(c *trilha.Ctx) h.Node {
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
	"testing"

	"github.com/emersonjoe/trilha"
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

	c.PostForm("{{.URL}}entrar", url.Values{
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
}
`
