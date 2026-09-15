package code

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// Page renderiza uma entrada do catálogo de erros de runtime.
func Page(c *trilha.Ctx) (h.Node, error) {
	guide, ok := trilha.ErrorGuideByCode(strings.ToUpper(c.Param("code")))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	c.SetTitle(guide.Code)
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.A(h.Href(c.Base()+"/pt/docs/errors"), h.Text("Catálogo de erros"))),
		h.H1(h.Code(h.Text(guide.Code))),
		h.H2(h.Text(guide.Title)),
		h.P(h.Text(guide.Description)),
		h.H2(h.Text("Como corrigir")),
		h.P(h.Text(guide.Repair)),
		h.P(h.A(h.Href(c.Base()+translatedReference(guide.Reference)), h.Text("Leia a referência relacionada"))),
	), nil
}

func translatedReference(reference string) string {
	path, fragment, hasFragment := strings.Cut(reference, "#")
	section, slug, ok := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	if !ok {
		return reference
	}
	page, ok := docs.Get("en", section, slug)
	if !ok {
		return reference
	}
	translated, ok := docs.Translation(page, "pt")
	if !ok {
		return reference
	}
	if hasFragment {
		return translated.Path() + "#" + fragment
	}
	return translated.Path()
}
