package painel

import (
	"strconv"
	"sync/atomic"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// meta is the month's goal, the one thing this page writes.
var meta atomic.Int64

func init() { meta.Store(10) }

// Page renders GET /painel.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Painel")
	st := trilha.Use[*posts.Store](c)
	n := len(st.All())
	return ui.Stack(
		h.H1(h.Class("ui-h1"), h.Text("Painel")),
		ui.Grid(
			ui.Card(ui.CardHeader(ui.CardDescription("Posts")), ui.CardContent(h.Span(h.Class("ui-h2"), h.Textf("%d", n)), h.P(h.Textf("%d posts.", n)))),
			ui.Card(ui.CardHeader(ui.CardDescription("Meta do mês")), ui.CardContent(
				ui.Progress(n, int(meta.Load())), ui.Muted(h.Textf("%d de %d", n, meta.Load())),
				h.Form(h.Method("post"), h.Action("/painel"), h.Class("ui-stack"), trilha.CSRFInput(c),
					ui.Field("meta", "Nova meta", ui.Input(h.ID("meta"), h.Name("meta"), h.Type("number"))),
					ui.Submit(h.Text("Salvar meta")),
				),
			)),
		),
		ui.Tabs("painel-tabs",
			ui.Tab{Label: "Recentes", Content: h.Ul(h.Map(st.All(), func(p posts.Post) h.Node { return h.Li(h.A(h.Href("/blog/"+p.Slug), h.Text(p.Title))) }))},
			ui.Tab{Label: "Ajuda", Content: h.P(h.Text("Abas, cards e barra de progresso vêm do kit ui."))},
		),
	), nil
}

// POST changes the month's goal. Who may do it is decided in middleware.go, by
// MiddlewarePOST; this handler only handles the goal.
func POST(c *trilha.Ctx) error {
	n, err := strconv.Atoi(c.Form("meta"))
	if err != nil || n < 1 {
		return trilha.Errorf(422, "a meta é um número maior que zero")
	}
	meta.Store(int64(n))
	return c.Redirect("/painel")
}
