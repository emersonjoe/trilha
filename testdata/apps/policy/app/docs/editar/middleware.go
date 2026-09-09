package editar

import "github.com/emersonjoe/trilha"

// A pasta de baixo declara a sua, e a de baixo ganha — chamando o
// RequirePolicy direto, que é a outra forma que a leitura reconhece.
var guarda = Auth.RequirePolicy(Politica, "docs", "administrar")

func Middleware(c *trilha.Ctx, next trilha.Next) error { return guarda(c, next) }
