package convites

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// Convidar alguém é criar uma conta, então a regra é a mesma que administra
// usuários. O convite em si não pede login — ele é a pasta de baixo, que tem o
// próprio middleware.
var guarda = sessao.Exige("usuarios", "editar")

func Middleware(c *trilha.Ctx, next trilha.Next) error { return guarda(c, next) }
