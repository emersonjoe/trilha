// Package conversa is a ui.Chat whose answer streams many small tokens, the
// shape of a long answer from a model (spec 177).
package conversa

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("Chat")),
		ui.Chat(c, ui.ChatOpts{Action: "/fluxos/conversa/fala"}),
		ui.ChatScript(c)), nil
}
