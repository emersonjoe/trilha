package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /, ported from app/page.tsx.
//
// Source: 11 lines, server component.
// Suggested: A — no island signal.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Home")
	return ui.Container(
		ui.H1(h.Text("Home")),
	), nil
}
