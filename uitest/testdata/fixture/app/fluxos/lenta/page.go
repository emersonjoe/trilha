// Package lenta is a list swapped in place whose pages answer slowly, and a
// save that answers slowly too: two clicks in a row are what #294 is about.
package lenta

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// delays make page 2 the slow one: a click on 3 after a click on 2 is the
// newer intent arriving first.
var delays = map[string]time.Duration{"2": 900 * time.Millisecond, "3": 400 * time.Millisecond}

var saves atomic.Int64

func Page(c *trilha.Ctx) (h.Node, error) {
	p := c.Query("p")
	if p == "" {
		p = "1"
	}
	time.Sleep(delays[p])
	if c.Fragment() == "lista" {
		return lista(c, p), nil
	}
	c.SetTitle("Slow list")
	return h.Div(h.H1(h.Text("Slow list")), lista(c, p)), nil
}

func POST(c *trilha.Ctx) error {
	time.Sleep(500 * time.Millisecond)
	saves.Add(1)
	return c.Render(http.StatusOK, lista(c, "1"))
}

func lista(c *trilha.Ctx, p string) h.Node {
	return h.Div(h.ID("lista"),
		h.P(h.ID("pagina"), h.Text("page "+p)),
		h.P(h.ID("salvos"), h.Text(fmt.Sprintf("saves: %d", saves.Load()))),
		h.A(h.ID("p2"), h.Href("/fluxos/lenta?p=2"), ui.Swap("lista"), h.Text("2")),
		h.A(h.ID("p3"), h.Href("/fluxos/lenta?p=3"), ui.Swap("lista"), h.Text("3")),
		h.Form(h.ID("salvar"), h.Method("post"), h.Action("/fluxos/lenta"), ui.Swap("lista"),
			trilha.CSRFInput(c), h.Button(h.Type("submit"), h.Text("Save"))))
}
