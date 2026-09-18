package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /: the trace this request belongs to, which is the same
// identifier the access log writes and the same one the collector shows.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Trilha + OpenTelemetry")
	id := c.TraceID()
	if id == "" {
		id = "(this request started its own trace)"
	}
	return h.Div(
		h.H1(h.Text("Trilha + OpenTelemetry")),
		h.P(h.Text("trace: "+id)),
		h.P(h.Text("request: "+c.RequestID())),
		h.P(h.A(h.Href("/traco"), h.Text("a hop to another service"))),
	), nil
}
