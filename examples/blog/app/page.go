package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Início")
	return h.Fragment(
		h.H1(h.Text("Trilha")),
		h.P(h.Text("Um framework web para Go no estilo Next.js: páginas, layouts, rotas de API e middleware descobertos a partir da pasta app/.")),
		h.Ul(
			h.Li(h.A(h.Href("/blog"), h.Text("Lista de posts")), h.Text(" — página com layout aninhado")),
			h.Li(h.A(h.Href("/blog/ola-trilha"), h.Text("/blog/ola-trilha")), h.Text(" — segmento dinâmico slug_")),
			h.Li(h.A(h.Href("/docs/guia/rotas"), h.Text("/docs/guia/rotas")), h.Text(" — catch-all path__")),
			h.Li(h.A(h.Href("/api/posts"), h.Text("/api/posts")), h.Text(" — rota de API (route.go)")),
			h.Li(h.A(h.Href("/admin"), h.Text("/admin")), h.Text(" — protegido por middleware.go")),
			h.Li(h.A(h.Href("/precos"), h.Text("/precos")), h.Text(" — grupo de rota marketing- (layout sem segmento na URL)")),
			h.Li(h.A(h.Href("/relatorio"), h.Text("/relatorio")), h.Text(" — página com html/template via tmpl")),
		),
	), nil
}
