package id

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /documents/{id}, ported from app/documents/[id]/page.tsx.
//
// Source: 24 lines, 'use client', 1 useEffect, 1 useRef, 2 useState.
// Calls: GET /api/documents/:id, POST /api/documents/:id/reprocess, GET /api/documents/:id/status.
// Suggested: B — polling.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Documents")
	return ui.Container(
		ui.H1(h.Text("Documents")),
		h.P(h.Text(c.Param("id"))),
	), nil
}
