// Package api agrupa as rotas de API.
package api

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Middleware exige sessão em /api.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.Require(c, next) }
