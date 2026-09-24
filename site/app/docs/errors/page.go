// Package errors renders the error catalog, generated from the same Go table
// the gate prints (spec 161): every stable code with its cause, its fix and
// a page of its own. The search is a plain form answered on the server, so
// the catalog filters without JavaScript.
package errors

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/internal/checkerr"
	kitui "github.com/emersonjoe/trilha/ui"
)

// Page lists the catalog, narrowed by ?q= when one is given.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Error catalog")
	q := strings.TrimSpace(c.Request().URL.Query().Get("q"))
	docs := checkerr.Docs()
	if q != "" {
		var filtered []checkerr.Doc
		needle := strings.ToLower(q)
		for _, d := range docs {
			if strings.Contains(strings.ToLower(d.Code+" "+d.Title+" "+d.Cause), needle) {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}
	items := h.Map(docs, func(d checkerr.Doc) h.Node {
		return h.Li(h.ID(d.Code),
			h.A(h.Href(c.Base()+"/docs/errors/"+d.Code), h.Code(h.Text(d.Code))),
			h.Text(" — "+d.Title),
			h.P(h.Class("descricao"), h.Text(d.Cause)),
		)
	})
	empty := h.Nil
	if len(docs) == 0 {
		empty = kitui.Lead(h.Text("No code answers that search. Codes are E_ and all caps — try the family, like VULN."))
	}
	return h.Article(h.Class("conteudo"),
		h.P(h.Class("secao"), h.A(h.Href(c.Base()+"/reference/errors"), h.Text("Reference"))),
		h.H1(h.Text("Error catalog")),
		h.P(h.Class("descricao"), h.Text("Every stable code with its cause and its fix — the same table the gate prints.")),
		h.Form(h.Method("get"), h.Action(c.Base()+"/docs/errors"), h.Class("busca-erros"),
			kitui.SearchBox(c, c.Base()+"/docs/errors", kitui.SearchBoxOpts{Name: "q", Submit: "Search", Hint: "-", Placeholder: "filter by code, title or cause (try: vuln, duplicate)"}),
			h.If(q != "", h.P(h.Class("ui-muted"), h.Textf("Filtered by %q — ", q), h.A(h.Href(c.Base()+"/docs/errors"), h.Text("see every code")))),
		),
		h.Ul(items),
		empty,
	), nil
}
