package admin

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// documentIcon é o ícone que o conjunto embutido do kit não tem: nenhum dos 31
// nomes de ui.Icons() fala de documento. O app desenha o seu próprio h.Svg — a
// classe ui-icon é o que faz o kit continuar dono do tamanho e do alinhamento.
func documentIcon() h.Node {
	return h.Svg(h.Class("ui-icon"), h.Attr("viewBox", "0 0 24 24"), h.Attr("fill", "none"),
		h.Attr("stroke", "currentColor"), h.Attr("stroke-width", "2"),
		h.Attr("stroke-linecap", "round"), h.Attr("stroke-linejoin", "round"), h.Aria("hidden", "true"),
		h.El("path", h.Attr("d", "M6 2h9l5 5v13a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1Z")),
		h.El("path", h.Attr("d", "M15 2v5h5")),
	)
}

// Layout wraps /admin with the sidebar every management app writes in its
// first week: ui.Shell, with one entry using ui.Icon (a name the kit carries)
// and one using NavItem.IconNode (a document, which it does not).
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	cur := c.Request().URL.Path
	return ui.Shell(c, ui.ShellOpts{
		Nav: []ui.NavGroup{{Items: []ui.NavItem{
			{Href: "/admin", Label: "Painel", Icon: "house"},
			{Href: "/anexos", Label: "Anexos", IconNode: documentIcon()},
		}}},
		Current: cur,
	}, children), nil
}
