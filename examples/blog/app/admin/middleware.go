package admin

import "github.com/emersonjoe/trilha"

// Middleware guards /admin: requires the session cookie set by /login.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	user, ok := c.Signed("sessao") // cookie assinado: não dá para forjar
	if !ok {
		return trilha.RedirectCode("/login?next="+c.Request().URL.Path, 302)
	}
	c.Set("user", user)
	return next()
}
