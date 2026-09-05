package painel

import "github.com/emersonjoe/trilha"

// Middleware runs for every route in the group, after the root middleware.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	c.Set("area", "painel")
	c.Header("X-Area", "painel")
	return next()
}
