// Package docs loads the Markdown content of the documentation site in every
// locale, builds the navigation and renders pages.
package docs

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/emersonjoe/trilha/site/internal/md"
)

//go:embed content
var content embed.FS

// Section is one top-level navigation entry with its pages in order.
// Slugs map to content/<locale>/<section>/<slug>.md ("" is index.md).
type Section struct {
	Key, Title string
	Slugs      []string
}

// Locale is one language of the site. Sections and slugs are parallel across
// locales, position by position: the translation of
// Locales[0].Sections[i].Slugs[j] is Locales[k].Sections[i].Slugs[j]. That is
// what makes the language switcher and the hreflang links work without a
// mapping table; the site tests check that every locale has the same shape.
type Locale struct {
	Code     string // "en", "pt": also the content directory
	Prefix   string // URL prefix: "" for the default locale, "/pt"
	Lang     string // <html lang>
	Name     string // label shown in the language switcher
	Sections []Section
}

// Locales lists the languages of the site; the first one is the default and
// lives at the root.
var Locales = []Locale{
	{Code: "en", Prefix: "", Lang: "en", Name: "English", Sections: []Section{
		{"learn", "Learn", []string{"", "pages-and-routes", "layouts", "html-with-h", "forms", "api", "middleware", "security", "observability", "authentication", "ui-kit", "ai-and-agents", "examples", "dev-and-production", "troubleshooting"}},
		{"reference", "Reference", []string{"", "conventions", "ctx", "h", "tmpl", "errors", "app", "security", "observability", "auth", "ui", "ai", "mcp", "cli", "performance"}},
	}},
	{Code: "pt", Prefix: "/pt", Lang: "pt-BR", Name: "Português", Sections: []Section{
		{"aprender", "Aprender", []string{"", "paginas-e-rotas", "layouts", "html-com-h", "formularios", "api", "middleware", "seguranca", "observabilidade", "autenticacao", "interface-com-ui", "ia-e-agentes", "exemplos", "dev-e-producao", "problemas-comuns"}},
		{"referencia", "Referência", []string{"", "convencoes", "ctx", "h", "tmpl", "erros", "app", "seguranca", "observabilidade", "auth", "ui", "ai", "mcp", "cli", "desempenho"}},
	}},
}

// LocaleOf returns the locale with the given code, or the default one.
func LocaleOf(code string) Locale {
	for _, l := range Locales {
		if l.Code == code {
			return l
		}
	}
	return Locales[0]
}

// Home is the path of the locale's home page.
func (l Locale) Home() string {
	if l.Prefix == "" {
		return "/"
	}
	return l.Prefix
}

// Page is one documentation page.
type Page struct {
	Locale      string // locale code
	Section     string // section key ("learn", "aprender"...)
	Slug        string // "" for the section index
	Title       string
	Description string
	Body        string // markdown

	section, index int // positions, used to find translations
}

// Path returns the URL path without base.
func (p Page) Path() string {
	path := LocaleOf(p.Locale).Prefix + "/" + p.Section
	if p.Slug != "" {
		path += "/" + p.Slug
	}
	return path
}

var (
	once  sync.Once
	pages map[string]Page
	order []Page
)

func load() {
	once.Do(func() {
		pages = map[string]Page{}
		for _, l := range Locales {
			for si, s := range l.Sections {
				for ji, slug := range s.Slugs {
					name := slug
					if name == "" {
						name = "index"
					}
					raw, err := fs.ReadFile(content, "content/"+l.Code+"/"+s.Key+"/"+name+".md")
					if err != nil {
						panic(fmt.Sprintf("docs: %s/%s/%s: %v", l.Code, s.Key, name, err))
					}
					p := parse(string(raw))
					p.Locale, p.Section, p.Slug, p.section, p.index = l.Code, s.Key, slug, si, ji
					pages[p.Path()] = p
					order = append(order, p)
				}
			}
		}
	})
}

// parse splits a simple front matter (title:, description:) from the body.
func parse(src string) Page {
	var p Page
	if !strings.HasPrefix(src, "---\n") {
		p.Body = src
		return p
	}
	rest := src[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		p.Body = src
		return p
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		k, v, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(k) {
		case "title":
			p.Title = strings.TrimSpace(v)
		case "description":
			p.Description = strings.TrimSpace(v)
		}
	}
	p.Body = rest[end+5:]
	return p
}

// Get returns a page by locale, section and slug.
func Get(locale, section, slug string) (Page, bool) {
	load()
	p, ok := pages[(Page{Locale: locale, Section: section, Slug: slug}).Path()]
	return p, ok
}

// All returns every page of every locale in navigation order.
func All() []Page {
	load()
	return order
}

// Pages returns the pages of one locale in navigation order.
func Pages(locale string) []Page {
	load()
	var out []Page
	for _, p := range order {
		if p.Locale == locale {
			out = append(out, p)
		}
	}
	return out
}

// Translation returns the same page in another locale.
func Translation(p Page, locale string) (Page, bool) {
	l := LocaleOf(locale)
	if p.section >= len(l.Sections) {
		return Page{}, false
	}
	s := l.Sections[p.section]
	if p.index >= len(s.Slugs) {
		return Page{}, false
	}
	return Get(l.Code, s.Key, s.Slugs[p.index])
}

// Neighbors returns the previous and next page within the same section and
// locale.
func Neighbors(p Page) (prev, next *Page) {
	load()
	for i, q := range order {
		if q.Path() != p.Path() {
			continue
		}
		if i > 0 && order[i-1].Locale == p.Locale && order[i-1].Section == p.Section {
			prev = &order[i-1]
		}
		if i+1 < len(order) && order[i+1].Locale == p.Locale && order[i+1].Section == p.Section {
			next = &order[i+1]
		}
	}
	return prev, next
}

// Rendered is a page converted to HTML.
type Rendered struct {
	Page
	HTML     string
	Headings []md.Heading
}

// Render converts the page body with links prefixed by base.
func Render(p Page, base string, demo func(string) string) Rendered {
	html, hs := md.Render(p.Body, md.Options{Base: base, Demo: demo, Locale: p.Locale})
	return Rendered{Page: p, HTML: html, Headings: hs}
}
