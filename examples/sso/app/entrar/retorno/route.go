// Package retorno termina o login.
package retorno

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// Kind: erros aqui são vistos por gente, não por um cliente HTTP.
var Kind = trilha.KindPage

// GET valida o retorno do provedor e cria a sessão.
func GET(c *trilha.Ctx) error { return sso.Callback(c) }
