package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Ported from app/error.tsx.
//
// Source: 6 lines, 'use client'.
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	return ui.Container(ui.H1(h.Text("Something went wrong"))), nil
}
