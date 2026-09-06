// Package painel is what only a logged-in person sees.
package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// Middleware requires a session in everything under /painel.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return sessao.Flow.Require()(c, next)
}
