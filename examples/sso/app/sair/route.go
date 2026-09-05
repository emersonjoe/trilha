// Package sair encerra a sessão.
package sair

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Kind: POST de formulário, com CSRF, e erro em HTML.
var Kind = trilha.KindPage

// POST apaga a sessão e, quando o provedor oferece, encerra lá também.
func POST(c *trilha.Ctx) error { return sso.Logout(c) }
