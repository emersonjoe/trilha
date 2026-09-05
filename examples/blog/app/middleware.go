package app

import (
	"time"

	"github.com/emersonjoe/trilha"
)

// Middleware runs for every route: measures the handler and exposes the
// duration as a response header.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	start := time.Now()
	c.Set("site", c.App().Values()["site"])
	err := next()
	c.Header("Server-Timing", "app;dur="+time.Since(start).Round(time.Microsecond).String())
	return err
}
