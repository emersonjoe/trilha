// Package errors renderiza o catálogo de erros, gerado da mesma tabela Go que
// o portão imprime (spec 161): cada código estável com a causa, o conserto e
// uma página própria. A busca é um formulário respondido no servidor, para o
// catálogo filtrar sem JavaScript.
package errors

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/internal/checkerr"
	kitui "github.com/emersonjoe/trilha/ui"
)

// Page lista o catálogo, estreitado por ?q= quando houver.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Catálogo de erros")
	q := strings.TrimSpace(c.Request().URL.Query().Get("q"))
	docs := checkerr.Docs()
	if q != "" {
		var filtrados []checkerr.Doc
		agulha := strings.ToLower(q)
		for _, d := range docs {
			if strings.Contains(strings.ToLower(d.Code+" "+d.Title+" "+d.Cause), agulha) {
				filtrados = append(filtrados, d)
			}
		}
		docs = filtrados
	}
	items := h.Map(docs, func(d checkerr.Doc) h.Node {
		return h.Li(h.ID(d.Code),
			h.A(h.Href(c.Base()+"/pt/docs/errors/"+d.Code), h.Code(h.Text(d.Code))),
			h.Text(" — "+d.Title),
			h.P(h.Class("descricao"), h.Text(d.Cause)),
		)
	})
	vazio := h.Nil
	if len(docs) == 0 {
		vazio = kitui.Lead(h.Text("Nenhum código responde a essa busca. Os códigos são E_ e caixa alta — tente a família, como VULN."))
	}
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.A(h.Href(c.Base()+"/pt/referencia/erros"), h.Text("Referência"))),
		h.H1(h.Text("Catálogo de erros")),
		h.P(h.Class("descricao"), h.Text("Todo código estável com a causa e o conserto — a mesma tabela que o portão imprime.")),
		h.Form(h.Method("get"), h.Action(c.Base()+"/pt/docs/errors"), h.Class("busca-erros"),
			kitui.SearchBox(c, c.Base()+"/pt/docs/errors", kitui.SearchBoxOpts{Name: "q", Submit: "Buscar", Hint: "-", Placeholder: "filtre por código, título ou causa (tente: vuln, duplicate)"}),
			h.If(q != "", h.P(h.Class("ui-muted"), h.Textf("Filtrado por %q — ", q), h.A(h.Href(c.Base()+"/pt/docs/errors"), h.Text("ver todos os códigos")))),
		),
		h.Ul(items),
		vazio,
	), nil
}
