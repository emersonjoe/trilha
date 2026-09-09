package chaves

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

var exige = sessao.Exige("usuarios", "administrar")

// Middleware guards the screen that issues keys. Issuing one is handing out
// access to the API, which is administration by any other name.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
