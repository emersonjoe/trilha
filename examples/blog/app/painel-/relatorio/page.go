// Package relatorio renders a page from an html/template file instead of the
// h DSL, using the tmpl adapter. Layouts and titles work the same way.
package relatorio

import (
	"embed"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/tmpl"
)

//go:embed relatorio.html
var files embed.FS

var t = tmpl.Must(files, "*.html")

// Page renders GET /relatorio.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Relatório")
	name := "relatorio"
	if c.Query("t") != "" {
		name = c.Query("t") // only to demonstrate a template error (500)
	}
	return tmpl.Node(t, name, map[string]any{
		"Titulo": "Relatório <de posts>",
		"Posts":  trilha.Use[*posts.Store](c).All(),
	}), nil
}
