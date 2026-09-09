// Package resumo é a parte cara da tela de documentos: a conta que varre tudo
// e demora. Ela não segura a página — o ui.Defer serve o resto na hora e pede
// este pedaço logo depois.
package resumo

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/documentos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page responde os dois pedidos com o mesmo bloco: o fragmento que o ui.Defer
// busca, e a página inteira para quem clicou no link do <noscript>.
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() != "" {
		return bloco(c), nil
	}
	c.SetTitle("Resumo dos documentos")
	return h.Div(ui.H1(h.Text("Resumo dos documentos")), bloco(c)), nil
}

// bloco carrega o id que o ui.Defer pôs na página: é o elemento que vai ser
// trocado, e é por isso que o id aparece nos dois lugares — o mesmo contrato do
// ui.Poll.
func bloco(c *trilha.Ctx) h.Node {
	// A demora que justifica a tela: aqui é um sleep, na sua aplicação é a
	// consulta que varre a tabela inteira.
	if c.Env() != trilha.Dev {
		time.Sleep(30 * time.Millisecond)
	}
	docs, total := documentos.Buscar(documentos.Consulta{Limite: 1000})
	var bytes int
	for _, d := range docs {
		bytes += d.Bytes
	}
	return h.Div(h.ID("resumo"),
		ui.Card(ui.CardHeader(ui.CardTitle("Resumo")), ui.CardContent(
			h.P(h.Textf("%d documentos", total)),
			h.P(ui.Bytes(c, int64(bytes))),
			h.P(h.Textf("%d ainda na fila", documentos.Pendentes())),
		)))
}
