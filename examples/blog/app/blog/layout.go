package blog

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/cache"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// Layout wraps everything under /blog.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	// cache.Once não é cache: a resposta vale por esta requisição e morre com
	// ela. É para a pergunta que o layout e a página fazem — aqui a contagem;
	// num app, quem está logado — sem passar o resultado de mão em mão.
	total, err := cache.Once(c, "posts:total", func() (int, error) {
		return trilha.Use[*posts.Store](c).Count(), nil
	})
	if err != nil {
		return nil, err
	}
	return h.Section(h.Class("blog"),
		h.Aside(
			ui.ButtonLink("/blog/novo", ui.Sm(), ui.Icon("plus"), h.Text("Novo post")),
			ui.Muted(h.Textf("%d posts", total)),
		),
		children,
	), nil
}
