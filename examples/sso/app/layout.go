package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
	"github.com/emersonjoe/trilha/h"
)

// Layout é o layout raiz: cabeçalho com o estado da sessão e o conteúdo.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	title := c.Title()
	if title == "" {
		title = "SSO · Trilha"
	}
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			h.Link(h.Rel("stylesheet"), h.Href("/style.css")),
		),
		h.Body(
			h.Header(h.Class("topo"),
				h.A(h.Href("/"), h.Class("marca"), h.Text("Trilha SSO")),
				sessao(c),
			),
			h.Main(children),
		),
	), nil
}

// sessao mostra quem está logado e o botão de sair (POST, com CSRF).
func sessao(c *trilha.Ctx) h.Node {
	u := sso.User(c)
	if u == nil {
		return h.Nav(h.A(h.Href("/entrar"), h.Class("botao"), h.Text("Entrar")))
	}
	return h.Nav(
		h.Span(h.Class("quem"), h.Text(nome(u.Name, u.Email, u.Subject))),
		h.A(h.Href("/painel"), h.Text("Painel")),
		h.Form(h.Method("post"), h.Action("/sair"), trilha.CSRFInput(c),
			h.Button(h.Type("submit"), h.Class("botao"), h.Text("Sair")),
		),
	)
}

func nome(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "anônimo"
}
