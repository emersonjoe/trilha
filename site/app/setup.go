package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// Setup lists every documentation page for `trilha export`: chapters live
// under dynamic routes (/aprender/{slug}), so they are declared here.
func Setup(a *trilha.App) error {
	for _, p := range docs.All() {
		a.AddExportPath(p.Path())
	}
	return nil
}
