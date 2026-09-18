// Package campo is a screen filled where the network comes and goes: the
// service worker keeps a copy of it and the form waits in the outbox.
package campo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return h.Div(), nil }

func POST(c *trilha.Ctx) error { return c.Redirect("/campo") }
