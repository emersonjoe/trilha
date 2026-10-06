// Package contagem answers what the server counted for an address.
package contagem

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"

	"example.com/uitest/internal/conta"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.P(h.ID("n"), h.Text(conta.Of(c.Query("u")))), nil
}
