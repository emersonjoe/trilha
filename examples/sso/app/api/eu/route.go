// Package eu devolve a sessão como JSON.
package eu

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
)

// GET responde com quem está logado. Sem sessão, o middleware responde 401
// em JSON — nada de redirecionar uma chamada de API para um formulário.
func GET(c *trilha.Ctx) error {
	u := sso.User(c)
	return c.JSON(200, map[string]any{"sub": u.Subject, "email": u.Email, "nome": u.Name, "papeis": u.Roles})
}
