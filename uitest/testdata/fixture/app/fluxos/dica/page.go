// Package dica has one tooltip on a button.
package dica

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Tooltip")),
		ui.Tooltip("Copies the address", ui.Button(h.ID("copiar"), h.Type("button"), h.Text("Copy")))), nil
}
