package recipes

// approvalsRecipe is the queue of things waiting for a person, wired into a
// project: the package, the screen, and the handler where a decision becomes
// whatever the decision means.
func approvalsRecipe() Recipe {
	return Recipe{
		Name: "approvals",
		Summary: map[string]string{
			"en": "the queue that waits for a person: open, decide, and the deadline that expires",
			"pt": "a fila que espera uma pessoa: abrir, decidir, e o prazo que vence",
		},
		Doc:   "/reference/approval",
		Needs: []Need{{Recipe: "login", File: "internal/sessao/sessao.go"}},
		Files: []File{
			{Rel: "internal/aprovacoes/aprovacoes.go", Go: true, Body: approvalsDecl},
			{Rel: "internal/aprovacoes/aprovacoes_test.go", Go: true, Body: approvalsDeclTest},
			{Rel: "{{.At}}aprovacoes/page.go", Go: true, Body: approvalsPage},
			{Rel: "{{.At}}aprovacoes/middleware.go", Go: true, Body: approvalsMiddleware},
			{Rel: "aprovacoes_test.go", Go: true, Body: approvalsTest},
		},
		Setup: []Insert{{
			Marker: "// trilha:add approvals",
			Line:   "\tif err := aprovacoes.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/internal/aprovacoes"},
		Next: map[string]string{
			"en": "Run `trilha dev` and open {{.URL}}aprovacoes. Open a request where the thing happens — " +
				"`aprovacoes.Fila.Open(c, approval.Request{…})` — and write what the decision means in the " +
				"On handler of internal/aprovacoes: that is the only place this package leaves for you.",
			"pt": "Rode `trilha dev` e abra {{.URL}}aprovacoes. Abra um pedido onde a coisa acontece — " +
				"`aprovacoes.Fila.Open(c, approval.Request{…})` — e escreva o que a decisão significa no " +
				"gancho On do internal/aprovacoes: é o único lugar que este pacote deixa para você.",
		},
	}
}

const approvalsDecl = `// Package aprovacoes is the queue of things somebody has to decide.
//
// The package answers the four things such a queue always needs — an owner, a
// deadline, the reason, and who decided — and leaves one thing to this file:
// what a decision means for this application.
package aprovacoes

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"

	"{{.Module}}/internal/sessao"
)

// Tipo is what this application asks people to decide. It is a constant for
// the same reason a task name is: the screen that opens the request and the
// handler that answers it have to agree.
const Tipo = "exemplo"

// Setup builds the queue, says what a decision means and starts the deadline
// clock.
//
// The queue is a value the app provides, and not a package variable: a suite
// that stands up one server per test gives each one its own, and registering
// the same handler twice on a shared queue is a panic on the second test.
//
// Roles is how a request assigned to a role finds its people: the package does
// not know how this application authenticates, and a check it guessed would
// look like a guarantee without being one.
func Setup(a *trilha.App) error {
	fila := approval.New(approval.Options{
		Logger: a.Logger(),
		Roles: func(c *trilha.Ctx) []string {
			if u := sessao.Atual(c); u != nil {
				return u.Roles
			}
			return nil
		},
	})
	fila.On(Tipo, decidido)
	trilha.Provide(a, fila)
	return fila.Setup(a)
}

// decidido runs after the decision is written. Write here what the decision
// actually does — delete the thing, send the e-mail, emit the webhook.
//
// An error here does not undo the decision: a person chose, and it is
// recorded. Making the work survive a failure is this function's job, and the
// task recipe is where that lives.
func decidido(c *trilha.Ctx, r approval.Record) error {
	c.Log().Info("aprovacoes: decidido", "id", r.ID, "estado", r.State, "por", r.By)
	return nil
}
`

const approvalsDeclTest = `package aprovacoes

import (
	"testing"

	"github.com/emersonjoe/trilha/approval"
)

// O tipo é uma constante porque a tela e o gancho precisam concordar: um erro
// de digitação aqui seria um pedido que ninguém decide.
func TestTipoTemNome(t *testing.T) {
	if Tipo == "" {
		t.Fatal("o tipo do pedido não tem nome")
	}
	// E o estado inicial de um pedido é pendente, que é o que a tela filtra.
	if approval.Pending != "pending" {
		t.Fatalf("estado inicial = %q", approval.Pending)
	}
}
`

const approvalsPage = `// Package aprovacoes is the inbox: what is waiting for whoever is reading, and
// the two buttons.
//
// Who may decide is the package's answer and not this screen's — a hidden
// button is a screen, and the address behind it is still an address.
package aprovacoes

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET {{.URL}}aprovacoes.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.T.inbox_title}}")
	fila := trilha.Use[*approval.Approvals](c)
	pendentes, err := fila.Inbox(c, approval.ListParams{})
	if err != nil {
		return nil, err
	}
	decididas, err := fila.List(c.Context(), approval.ListParams{Limit: 20})
	if err != nil {
		return nil, err
	}
	return ui.Stack(
		ui.PageHeader("{{.T.inbox_title}}"),
		ui.Muted(h.Text("{{.T.inbox_desc}}")),
		ui.Inbox(c, linhas(c, pendentes), ui.InboxOpts{
			Decide: "{{.URL}}aprovacoes",
			CSRF:   trilha.CSRFInput(c),
		}),
		ui.H2(h.Text("{{.T.inbox_recent}}")),
		ui.Inbox(c, linhas(c, decididas), ui.InboxOpts{}),
	), nil
}

// POST decides. The decision and the reason travel in the same form, because a
// reason typed into a field a second click discards is a reason nobody wrote.
func POST(c *trilha.Ctx) error {
	if err := trilha.Use[*approval.Approvals](c).Decide(c, c.Form("id"), c.Form("decision"), c.Form("reason")); err != nil {
		if err == approval.ErrNotYours {
			return trilha.Errorf(http.StatusForbidden, "%s", "{{.T.inbox_not_yours}}")
		}
		return err
	}
	c.Flash(ui.FlashSuccess, "{{.T.inbox_done}}")
	return c.Redirect("{{.URL}}aprovacoes")
}

// linhas maps what the package keeps to what the screen shows. Late is decided
// here because only the request knows what now means.
func linhas(c *trilha.Ctx, lista []approval.Record) []ui.InboxRow {
	agora := time.Now()
	out := make([]ui.InboxRow, 0, len(lista))
	for _, r := range lista {
		out = append(out, ui.InboxRow{
			ID: r.ID, Kind: r.Kind, Subject: r.Subject, Target: r.Target,
			State: r.State, Due: r.Due, Late: r.Late(agora), By: r.By, Reason: r.Reason,
		})
	}
	return out
}
`

const approvalsMiddleware = `package aprovacoes

import (
	"github.com/emersonjoe/trilha"

	"{{.Module}}/internal/sessao"
)

// exige is the rule, and Middleware is what the scanner reads.
var exige = sessao.Flow.Require()

// Middleware asks for a session: an inbox is personal, and what it shows
// depends on who is asking.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
`

const approvalsTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"

	"{{.Module}}/internal/aprovacoes"
)

// A caixa mostra o que espera por quem olha, e decidir grava quem decidiu.
func TestCaixaDeAprovacoes(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "a-password-nobody-guesses")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()
	c := trilha.NewTestClient(t, a)

	c.PostForm("{{.URL}}entrar", url.Values{
		"email":    {"admin@example.com"},
		"password": {"a-password-nobody-guesses"},
	}).WantStatus(http.StatusSeeOther)

	// Um pedido é aberto de dentro de uma requisição, que é onde o ator está.
	var id string
	a.Register(trilha.Route{Pattern: "/_teste/abrir", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"POST": func(c *trilha.Ctx) error {
				var err error
				id, err = trilha.Use[*approval.Approvals](c).Open(c, approval.Request{
					Kind: aprovacoes.Tipo, Subject: "Pedido de exemplo",
					Assign: approval.Role("admin"),
				})
				if err != nil {
					return err
				}
				return c.Text(http.StatusOK, id)
			},
		}})
	c.PostForm("/_teste/abrir", url.Values{}).WantStatus(http.StatusOK)

	corpo := c.Get("{{.URL}}aprovacoes").WantStatus(http.StatusOK).Body.String()
	if !strings.Contains(corpo, "Pedido de exemplo") {
		t.Fatalf("o pedido não apareceu na caixa:\n%s", corpo)
	}

	c.PostForm("{{.URL}}aprovacoes", url.Values{
		"id":       {id},
		"decision": {approval.Approved},
		"reason":   {"combinado"},
	}).WantStatus(http.StatusSeeOther)

	// E o registro guarda a decisão: quem, quando e por quê.
	var rec approval.Record
	a.Register(trilha.Route{Pattern: "/_teste/ler", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				var err error
				if rec, err = trilha.Use[*approval.Approvals](c).Get(c.Context(), id); err != nil {
					return err
				}
				return c.Text(http.StatusOK, rec.State)
			},
		}})
	c.Get("/_teste/ler").WantStatus(http.StatusOK)
	if rec.State != approval.Approved || rec.Reason != "combinado" || rec.By == "" {
		t.Fatalf("decisão = %+v", rec)
	}
}
`
