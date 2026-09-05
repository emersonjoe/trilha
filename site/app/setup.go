package app

import (
	"os"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/ui"
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
	// Estatísticas sem cookies, só quando habilitadas por variável de ambiente
	// (no Pages: Settings → Variables → SITE_ANALYTICS = goatcounter:<código>).
	if v := strings.TrimSpace(os.Getenv("SITE_ANALYTICS")); v != "" {
		an, err := ui.ParseAnalytics(v)
		if err != nil {
			return err
		}
		a.Values()["analytics"] = an
		a.Security().CSPExtra["script-src"] = []string{an.ScriptOrigin}
		a.Security().CSPExtra["connect-src"] = []string{an.CountOrigin}
		a.Security().CSPExtra["img-src"] = []string{an.CountOrigin}
	}
	return nil
}
