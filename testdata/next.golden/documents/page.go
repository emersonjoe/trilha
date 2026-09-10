package documents

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /documents, ported from app/documents/page.tsx.
//
// Source: 15 lines, 'use client', 1 useEffect, 2 useState.
// Calls: GET /api/documents?q=:query.
// Suggested: A — no island signal.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Documents")
	return ui.Container(
		ui.H1(h.Text("Documents")),
	), nil
}
