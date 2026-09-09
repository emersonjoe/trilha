package docs

import (
	"github.com/emersonjoe/trilha"
	"example.com/policy/internal/acesso"
)

// A pasta inteira exige ver; escrever exige editar. É a pasta legível por um
// nível e gravável por outro, que é o caso que a matriz existe para expressar.
var ver = acesso.Exige("docs", "ver")

var editar = acesso.Exige("docs", "editar")

func Middleware(c *trilha.Ctx, next trilha.Next) error { return ver(c, next) }

func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error { return editar(c, next) }
