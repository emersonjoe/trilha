package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /painel.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Painel")
	n := len(posts.All())
	return ui.Stack(
		h.H1(h.Class("ui-h1"), h.Text("Painel")),
		ui.Grid(
			ui.Card(ui.CardHeader(ui.CardDescription("Posts")), ui.CardContent(h.Span(h.Class("ui-h2"), h.Textf("%d", n)), h.P(h.Textf("%d posts.", n)))),
			ui.Card(ui.CardHeader(ui.CardDescription("Meta do mês")), ui.CardContent(ui.Progress(n, 10), ui.Muted(h.Textf("%d de 10", n)))),
		),
		ui.Tabs("painel-tabs",
			ui.Tab{Label: "Recentes", Content: h.Ul(h.Map(posts.All(), func(p posts.Post) h.Node { return h.Li(h.A(h.Href("/blog/"+p.Slug), h.Text(p.Title))) }))},
			ui.Tab{Label: "Ajuda", Content: h.P(h.Text("Abas, cards e barra de progresso vêm do kit ui."))},
		),
	), nil
}
