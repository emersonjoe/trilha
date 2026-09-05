package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/sso/internal/sso"
	"github.com/emersonjoe/trilha/h"
)

// Page explica o exemplo e mostra o estado da configuração.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Entrar com Entra ID ou Keycloak")
	var aviso h.Node
	if !sso.Configurado() {
		aviso = h.Div(h.Class("aviso"),
			h.Strong(h.Text("Login indisponível. ")),
			h.Text(sso.Motivo()),
		)
	}
	return h.Div(h.Class("cartao"),
		h.H1(h.Text("Entrar com Entra ID ou Keycloak")),
		h.P(h.Text("O fluxo inteiro está em três rotas: /entrar começa, /entrar/retorno "+
			"termina, /sair encerra. O resto do app só usa Require e RequireRole.")),
		aviso,
		h.Dl(
			h.Dt(h.Text("Provedor")), h.Dd(h.Text(sso.Descricao())),
			h.Dt(h.Text("Área protegida")), h.Dd(h.A(h.Href("/painel"), h.Text("/painel"))),
			h.Dt(h.Text("Exige papel "+sso.AdminRole())), h.Dd(h.A(h.Href("/painel/relatorio"), h.Text("/painel/relatorio"))),
			h.Dt(h.Text("API protegida")), h.Dd(h.A(h.Href("/api/eu"), h.Text("/api/eu"))),
		),
	), nil
}
