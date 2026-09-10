package recipes

// shareLinkRecipe is the link that gives access without giving an account.
//
// "Send the client a link so they can see it" is a monthly request, and the
// hand-written answer is always the same: a sequential id in the URL, no
// deadline, no use limit, still working two years later — and guessable by
// adding one.
func shareLinkRecipe() Recipe {
	return Recipe{
		Name: "share-link",
		Summary: map[string]string{
			"en": "a signed link with a deadline: access to one thing, without an account",
			"pt": "um link assinado com prazo: acesso a uma coisa, sem conta",
		},
		Doc: "/reference/links",
		Files: []File{
			// A folder of its own, outside anything the recipe was pointed at:
			// whoever opens the link has no session, and a middleware guards
			// its folder and everything under it.
			{Rel: "app/compartilhado/token_/page.go", Go: true, Body: shareLinkPage},
			{Rel: "app/compartilhado/token_/kind.go", Go: true, Body: shareLinkKind},
			{Rel: "compartilhado_test.go", Go: true, Body: shareLinkTest},
		},
		Next: map[string]string{
			"en": "Create one where the thing is: `c.Link(\"compartilhado\", trilha.LinkOpts{Data: " +
				"map[string]string{\"id\": id}, TTL: 24 * time.Hour, Path: \"/compartilhado\"})`. Do not put " +
				"that route behind a login — whoever receives the link is precisely whoever has no account, " +
				"and `trilha audit` complains about a Claim behind Require.",
			"pt": "Crie um link onde a coisa está: `c.Link(\"compartilhado\", trilha.LinkOpts{Data: " +
				"map[string]string{\"id\": id}, TTL: 24 * time.Hour, Path: \"/compartilhado\"})`. Não ponha " +
				"essa rota atrás de login — quem recebe o link é justamente quem não tem conta, e o " +
				"`trilha audit` reclama de um Claim atrás de Require.",
		},
	}
}

const shareLinkPage = `// Package token_ is the public side of a share link: the page somebody opens
// from a message, with no session.
//
// It is a folder of its own, and that is not tidiness: a middleware guards its
// folder and everything under it, so this page under a screen that requires a
// session would demand the session whoever received the link does not have.
// The link is the authorization here.
//
// c.Claim checks the signature, the purpose and the deadline, and answers the
// same thing for every way it can fail — telling a stranger whether a token
// expired or never existed tells them how close they are.
package token_

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Nome is the purpose of the link. A link minted for one purpose does not open
// a route that claims another: the name travels signed inside the token.
const Nome = "compartilhado"

// Page renders GET /compartilhado/{token}.
func Page(c *trilha.Ctx) (h.Node, error) {
	link, err := c.Claim(Nome)
	if err != nil {
		return nil, err // 404: invalid, expired or spent — never which
	}
	// What travels in the token is an id, never a name, a price or a reason:
	// the token is signed, not secret, and whoever holds the link can read it.
	id := link.Data["id"]

	// Replace this with the thing itself. What matters is above: the route is
	// public, the claim is checked, and what came out of it is an id you look
	// up in your own table.
	c.SetTitle("{{.T.share_title}}")
	return ui.Stack(
		ui.PageHeader("{{.T.share_title}}"),
		ui.Muted(h.Text("{{.T.share_desc}}")),
		ui.Card(ui.CardContent(h.P(h.Textf("{{.T.share_item}}", id)))),
	), nil
}
`

const shareLinkKind = `package token_

import "github.com/emersonjoe/trilha"

// Kind says this branch answers pages: an error here is the application's
// error page, and not problem+json, because whoever opens a share link is a
// person with a browser.
var Kind = trilha.KindPage
`

const shareLinkTest = `package main

import (
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// O link abre; um token mexido não abre; e um link vencido não abre. As três
// coisas são a mesma resposta para quem está de fora.
func TestLinkCompartilhado(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	a := newApp()

	// O link é criado de dentro de uma requisição, que é onde o segredo do app
	// está. Uma rota de teste serve para isso e some com ela.
	var link string
	a.Register(trilha.Route{Pattern: "/_teste/criar", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				u, err := c.Link("compartilhado", trilha.LinkOpts{
					Data: map[string]string{"id": "doc-1"},
					TTL:  time.Hour,
					Path: "/compartilhado",
				})
				if err != nil {
					return err
				}
				return c.Text(200, u)
			},
		}})

	c := trilha.NewTestClient(t, a)
	link = c.Get("/_teste/criar").WantStatus(http.StatusOK).Body.String()

	// Sem sessão nenhuma: é para isso que o link existe.
	c.Get(caminho(link)).WantStatus(http.StatusOK).WantContains("doc-1")

	// Um token mexido não abre, e não diz por quê.
	c.Get(caminho(link) + "x").WantStatus(http.StatusNotFound)
}

// caminho tira o esquema e o host do que o Link devolveu.
func caminho(u string) string {
	for i := 0; i+3 <= len(u); i++ {
		if u[i:i+3] == "://" {
			resto := u[i+3:]
			for j := 0; j < len(resto); j++ {
				if resto[j] == '/' {
					return resto[j:]
				}
			}
			return "/"
		}
	}
	return u
}
`
