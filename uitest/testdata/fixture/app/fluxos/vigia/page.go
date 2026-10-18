// Package vigia has two polled fragments: one whose answer never changes and
// one that counts the times it was asked. The first must stay the same
// element on the page; the second must keep changing (spec 177).
package vigia

import (
	"sync/atomic"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

var asks atomic.Int64

func fixo() h.Node {
	return h.Div(h.ID("fixo"), ui.Poll("200ms", ""), h.P(h.Text("always the same")))
}

func conta() h.Node {
	return h.Div(h.ID("conta"), ui.Poll("200ms", ""), h.P(h.Textf("ask %d", asks.Add(1))))
}

func Page(c *trilha.Ctx) (h.Node, error) {
	switch c.Fragment() {
	case "fixo":
		return fixo(), nil
	case "conta":
		return conta(), nil
	}
	return h.Div(h.H1(h.Text("Watch")), ui.LiveScript(c), fixo(), conta()), nil
}
