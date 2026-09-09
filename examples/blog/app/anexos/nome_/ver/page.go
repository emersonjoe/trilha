// Package anexover mostra o anexo ao lado do que se sabe sobre ele — a tela que
// todo app de documentos tem. Quem enquadra é o ui.Preview; quem deixa ser
// enquadrado é o c.Inline da rota do arquivo.
package anexover

import (
	"net/http"
	"net/url"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/anexos"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	a, ok := anexos.Por(c.Param("nome"))
	if !ok {
		return nil, trilha.Errorf(http.StatusNotFound, "anexo não encontrado")
	}
	arquivo := "/anexos/" + url.PathEscape(a.Nome)
	c.SetTitle(a.Nome)
	return h.Div(
		ui.H1(h.Text(a.Nome)),
		ui.Muted(h.Text(a.Tamanho()+" · "+a.Tipo)),
		ui.Preview(c, arquivo+"?ver=1", ui.PreviewOpts{
			Title:    a.Nome,
			Type:     a.Tipo,
			Height:   "60vh",
			Download: arquivo,
		}),
		h.P(h.A(h.Href("/anexos"), h.Text("Voltar aos anexos"))),
	), nil
}
