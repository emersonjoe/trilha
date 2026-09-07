package path

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /files/{path...}, ported from app/files/[...path]/page.tsx.
//
// Source: 4 lines, server component.
// Suggested: A — no island signal.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Files")
	return ui.Container(
		ui.H1(h.Text("Files")),
		h.P(h.Text(c.Param("path"))),
	), nil
}
