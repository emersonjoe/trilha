package path

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET /docs/{path...}: every segment is available in path.
func Page(c *trilha.Ctx) (h.Node, error) {
	parts := strings.Split(c.Param("path"), "/")
	c.SetTitle("Docs: " + parts[len(parts)-1])
	return h.Fragment(
		h.H1(h.Text("Docs")),
		h.P(h.Text("Segmentos capturados por path__:")),
		h.Ol(h.Map(parts, func(s string) h.Node { return h.Li(h.Code(h.Text(s))) })),
	), nil
}
