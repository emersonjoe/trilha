package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Início")
	item := func(href, label, desc string) h.Node {
		return ui.Card(ui.CardHeader(ui.CardTitle(label), ui.CardDescription(desc)),
			ui.CardFooter(ui.ButtonLink(href, ui.Outline(), ui.Sm(), h.Text(href), ui.Icon("arrow-right"))))
	}
	return ui.Stack(
		ui.H1(h.Text("Trilha")),
		ui.Lead(h.Text("Um framework web para Go com roteamento por arquivos: páginas, layouts, rotas de API e middleware descobertos a partir da pasta app/.")),
		ui.Grid(
			item("/blog", "Lista de posts", "página com layout aninhado"),
			item("/blog/ola-trilha", "Um post", "segmento dinâmico slug_"),
			item("/docs/guia/rotas", "Docs", "catch-all path__"),
			item("/api/posts", "API", "rota de API (route.go)"),
			item("/admin", "Admin", "protegido por middleware.go"),
			item("/precos", "Preços", "grupo de rota marketing- (layout sem segmento na URL)"),
			item("/relatorio", "Relatório", "página com html/template via tmpl"),
		),
	), nil
}
