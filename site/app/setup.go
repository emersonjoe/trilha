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
	// Fontes do Google no site (o export estático não envia cabeçalhos, mas o dev sim).
	a.Security().CSPExtra = map[string][]string{
		"style-src": {"https://fonts.googleapis.com"},
		"font-src":  {"https://fonts.gstatic.com"},
	}
	return nil
}
