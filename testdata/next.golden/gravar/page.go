package gravar

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /gravar, ported from app/gravar/page.tsx.
//
// Source: 16 lines, 'use client', 1 useRef, 1 useState.
// Suggested: C — media capture (line 9).
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Gravar")
	return ui.Container(
		ui.H1(h.Text("Gravar")),
	), nil
}
