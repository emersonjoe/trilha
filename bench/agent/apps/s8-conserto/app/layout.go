package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout is the root layout: the <html> document and the frame around every
// page.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	title := c.Title()
	if title == "" {
		title = "App"
	}
	return h.Html(h.Lang(c.Locale()),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text(title)),
			ui.Head(c), // public/ui.theme.css (your theme), ui.css and ui.js
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))),
		),
		h.Body(ui.Body(), h.Main(ui.Container(children)), ui.Flashes(c)),
	), nil
}
