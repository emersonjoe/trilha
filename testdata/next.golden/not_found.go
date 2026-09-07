package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Ported from app/not-found.tsx.
//
// Source: 4 lines, server component.
func NotFound(c *trilha.Ctx) (h.Node, error) {
	return ui.Container(ui.H1(h.Text("Not found"))), nil
}
