package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Error renders the 500 page. The cause is shown only in dev.
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	c.SetTitle("Erro")
	return ui.Stack(
		h.H1(h.Text("Algo deu errado")),
		h.If(c.Env() == trilha.Dev, ui.Alert("Detalhe (só em dev)", ui.Destructive(), ui.Icon("triangle-alert"), ui.AlertDescription(h.Pre(h.Text(err.Error()))))),
		ui.Muted(h.Textf("Cite o id %s ao reportar.", c.RequestID())),
	), nil
}
