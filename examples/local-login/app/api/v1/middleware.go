// Package apiv1 is the public API of this app: it answers to a key and never
// to a session cookie. The middleware guards the whole folder.
package apiv1

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

var exige = sessao.Chaves.Require("documentos:ler")

// Middleware asks for a key with the read scope. A route under here that needs
// more says so on its own file — the folder is the floor, not the ceiling.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
