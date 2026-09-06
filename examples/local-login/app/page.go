package app

import (
	"os"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page explains what the example is.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Login local · Trilha")
	api := os.Getenv("API_URL")
	if api == "" {
		api = "(vazio: defina API_URL para ligar o proxy)"
	}
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Sessão própria, API de fora")),
		h.P(h.Text("O login confere a senha na tabela do app (PBKDF2, no formato que o Python grava) e guarda na sessão o token que a API espera.")),
		h.P(h.Text("Tudo abaixo de /api/ é encaminhado para "+api+" com esse token no Authorization.")),
		h.Ul(
			h.Li(h.Text("ana@exemplo.com / segredo-da-ana — analista")),
			h.Li(h.Text("bia@exemplo.com / segredo-da-bia — admin")),
		),
	), nil
}
