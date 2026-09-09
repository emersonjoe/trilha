package auditoria

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// exige is the guard the policy builds; Middleware is what the scanner asks
// for.
var exige = sessao.Exige("usuarios", "administrar")

// Middleware guards the folder. A trail says who did what, from where — which
// makes it, itself, one of the most sensitive screens in the app: it is the
// list of everybody's actions, and reading it is an administrative act.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
