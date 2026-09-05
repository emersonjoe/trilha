package login

import (
	"net/http"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the login form.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Entrar")
	return h.Div(h.Class("login"), ui.Card(
		ui.CardHeader(h.H1(h.Class("ui-card-title"), h.Text("Entrar")), ui.CardDescription("Usuário admin, senha trilha.")),
		ui.CardContent(h.Form(h.Method("post"), h.Action("/login"), h.Class("ui-stack"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("next"), h.Value(c.Query("next"))),
			ui.Field("u", "Usuário", ui.Input(h.ID("u"), h.Name("usuario"), h.Attr("autocomplete", "username"))),
			ui.Field("p", "Senha", ui.Input(h.ID("p"), h.Name("senha"), h.Type("password"), h.Attr("autocomplete", "current-password"))),
			ui.Submit(h.Text("Entrar")),
		)),
	)), nil
}

// POST authenticates (or signs out with sair=1) and redirects.
func POST(c *trilha.Ctx) error {
	if c.Form("sair") == "1" {
		c.ClearCookie("sessao")
		return c.Redirect("/")
	}
	if c.Form("usuario") != "admin" || c.Form("senha") != "trilha" {
		return trilha.Errorf(http.StatusUnauthorized, "usuário ou senha inválidos")
	}
	if err := c.SetSigned("sessao", "admin", 8*time.Hour); err != nil {
		return err // sem TRILHA_SECRET em produção
	}
	next := c.Form("next")
	if next == "" || next[0] != '/' {
		next = "/admin"
	}
	return c.Redirect(next)
}
