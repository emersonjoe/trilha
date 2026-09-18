// Package idiomas is the screen of an office that serves people who do not
// read Portuguese: the same page, in the language of whoever is asking.
//
// Nothing here chooses the language. c.Locale negotiated it — the preference
// in the session, the ?lang of the links below, the cookie that ?lang left,
// the browser's Accept-Language — and c.T says the sentence in it, falling
// back through the chain the catalog was given (ht → fr → pt-BR) instead of
// showing a raw key.
package idiomas

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /idiomas in the language of the request.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle(c.T("atendimento.titulo"))
	return h.Div(
		ui.PageHeader(c.T("atendimento.titulo")),
		h.P(h.Text(c.T("atendimento.bemvindo", "Jean"))),
		h.P(h.ID("protocolos"), h.Text(c.T("atendimento.protocolos", 3))),
		ui.Muted(h.Text(c.T("atendimento.escolha"))),
		h.P(
			h.A(h.Href("/idiomas?lang=pt-BR"), h.Text("Português")), h.Text(" · "),
			h.A(h.Href("/idiomas?lang=ht"), h.Text("Kreyòl")), h.Text(" · "),
			h.A(h.Href("/idiomas?lang=fr"), h.Text("Français")),
		),
		ui.Muted(h.Text("Locale: "+c.Locale())),
	), nil
}
