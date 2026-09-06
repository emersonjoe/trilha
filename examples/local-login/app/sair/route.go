// Package sair ends the session.
package sair

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
)

// POST clears the session. It needs the CSRF token because app/kind.go says
// this tree is pages — nothing in this file says it.
func POST(c *trilha.Ctx) error { return sessao.Flow.Logout(c) }
