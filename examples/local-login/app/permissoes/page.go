// Package permissoes is the screen that edits the matrix: a role per row, a
// module per column, and a level in each cell. It is the most valuable screen
// in the app, so it sits behind the module that administers users — the
// middleware below is the rule, not the fact that the menu hides the link.
package permissoes

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/local-login/internal/sessao"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page draws the grid at GET /permissoes.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Permissões")
	return tela(c), nil
}

// POST saves what the grid posted and comes back to it.
func POST(c *trilha.Ctx) error {
	papeis, err := sessao.BindMatriz(c)
	if err != nil {
		return err
	}
	sessao.SalvarMatriz(papeis)
	// One line, and the trail knows who did it, from where and on which route:
	// changing who may do what is exactly the action somebody asks about later.
	c.Audit("permissao.alterou", "matriz", trilha.Fields{"papeis": len(papeis)})
	c.Flash("success", "Permissões salvas.")
	return c.Redirect("/permissoes")
}

func tela(c *trilha.Ctx) h.Node {
	return ui.Card(
		ui.CardHeader(
			ui.CardTitle("Permissões"),
			ui.CardDescription("Um papel por linha, um módulo por coluna. Sem JavaScript."),
		),
		ui.CardContent(ui.PolicyGrid(sessao.Policy, ui.PolicyGridOpts{
			Action: "/permissoes",
			CSRF:   trilha.CSRFInput(c),
			Labels: map[string]string{"relatorios": "Relatórios", "usuarios": "Usuários"},
			None:   "sem acesso",
			Submit: "Salvar",
		})),
	)
}
