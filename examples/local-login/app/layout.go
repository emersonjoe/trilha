package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
)

// Layout is the root shell: a header saying who is logged in, and the page.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	titulo := c.Title()
	if titulo == "" {
		titulo = "Login local · Trilha"
	}
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(titulo)),
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))),
		),
		h.Body(
			h.Header(h.Class("topo"),
				h.A(h.Href("/"), h.Class("marca"), h.Text("Login local")),
				quem(c),
			),
			h.Main(children),
		),
	), nil
}

func quem(c *trilha.Ctx) h.Node {
	u := sessao.Flow.User(c)
	if u == nil {
		return h.Nav(h.A(h.Href("/entrar"), h.Class("botao"), h.Text("Entrar")))
	}
	return h.Nav(
		h.Span(h.Class("quem"), h.Text(u.Name)),
		h.A(h.Href("/painel"), h.Text("Painel")),
		h.Form(h.Method("post"), h.Action("/sair"), trilha.CSRFInput(c),
			h.Button(h.Type("submit"), h.Class("botao"), h.Text("Sair"))),
	)
}
