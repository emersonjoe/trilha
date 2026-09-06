// Package apagar is a write that lives in a route.go, as the old app had it:
// one folder per action. It is a page route because app/legado-/kind.go says
// the whole branch is, not because this file says so.
package apagar

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ui"
)

// POST answers the delete button of /legado.
func POST(c *trilha.Ctx) error {
	c.Flash(ui.FlashSuccess, "Apagado")
	return c.Redirect("/legado")
}
