package slug

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /docs/{slug...}, ported from app/docs/[[...slug]]/page.tsx.
//
// Source: 4 lines, server component.
// Suggested: A — no island signal.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Docs")
	return ui.Container(
		ui.H1(h.Text("Docs")),
		h.P(h.Text(c.Param("slug"))),
	), nil
}
