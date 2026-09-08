package permissoes

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// exige is the guard the policy builds; Middleware is what the scanner asks
// for. Two lines instead of one because the convention is a function with a
// fixed signature, and a var of the right type is not one.
var exige = sessao.Exige("usuarios", "administrar")

// Middleware guards the whole folder: the screen that edits the matrix is the
// most valuable one in the app, so it sits behind the module that administers
// users. The menu hiding the link is cosmetic — this is the rule.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
