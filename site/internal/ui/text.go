package ui

import (
	"os"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// texts are the interface strings of the site, per locale. Content lives in
// Markdown; this table only covers the shell (header, footer, labels).
var texts = map[string]map[string]string{
	"en": {
		"site.title":       "Trilha — a Go web framework with file-based routing",
		"site.description": "Trilha: pages, layouts, API routes and middleware discovered from the app/ directory. Pure Go, zero dependencies.",
		"skip":             "Skip to content",
		"nav":              "Main",
		"theme":            "Toggle theme",
		"chapters":         "Chapters",
		"toc":              "On this page",
		"neighbors":        "Neighboring pages",
		"prev":             "Previous",
		"next":             "Next",
		"footer.license":   "Trilha is free software under the MIT license. ",
		"footer.repo":      "Code, issues and discussions on GitHub",
		"footer.built":     "This site was built with Trilha itself and exported with ",
		"analytics.1":      "Visits are counted without cookies and without personal data, using ",
		"analytics.2":      " (free software); the numbers are ",
		"analytics.public": "public",
		"notfound.title":   "Page not found",
		"notfound.h1":      "This trail does not exist",
		"notfound.p":       "The page you looked for is not here. Maybe the chapter was renamed.",
		"notfound.learn":   "Go to Learn",
		"notfound.home":    "Home",
	},
	"pt": {
		"site.title":       "Trilha — framework web para Go com roteamento por arquivos",
		"site.description": "Trilha: páginas, layouts, rotas de API e middleware descobertos a partir da pasta app/. Go puro, zero dependências.",
		"skip":             "Ir para o conteúdo",
		"nav":              "Principal",
		"theme":            "Alternar tema",
		"chapters":         "Capítulos",
		"toc":              "Nesta página",
		"neighbors":        "Páginas vizinhas",
		"prev":             "Anterior",
		"next":             "Próximo",
		"footer.license":   "Trilha é software livre sob licença MIT. ",
		"footer.repo":      "Código, issues e discussões no GitHub",
		"footer.built":     "Este site foi construído com o próprio Trilha e exportado com ",
		"analytics.1":      "Visitas são contadas sem cookies e sem dados pessoais, com o ",
		"analytics.2":      " (software livre); os números são ",
		"analytics.public": "públicos",
		"notfound.title":   "Página não encontrada",
		"notfound.h1":      "Essa trilha não existe",
		"notfound.p":       "A página que você procurou não está aqui. Talvez o capítulo tenha mudado de nome.",
		"notfound.learn":   "Ir para Aprender",
		"notfound.home":    "Início",
	},
}

// Locale returns the locale code of the current request (set by the
// middleware from the URL prefix); "en" by default.
func Locale(c *trilha.Ctx) string {
	if l, _ := c.Get("locale").(string); l != "" {
		return l
	}
	return docs.Locales[0].Code
}

// T returns an interface string in the request's locale.
func T(c *trilha.Ctx, key string) string {
	loc := Locale(c)
	if s, ok := texts[loc][key]; ok {
		return s
	}
	return texts["en"][key]
}

// SetAlternate records the path of this page in a locale, so the layout can
// emit hreflang links and the header can point to the translation.
func SetAlternate(c *trilha.Ctx, locale, path string) { c.Set("alt:"+locale, path) }

// Alternate returns the path of the current page in a locale, or "".
func Alternate(c *trilha.Ctx, locale string) string {
	p, _ := c.Get("alt:" + locale).(string)
	return p
}

// Origin is the public origin of the site (SITE_ORIGIN, e.g.
// "https://emersonjoe.github.io"), used to make hreflang URLs absolute. Empty
// means relative URLs.
func Origin() string { return strings.TrimRight(os.Getenv("SITE_ORIGIN"), "/") }
