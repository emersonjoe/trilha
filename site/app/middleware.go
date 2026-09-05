package app

import (
	"strings"

	"github.com/emersonjoe/trilha"
)

// Middleware tells the layout which top-level section is active.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	p := strings.TrimPrefix(c.Request().URL.Path, "/")
	if i := strings.Index(p, "/"); i >= 0 {
		p = p[:i]
	}
	c.Set("secao", p)
	return next()
}
