package admin

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /admin (only reachable through the middleware).
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Admin")
	user, _ := c.Get("user").(string)
	return ui.Stack(
		ui.Row(ui.Avatar("AD", ""), h.H1(h.Class("ui-h1"), h.Textf("Olá, %s", user))),
		h.P(h.Textf("%d posts publicados.", len(posts.All()))),
		h.Form(h.Method("post"), h.Action("/login"), trilha.CSRFInput(c),
			h.Input(h.Type("hidden"), h.Name("sair"), h.Value("1")),
			ui.Submit(ui.Outline(), h.Text("Sair"))),
	), nil
}
