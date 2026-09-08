// Package ordem is the ceiling of the swap model, and the way across it.
//
// Dragging is continuous state: while the pointer is down the truth about the
// order lives in the browser, and no round trip to the server can hold it.
// That is exactly the case an island is for. What the island does not become is
// the owner of the data: it writes the order into a hidden field, and the same
// POST → redirect → GET the rest of this app uses is what saves it. With the
// module blocked the page is still usable: each row carries a button that moves
// it one place, and the same handler answers both.
package ordem

import (
	"slices"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Page renders the reorderable list at GET /blog/ordem.
func Page(c *trilha.Ctx) (h.Node, error) {
	lista := trilha.Use[*posts.Store](c).All()
	c.SetTitle("Ordem dos posts")

	linhas := make([]h.Node, 0, len(lista))
	slugs := make([]string, 0, len(lista))
	for _, p := range lista {
		slugs = append(slugs, p.Slug)
		linhas = append(linhas, h.Li(
			h.Class("ui-row"), h.Data("slug", p.Slug),
			ui.Icon("menu", h.Class("ui-muted"), h.Aria("hidden", "true")),
			h.Span(h.Text(p.Title)), ui.Spacer(),
			// The way up and down without a pointer. The island does not hide
			// these: dragging is not reachable from a keyboard, and an island
			// that takes accessibility away is not an improvement.
			ui.Button(ui.Ghost(), ui.Sm(), h.Type("submit"), h.Name("mover"), h.Value("cima:"+p.Slug),
				h.Aria("label", "Subir "+p.Title), h.Text("↑")),
			ui.Button(ui.Ghost(), ui.Sm(), h.Type("submit"), h.Name("mover"), h.Value("baixo:"+p.Slug),
				h.Aria("label", "Descer "+p.Title), h.Text("↓")),
		))
	}

	return ui.Card(
		ui.CardHeader(
			ui.CardTitle("Ordem dos posts"),
			ui.CardDescription("Arraste para reordenar. O cliente manda enquanto você arrasta; o servidor manda quando você salva."),
		),
		ui.CardContent(h.Form(h.Method("post"), h.Action("/blog/ordem"), h.Class("ui-stack"),
			trilha.CSRFInput(c),
			// The island owns the dragging and nothing else. Its props are the
			// order the server rendered, so a module that loads late still
			// starts from the truth.
			c.Island("/ilha-ordem.js", map[string]any{"ordem": slugs},
				h.Input(h.Type("hidden"), h.Name("ordem"), h.Value(strings.Join(slugs, ","))),
				h.Ol(h.Class("ui-stack"), h.Data("lista", ""), h.Fragment(linhas...)),
			),
			h.Div(ui.Submit(h.Text("Salvar ordem")), h.Text(" "), ui.ButtonLink("/blog", ui.Ghost(), h.Text("Voltar"))),
		)),
	), nil
}

// POST saves the order and comes back to the list.
func POST(c *trilha.Ctx) error {
	var in struct {
		Ordem string `form:"ordem"`
		Mover string `form:"mover"`
	}
	if err := c.Bind(&in); err != nil {
		return err
	}
	slugs := strings.Split(in.Ordem, ",")
	for i := range slugs {
		slugs[i] = strings.TrimSpace(slugs[i])
	}
	if in.Mover != "" {
		slugs = mover(slugs, in.Mover)
	}
	trilha.Use[*posts.Store](c).Reorder(slugs)
	c.Flash("success", "Ordem salva.")
	return c.Redirect("/blog")
}

// mover applies one step of "cima:<slug>" or "baixo:<slug>", which is what the
// page does when the island never loaded. An unknown slug or a step off either
// end leaves the order alone: the button was stale, not malicious.
func mover(slugs []string, cmd string) []string {
	dir, slug, ok := strings.Cut(cmd, ":")
	if !ok {
		return slugs
	}
	i := slices.Index(slugs, slug)
	j := i - 1
	if dir == "baixo" {
		j = i + 1
	}
	if i < 0 || j < 0 || j >= len(slugs) {
		return slugs
	}
	slugs[i], slugs[j] = slugs[j], slugs[i]
	return slugs
}
