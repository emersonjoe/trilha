package admin

import "github.com/emersonjoe/trilha"

// Middleware guards /admin: requires the session cookie set by /login.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	ck, err := c.Cookie("session")
	if err != nil || ck.Value != "ok" {
		return trilha.RedirectCode("/login?next="+c.Request().URL.Path, 302)
	}
	c.Set("user", "admin")
	return next()
}
