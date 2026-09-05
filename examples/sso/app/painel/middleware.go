// Package painel é a área que exige sessão.
package painel

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Middleware exige login em tudo abaixo de /painel.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.Require(c, next) }
