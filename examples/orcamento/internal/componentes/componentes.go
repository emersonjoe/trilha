// Package componentes holds the reusable, nested UI pieces of the budget
// example. Each function returns an h.Node and composes the kit.
package componentes

import (
	"fmt"
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/orcamento/internal/plano"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Variacao is a badge with the deviation from budget; over ±10 % stands out.
func Variacao(tipo string, orcado, realizado int64) h.Node {
	if orcado == 0 && realizado == 0 {
		return ui.Badge(ui.Outline(), h.Text("—"))
	}
	pct := plano.Variacao(orcado, realizado)
	label := fmt.Sprintf("%+.0f%%", pct)
	// Spending above budget or revenue below it is bad.
	bad := (tipo == "despesa" && pct > 10) || (tipo == "receita" && pct < -10)
	good := (tipo == "despesa" && pct < -10) || (tipo == "receita" && pct > 10)
	switch {
	case bad:
		return ui.Badge(ui.Destructive(), h.Text(label))
	case good:
		return ui.Badge(h.Text(label))
	default:
		return ui.Badge(ui.Secondary(), h.Text(label))
	}
}

// Barra shows realized as a share of budget.
func Barra(orcado, realizado int64) h.Node {
	if orcado <= 0 {
		return h.Nil
	}
	return ui.Progress(int(realizado/100), int(orcado/100))
}

// Linha renders one account row, then its children (recursively) up to
// maxNivel below it, indented with ui.Depth. Nesting on the page mirrors the
// nesting in the data.
func Linha(c *plano.Conta, mes string, nivel, maxNivel int) h.Node {
	o, r := plano.Orcado(c, mes), plano.Realizado(c, mes)
	nome := h.Node(h.A(h.Href("/contas/"+c.Codigo+"?mes="+mes), h.Text(c.Nome)))
	if nivel == 0 {
		nome = h.Strong(nome)
	}
	row := h.Tr(ui.Depth(nivel),
		h.Td(ui.Muted(h.Text(c.Codigo)), h.Text(" "), nome),
		h.Td(ui.Num(), h.Text(plano.Money(o))),
		h.Td(ui.Num(), h.Text(plano.Money(r))),
		h.Td(h.Class("barra"), Barra(o, r)),
		h.Td(ui.Num(), Variacao(c.Tipo, o, r)),
	)
	if nivel >= maxNivel || c.Analitica() {
		return row
	}
	return h.Fragment(row, h.Map(c.Filhos, func(f *plano.Conta) h.Node { return Linha(f, mes, nivel+1, maxNivel) }))
}

// Tabela renders accounts (and their descendants) as a tree table.
func Tabela(contas []*plano.Conta, mes string, maxNivel int) h.Node {
	return ui.Table(
		h.Thead(h.Tr(h.Th(h.Text("Conta")), h.Th(ui.Num(), h.Text("Orçado")), h.Th(ui.Num(), h.Text("Realizado")), h.Th(h.Text("Execução")), h.Th(ui.Num(), h.Text("Variação")))),
		h.Tbody(h.Map(contas, func(c *plano.Conta) h.Node { return Linha(c, mes, 0, maxNivel) })),
	)
}

// Cartao is one KPI card of the overview.
func Cartao(titulo string, valor int64, sub string, node h.Node) h.Node {
	return ui.Card(
		ui.CardHeader(ui.CardDescription(titulo)),
		ui.CardContent(h.Div(h.Class("ui-h2"), h.Text(plano.Money(valor))), ui.Muted(h.Text(sub)), node),
	)
}

// ResumoCards renders the four KPI cards.
func ResumoCards(r plano.Resumo) h.Node {
	return ui.Grid(
		Cartao("Receita realizada", r.ReceitaReal, fmt.Sprintf("%.0f%% de %s", r.PctReceita, plano.Money(r.ReceitaOrcada)), Barra(r.ReceitaOrcada, r.ReceitaReal)),
		Cartao("Despesa realizada", r.DespesaReal, fmt.Sprintf("%.0f%% de %s", r.PctDespesa, plano.Money(r.DespesaOrcada)), Barra(r.DespesaOrcada, r.DespesaReal)),
		Cartao("Resultado", r.ResultadoReal, "orçado "+plano.Money(r.ResultadoOrcado), h.Nil),
		Cartao("Saldo × orçado", r.ResultadoReal-r.ResultadoOrcado, "diferença do resultado", Variacao("receita", r.ResultadoOrcado, r.ResultadoReal)),
	)
}

// SeletorMes is a GET form that changes ?mes= keeping the current path.
func SeletorMes(path, mes string) h.Node {
	opts := make([]ui.Option, 0, 4)
	for _, m := range plano.Meses() {
		opts = append(opts, ui.Option{Value: m, Label: plano.MesLabel(m)})
	}
	return h.Form(h.Method("get"), h.Action(path), h.Class("ui-row"),
		ui.Label(h.For("mes"), h.Text("Mês")),
		ui.Select(h.ID("mes"), h.Name("mes"), h.Attr("onchange", ""), h.Data("submit", ""), ui.SelectOptions(opts, mes)),
		ui.Submit(ui.Outline(), ui.Sm(), h.Text("Ver")),
	)
}

// Trilha (breadcrumb) from the overview down to the account.
func Trilha(c *plano.Conta, mes string) h.Node {
	crumbs := []ui.Crumb{{Label: "Orçamento", Href: "/?mes=" + mes}}
	for _, p := range c.Caminho() {
		crumbs = append(crumbs, ui.Crumb{Label: p.Codigo + " " + p.Nome, Href: "/contas/" + p.Codigo + "?mes=" + mes})
	}
	crumbs[len(crumbs)-1].Href = ""
	return ui.Breadcrumb(crumbs...)
}

// TabelaLancamentos lists the entries of an analytic account.
func TabelaLancamentos(list []plano.Lancamento) h.Node {
	if len(list) == 0 {
		return ui.Muted(h.Text("Nenhum lançamento no mês."))
	}
	var total int64
	for _, l := range list {
		total += l.Valor
	}
	return ui.Table(
		h.Thead(h.Tr(h.Th(h.Text("Data")), h.Th(h.Text("Descrição")), h.Th(ui.Num(), h.Text("Valor")))),
		h.Tbody(h.Map(list, func(l plano.Lancamento) h.Node {
			return h.Tr(h.Td(h.Time(h.Datetime(l.Data.Format("2006-01-02")), h.Text(l.Data.Format("02/01")))), h.Td(h.Text(l.Descricao)), h.Td(ui.Num(), h.Text(plano.Money(l.Valor))))
		})),
		h.Tfoot(h.Tr(h.Td(h.Attr("colspan", "2"), h.Text("Total")), h.Td(ui.Num(), h.Text(plano.Money(total))))),
	)
}

// FormLancamento is the entry form, used inside the dialog and on its own page.
func FormLancamento(c *trilha.Ctx, in plano.Lancamento, errs trilha.FieldErrors, voltar string) h.Node {
	contas := []ui.Option{{Value: "", Label: "Escolha a conta"}}
	for _, a := range plano.Analiticas() {
		contas = append(contas, ui.Option{Value: a.Codigo, Label: a.Codigo + " " + a.Nome})
	}
	data := ""
	if !in.Data.IsZero() {
		data = in.Data.Format("2006-01-02")
	}
	return h.Form(h.Method("post"), h.Action("/lancamentos"), h.Class("ui-stack"), h.Attr("novalidate", ""), trilha.CSRFInput(c),
		h.Input(h.Type("hidden"), h.Name("voltar"), h.Value(voltar)),
		ui.Field("conta", "Conta analítica", ui.Select(h.ID("conta"), h.Name("conta"), ui.InvalidIf(errs, "conta"), ui.SelectOptions(contas, in.Conta)), ui.Errors(errs, "conta")),
		h.Div(h.Class("ui-grid"),
			ui.Field("data", "Data", ui.Input(h.ID("data"), h.Name("data"), h.Type("date"), h.Value(data), ui.InvalidIf(errs, "data")), ui.Errors(errs, "data")),
			ui.Field("valor", "Valor (R$)", ui.Input(h.ID("valor"), h.Name("valor"), h.Attr("inputmode", "decimal"), h.Placeholder("1.234,56"), h.Value(in.ValorTxt), ui.InvalidIf(errs, "valor")), ui.Errors(errs, "valor")),
		),
		ui.Field("descricao", "Descrição", ui.Input(h.ID("descricao"), h.Name("descricao"), h.Value(in.Descricao), ui.InvalidIf(errs, "descricao")), ui.Errors(errs, "descricao")),
		ui.DialogFooter(ui.DialogClose(ui.Ghost(), h.Text("Cancelar")), ui.Submit(h.Text("Lançar"))),
	)
}

// DialogoLancamento wraps the form in a dialog with its trigger.
func DialogoLancamento(c *trilha.Ctx, conta, voltar string) h.Node {
	return h.Fragment(
		ui.DialogTrigger("novo", ui.Icon("plus"), h.Text("Novo lançamento")),
		ui.Dialog("novo", "Novo lançamento", FormLancamento(c, plano.Lancamento{Conta: conta}, nil, voltar)),
	)
}

// Pct formats a float percentage.
func Pct(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + "%" }
