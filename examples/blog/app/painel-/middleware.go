package painel

import (
	"net/http"

	"github.com/emersonjoe/trilha"
)

// Middleware runs for every route in the group, after the root middleware.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	c.Set("area", "painel")
	c.Header("X-Area", "painel")
	return next()
}

// MiddlewarePOST guards only the POST of the group: reading the panel is open
// to whoever got here, changing the month's goal needs the session /login sets.
// The rule lives beside the other middlewares instead of in the first line of
// the handler, which is where the eleventh route forgets it.
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	if _, ok := c.Signed("sessao"); !ok {
		return trilha.Errorf(http.StatusForbidden, "só quem entrou pode mudar a meta do mês")
	}
	return next()
}
