// Package code renders one entry of the error catalog, from the same Go
// table the gate prints (spec 161): cause, fix and an example, per stable
// code.
package code

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/internal/checkerr"
	siteui "github.com/emersonjoe/trilha/site/internal/ui"
)

// Page renders one entry, or 404 with the families a reader might have meant.
func Page(c *trilha.Ctx) (h.Node, error) {
	code := strings.ToUpper(c.Param("code"))
	d, ok := checkerr.ByCode(code)
	if !ok {
		return nil, trilha.ErrNotFound
	}
	c.SetTitle(d.Code)
	_ = siteui.Locale(c)
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.A(h.Href(c.Base()+"/docs/errors"), h.Text("Error catalog"))),
		h.H1(h.Code(h.Text(d.Code))),
		h.H2(h.Text(d.Title)),
		h.P(h.Text(d.Cause)),
		h.H2(h.Text("Fix")),
		h.P(h.Text(d.Fix)),
		h.If(d.Example != "", h.Pre(h.Code(h.Class("lang-go"), h.Text(d.Example)))),
	), nil
}
