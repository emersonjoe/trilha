package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/componentes"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the month overview: KPI cards and the level-1/2 tree.
func Page(c *trilha.Ctx) (h.Node, error) {
	m := mes(c)
	c.SetTitle("Orçamento " + plano.MesLabel(m))
	return ui.Stack(
		ui.Row(ui.H1(h.Text("Orçamento "), ui.Muted(h.Text(plano.MesLabel(m)))), ui.Spacer(), componentes.SeletorMes("/", m), componentes.DialogoLancamento(c, "", "/?mes="+m)),
		componentes.ResumoCards(plano.Resumir(m)),
		ui.Card(
			ui.CardHeader(ui.CardTitle("Plano de contas"), ui.CardDescription("Clique numa conta para aprofundar. Contas sintéticas somam as filhas.")),
			ui.CardContent(componentes.Tabela(plano.Raizes(), m, 1)),
		),
	), nil
}
