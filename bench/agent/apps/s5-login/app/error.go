package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Error renders every error status but 404, with the app's own layout. The
// status comes from the error, so a 403 reads like the app instead of like the
// framework.
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	c.SetTitle("Algo deu errado")
	return ui.Stack(
		ui.H1(h.Text("Algo deu errado")),
		h.If(c.Env() == trilha.Dev, ui.Alert("dev", ui.Destructive(), ui.Icon("triangle-alert"), ui.AlertDescription(h.Pre(h.Text(err.Error()))))),
		ui.Muted(h.Textf("Passe este código a quem olha os logs: %s", c.RequestID())),
	), nil
}
