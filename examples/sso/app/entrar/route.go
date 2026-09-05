// Package entrar começa o login.
package entrar

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Kind: é uma rota de navegador, então os erros saem como página.
var Kind = trilha.KindPage

// GET redireciona para o provedor.
func GET(c *trilha.Ctx) error { return sso.Start(c) }
