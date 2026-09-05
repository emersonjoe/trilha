package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/blog/internal/posts"
)

// Setup runs once before the server starts.
func Setup(a *trilha.App) error {
	posts.Seed()
	a.Values()["site"] = "Trilha Blog"
	return nil
}
