package about

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /about, ported from app/(marketing)/about/page.tsx.
//
// Source: 11 lines, server component.
// Suggested: A — no island signal of its own.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("About")
	return ui.Container(
		ui.H1(h.Text("About")),
	), nil
}
