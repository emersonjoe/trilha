package dashboard

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /dashboard, ported from app/dashboard/page.tsx.
//
// Source: 15 lines, 'use client', 1 useMemo, 1 useState.
// Calls: GET /api/metrics?range=:range.
// Suggested: C — pointer (line 10) and live svg (line 10).
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Dashboard")
	return ui.Container(
		ui.H1(h.Text("Dashboard")),
	), nil
}
