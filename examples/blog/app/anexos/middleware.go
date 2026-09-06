package anexos

import "github.com/emersonjoe/trilha"

// Middleware raises the body limit for this route only, before anything reads
// the body — CSRF parses the form, so the decision has to come first.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20)
		if err := c.NoReadDeadline(); err != nil {
			return err
		}
	}
	return next()
}
