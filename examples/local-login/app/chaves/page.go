// Package chaves is the screen that issues, lists and revokes the API keys.
package chaves

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// nova is the issue form. The scopes are checked against the same list the
// routes use, so a key can never carry something no route understands.
type nova struct {
	Nome    string   `form:"nome"    validate:"required,max=60"`
	Escopos []string `form:"escopos" validate:"minitems=1"`
}

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Chaves de API")
	return tela(c, nil, ""), nil
}

// POST issues a key and shows the secret — once. Nothing here can show it
// again, and the card says so.
func POST(c *trilha.Ctx) error {
	if id := c.Request().FormValue("id"); id != "" {
		if err := sessao.Chaves.Revoke(c, id); err != nil {
			return err
		}
		return c.Redirect("/chaves")
	}
	var f nova
	if err := c.Bind(&f); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, tela(c, errs, ""))
	}
	_, secret, err := sessao.Chaves.Issue(c, f.Nome, f.Escopos, 90*24*time.Hour)
	if err != nil {
		return err
	}
	return c.Render(http.StatusOK, tela(c, nil, secret))
}

func tela(c *trilha.Ctx, errs trilha.FieldErrors, novoSegredo string) h.Node {
	todas, _ := sessao.Chaves.All()
	linhas := make([]ui.APIKeyRow, 0, len(todas))
	for _, k := range todas {
		linhas = append(linhas, ui.APIKeyRow{
			ID: k.ID, Handle: k.Handle, Name: k.Name, Scopes: k.Scopes,
			Created: k.Created, LastUsed: k.LastUsed, Revoked: !k.RevokedAt.IsZero(),
		})
	}
	return h.Div(
		ui.H1(h.Text("Chaves de API")),
		h.If(novoSegredo != "", ui.SecretOnce(c, novoSegredo)),
		ui.Card(ui.CardHeader(ui.CardTitle("Nova chave")), ui.CardContent(
			h.Form(h.Method("post"), h.Class("ui-stack"), trilha.CSRFInput(c),
				ui.Field("nome", "Nome", ui.Input(h.ID("nome"), h.Name("nome"), ui.InvalidIf(errs, "nome")), ui.Errors(errs, "nome")),
				h.Fieldset(h.Class("ui-stack"),
					h.Legend(h.Class("ui-label"), h.Text("Pode")),
					h.Map(sessao.Escopos, func(e string) h.Node {
						return ui.CheckRow(ui.Checkbox(h.ID("e-"+e), h.Name("escopos"), h.Value(e)), e, "e-"+e)
					}),
					h.If(errs.Has("escopos"), h.P(h.Class("ui-field-error"), h.Text(errs.Get("escopos")))),
				),
				ui.Submit(h.Text("Emitir"))))),
		ui.APIKeysTable(c, linhas, ui.APIKeysOpts{Revoke: "/chaves", CSRF: trilha.CSRFInput(c)}),
	)
}
