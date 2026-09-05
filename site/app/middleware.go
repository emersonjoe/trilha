package app

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// Middleware tells the layout which locale and top-level section are active.
// The locale comes from the URL prefix ("/pt"); the default locale has none.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	p := strings.TrimPrefix(c.Request().URL.Path, "/")
	locale := docs.Locales[0].Code
	for _, l := range docs.Locales[1:] {
		code := strings.TrimPrefix(l.Prefix, "/")
		if p == code || strings.HasPrefix(p, code+"/") {
			locale = l.Code
			p = strings.TrimPrefix(strings.TrimPrefix(p, code), "/")
			break
		}
	}
	if i := strings.Index(p, "/"); i >= 0 {
		p = p[:i]
	}
	c.Set("locale", locale)
	c.Set("secao", p)
	return next()
}
