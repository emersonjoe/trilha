// Package documentos mostra a tela que todo app de gestão tem: filtro, tabela
// ordenável, paginação — tudo na URL, com o ui.DataTable — e um fragmento que
// se atualiza sozinho enquanto a fila anda, com o ui.Poll.
package documentos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// consulta é o que esta tela lê da URL: a listagem embutida mais o filtro que
// só ela tem. O Bind lê os dois de uma vez e aplica os limites da listagem.
type consulta struct {
	trilha.ListParams
	Tipo string `form:"tipo"`
}

// colunas declara o que a tabela mostra e, com Sort, o que o repositório pode
// receber como ordem: o que não está aqui é derrubado antes da consulta.
var colunas = ui.Columns[documentos.Documento]{
	{Key: "nome", Label: "Documento", Sort: true, Cell: func(d documentos.Documento) h.Node {
		return h.Text(d.Nome)
	}},
	{Key: "tipo", Label: "Tipo", Cell: func(d documentos.Documento) h.Node {
		return ui.Badge(ui.Outline(), h.Text(d.Tipo))
	}},
	{Key: "tamanho", Label: "Tamanho", Sort: true, Num: true, Cell: func(d documentos.Documento) h.Node {
		return h.Text(d.Tamanho())
	}},
	{Key: "status", Label: "Status", Sort: true, Cell: func(d documentos.Documento) h.Node {
		return ui.Badge(h.Text(d.Status))
	}},
}

// Page responde GET /documentos: a página inteira, a tabela quando o script
// pede só ela, ou o bloco da fila a cada tique do ui.Poll.
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "fila" {
		// O tique é o worker deste exemplo: a fila anda, e quando não sobra
		// nada o servidor encerra o polling na própria resposta.
		documentos.Andar()
		return fila(c), nil
	}
	var q consulta
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	q.Tipo = tipoValido(q.Tipo)
	tabela := lista(c, q)
	if c.Fragment() == "lista" {
		return tabela, nil
	}
	c.SetTitle("Documentos")
	return h.Div(
		ui.H1(h.Text("Documentos")),
		fila(c),
		tabela,
		ui.LiveScript(c),
	), nil
}

// tipoValido é para o filtro o que o Restrict é para a ordem: o que vem da URL
// só chega ao repositório se for um dos valores que a tela oferece.
func tipoValido(t string) string {
	for _, v := range documentos.Tipos() {
		if v == t {
			return t
		}
	}
	return ""
}

// lista monta a tabela a partir do que veio na URL.
func lista(c *trilha.Ctx, q consulta) h.Node {
	docs, total := documentos.Buscar(documentos.Consulta{
		Q: q.Q, Tipo: q.Tipo, Ordem: q.Sort, Asc: q.Asc(),
		Offset: q.Offset(), Limite: q.Limit(),
	})
	return ui.DataTable(c, colunas, docs, ui.ListState{
		Params:  q.ListParams,
		Total:   total,
		ID:      "lista",
		Search:  "Buscar no nome",
		Filters: filtro(q.Tipo),
		Caption: "Documentos recebidos",
		RowHref: func(i int) string { return "/documentos?q=" + docs[i].Nome },
		// Without this the DataTable already draws a sensible empty state, and
		// it tells "no documents" from "no results for that term". This one is
		// here because the app speaks Portuguese and the kit's default does not.
		Empty: ui.Empty(ui.EmptyOpts{
			Icon:  "info",
			Title: "Nenhum documento com esse filtro",
			Hint:  "Tente outro termo, ou limpe a busca.",
		}),
	})
}

// filtro é o campo que só esta tela tem; o ui.DataTable o põe dentro do mesmo
// <form method=get> da busca, então filtrar é mudar a URL.
func filtro(sel string) h.Node {
	opcoes := []ui.Option{{Value: "", Label: "Todos os tipos"}}
	for _, t := range documentos.Tipos() {
		opcoes = append(opcoes, ui.Option{Value: t, Label: t})
	}
	return ui.Select(h.Name("tipo"), h.Aria("label", "Tipo"), ui.SelectOptions(opcoes, sel))
}

// fila é o fragmento vivo: o ui.Poll pede este mesmo pedaço de dois em dois
// segundos, e o PollStop encerra quando não há mais o que esperar.
func fila(c *trilha.Ctx) h.Node {
	n := documentos.Pendentes()
	if n == 0 {
		c.PollStop()
		return h.Div(h.ID("fila"), ui.Poll("2s", ""),
			ui.Alert("Fila vazia", ui.AlertDescription(h.Text("Todos os documentos foram processados."))))
	}
	return h.Div(h.ID("fila"), ui.Poll("2s", ""),
		ui.Alert("Processando", ui.AlertDescription(h.Textf("%d documentos ainda na fila.", n))))
}
