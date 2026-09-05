// Package docs loads the Markdown content of the documentation site, builds
// the navigation and renders pages.
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

// Page is one documentation page.
type Page struct {
	Section     string // "aprender" | "referencia"
	Slug        string // "" for the section index
	Title       string
	Description string
	Body        string // markdown
}

// Path returns the URL path without base.
func (p Page) Path() string {
	if p.Slug == "" {
		return "/" + p.Section
	}
	return "/" + p.Section + "/" + p.Slug
}

// Sections define the navigation order. Slugs map to content/<section>/<slug>.md
// (the empty slug maps to index.md).
var Sections = []struct {
	Key, Title string
	Slugs      []string
}{
	{"aprender", "Aprender", []string{"", "paginas-e-rotas", "layouts", "html-com-h", "formularios", "api", "middleware", "seguranca", "observabilidade", "interface-com-ui", "ia-e-agentes", "exemplos", "dev-e-producao", "problemas-comuns"}},
	{"referencia", "Referência", []string{"", "convencoes", "ctx", "h", "tmpl", "erros", "app", "seguranca", "observabilidade", "ui", "ai", "mcp", "cli", "desempenho"}},
}

var (
	once  sync.Once
	pages map[string]Page
	order []Page
)

func load() {
	once.Do(func() {
		pages = map[string]Page{}
		for _, s := range Sections {
			for _, slug := range s.Slugs {
				name := slug
				if name == "" {
					name = "index"
				}
				raw, err := fs.ReadFile(content, "content/"+s.Key+"/"+name+".md")
				if err != nil {
					panic(fmt.Sprintf("docs: %s/%s: %v", s.Key, name, err))
				}
				p := parse(string(raw))
				p.Section, p.Slug = s.Key, slug
				pages[p.Path()] = p
				order = append(order, p)
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

// Get returns a page by section and slug.
func Get(section, slug string) (Page, bool) {
	load()
	p, ok := pages[(Page{Section: section, Slug: slug}).Path()]
	return p, ok
}

// All returns every page in navigation order.
func All() []Page {
	load()
	return order
}

// Neighbors returns the previous and next page within the same section.
func Neighbors(p Page) (prev, next *Page) {
	load()
	for i, q := range order {
		if q.Path() != p.Path() {
			continue
		}
		if i > 0 && order[i-1].Section == p.Section {
			prev = &order[i-1]
		}
		if i+1 < len(order) && order[i+1].Section == p.Section {
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
	html, hs := md.Render(p.Body, md.Options{Base: base, Demo: demo})
	return Rendered{Page: p, HTML: html, Headings: hs}
}
