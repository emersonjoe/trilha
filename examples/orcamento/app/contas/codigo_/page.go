package codigo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/componentes"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page is the drill-down: GET /contas/{codigo}?mes=.
func Page(c *trilha.Ctx) (h.Node, error) {
	conta, ok := plano.Get(c.Param("codigo"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	m := c.Query("mes")
	if m == "" {
		m = plano.MesPadrao()
	}
	c.SetTitle(conta.Codigo + " " + conta.Nome)
	o, r := plano.Orcado(conta, m), plano.Realizado(conta, m)
	path := "/contas/" + conta.Codigo
	var corpo h.Node
	if conta.Analitica() {
		corpo = ui.Card(
			ui.CardHeader(ui.CardTitle("Lançamentos"), ui.CardDescription("Conta analítica: os valores vêm daqui.")),
			ui.CardContent(componentes.TabelaLancamentos(plano.Lancamentos(conta.Codigo, m))),
		)
	} else {
		corpo = ui.Card(
			ui.CardHeader(ui.CardTitle("Contas filhas"), ui.CardDescription("Dois níveis abaixo; clique para continuar descendo.")),
			ui.CardContent(componentes.Tabela(conta.Filhos, m, 1)),
		)
	}
	return ui.Stack(
		componentes.Trilha(conta, m),
		ui.Row(ui.H1(h.Text(conta.Codigo+" "+conta.Nome)), ui.Badge(ui.Outline(), h.Text(conta.Tipo)), ui.Spacer(), componentes.SeletorMes(path, m), componentes.DialogoLancamento(c, conta.Codigo, path+"?mes="+m)),
		ui.Grid(
			componentes.Cartao("Orçado", o, plano.MesLabel(m), h.Nil),
			componentes.Cartao("Realizado", r, "execução", componentes.Barra(o, r)),
			ui.Card(ui.CardHeader(ui.CardDescription("Variação")), ui.CardContent(h.Div(h.Class("ui-h2"), componentes.Variacao(conta.Tipo, o, r)), ui.Muted(h.Text(plano.Money(r-o))))),
		),
		corpo,
	), nil
}
