// Package vigia is a page with two polled fragments: one that comes back the
// same at every tick, with an island and a form inside, and one that changes
// at every tick. Only the second may be replaced (#306), and its island has to
// be mounted again when it is.
package vigia

import (
	"strconv"
	"sync/atomic"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

var ticks atomic.Int64

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Polled page")
	parado := h.Section(h.ID("parado"), ui.Poll("250ms", ""),
		h.P(h.Text("Status: ready")),
		c.Island("/ilha-contador.js", map[string]int{"start": 7}, h.P(h.Text("fallback"))),
		h.Form(h.Method("post"), trilha.CSRFInput(c)))
	mudando := func() h.Node {
		return h.Section(h.ID("mudando"), ui.Poll("250ms", ""),
			h.P(h.Text("Tick "+strconv.FormatInt(ticks.Add(1), 10))),
			c.Island("/ilha-contador.js", map[string]int{"start": 100}, h.P(h.Text("fallback"))))
	}
	switch c.Fragment() {
	case "parado":
		return parado, nil
	case "mudando":
		return mudando(), nil
	}
	return h.Div(ui.LiveScript(c), parado, mudando()), nil
}
