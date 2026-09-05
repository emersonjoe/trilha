package api

import "github.com/emersonjoe/trilha"

// limit is created once: 5 requests/s per client with a burst of 20.
var limit = trilha.Limit(5, 20)

// Middleware applies the API rate limit to everything under /api.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return limit(c, next)
}
