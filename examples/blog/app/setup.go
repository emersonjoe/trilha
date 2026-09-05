package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// Config runs before trilha.New: the place to change fields New derives
// from (Logger, Secret, RateLimit, TrustedProxies) or to read your own config.
func Config(cfg *trilha.Config) {
	// Static files are versioned by the cache-busting query in the layout, so
	// they can be cached for a year.
	cfg.StaticCacheControl = "public, max-age=31536000, immutable"
}

// Setup runs once before the server starts.
func Setup(a *trilha.App) error {
	posts.Seed()
	a.Values()["site"] = "Trilha Blog"
	// Limite global brando; /api tem o seu próprio em app/api/middleware.go.
	a.Security().CSPExtra = map[string][]string{"img-src": {"https:"}}
	return nil
}
