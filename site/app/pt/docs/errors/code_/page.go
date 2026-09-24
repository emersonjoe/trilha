// Package code renderiza uma entrada do catálogo de erros, da mesma tabela Go
// que o portão imprime (spec 161): causa, conserto e exemplo, por código
// estável.
package code

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/internal/checkerr"
)

// Page renderiza uma entrada, ou 404 com as famílias que o leitor quis dizer.
func Page(c *trilha.Ctx) (h.Node, error) {
	code := strings.ToUpper(c.Param("code"))
	d, ok := checkerr.ByCode(code)
	if !ok {
		return nil, trilha.ErrNotFound
	}
	c.SetTitle(d.Code)
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.A(h.Href(c.Base()+"/pt/docs/errors"), h.Text("Catálogo de erros"))),
		h.H1(h.Code(h.Text(d.Code))),
		h.H2(h.Text(d.Title)),
		h.P(h.Text(d.Cause)),
		h.H2(h.Text("Conserto")),
		h.P(h.Text(d.Fix)),
		h.If(d.Example != "", h.Pre(h.Code(h.Class("lang-go"), h.Text(d.Example)))),
	), nil
}
