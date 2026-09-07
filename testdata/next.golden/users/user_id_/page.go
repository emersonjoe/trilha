package user_id

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders GET /users/{user_id}, ported from app/users/[user-id]/page.tsx.
//
// Source: 6 lines, server component.
// Calls: GET /api/users/:user_id.
// Suggested: A — no island signal.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Users")
	return ui.Container(
		ui.H1(h.Text("Users")),
		h.P(h.Text(c.Param("user_id"))),
	), nil
}
