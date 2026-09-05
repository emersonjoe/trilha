package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Error renders the 500 page. The cause is shown only in dev.
func Error(c *trilha.Ctx, err error) (h.Node, error) {
	c.SetTitle("Erro")
	return h.Fragment(
		h.H1(h.Text("Algo deu errado")),
		h.If(c.Env() == trilha.Dev, h.Pre(h.Text(err.Error()))),
		h.P(h.Textf("Cite o id %s ao reportar.", c.RequestID())),
	), nil
}
