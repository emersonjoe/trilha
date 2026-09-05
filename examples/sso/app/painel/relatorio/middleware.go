// Package relatorio exige um papel, não só sessão.
package relatorio

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Middleware exige o papel de administrador. Quem está logado sem o papel
// recebe 403: mandar de volta ao login viraria um laço.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.RequireAdmin(c, next) }
