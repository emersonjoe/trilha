package estudo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /estudo, ported from app/estudo/page.tsx.
//
// Source: 15 lines, server component.
// Server actions: registrarResposta (lib/actions.ts:3), criarBaralho (lib/actions.ts:7).
// Suggested: A — no island signal.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Estudo")
	return ui.Container(
		ui.H1(h.Text("Estudo")),
	), nil
}
