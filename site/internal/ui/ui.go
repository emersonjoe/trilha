// Package ui holds the shared building blocks of the documentation site:
// the document shell, sidebar, table of contents and helpers.
package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/demos"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/md"
)

func init() { demos.SetHighlighter(md.HighlightGo) }

// Repo is the project URL.
const Repo = "https://github.com/emersonjoe/trilha"

// Logo is an original mark: three waypoints joined by a winding path.
func Logo() h.Node {
	return h.Raw(`<svg class="logo" viewBox="0 0 32 32" width="28" height="28" aria-hidden="true"><path d="M5 25c6 0 5-12 11-12s5 12 11 12" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/><circle cx="5" cy="25" r="3" fill="currentColor"/><circle cx="16" cy="13" r="3" fill="currentColor"/><circle cx="27" cy="25" r="3" fill="currentColor"/></svg>`)
}

// Header renders the top bar: sections of the current locale, GitHub, the
// language switcher (to the same page in the other locale) and the theme
// toggle.
func Header(c *trilha.Ctx, active string) h.Node {
	b := c.Base()
	loc := docs.LocaleOf(Locale(c))
	link := func(href, key, label string) h.Node {
		return h.A(h.Href(b+href), h.If(active == key, h.Aria("current", "page")), h.Text(label))
	}
	return h.Header(h.Class("topo"),
		h.A(h.Class("marca"), h.Href(b+loc.Home()), Logo(), h.Span(h.Text("Trilha"))),
		h.Nav(h.Class("topo-nav"), h.Aria("label", T(c, "nav")),
			h.Map(loc.Sections, func(s docs.Section) h.Node {
				return link(loc.Prefix+"/"+s.Key, s.Key, s.Title)
			}),
			h.A(h.Href(Repo), h.Rel("noopener"), h.Text("GitHub")),
			h.Map(docs.Locales, func(other docs.Locale) h.Node {
				if other.Code == loc.Code {
					return h.Nil
				}
				href := Alternate(c, other.Code)
				if href == "" {
					href = other.Home()
				}
				return h.A(h.Class("idioma"), h.Href(b+href), h.Attr("hreflang", other.Lang), h.Lang(other.Lang), h.Text(other.Name))
			}),
		),
		h.Button(h.Class("tema"), h.Type("button"), h.Data("tema-toggle", "1"), h.Aria("label", T(c, "theme")), h.Raw(`<svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path class="sol" d="M12 4V2m0 20v-2m8-8h2M2 12h2m13.7-5.7 1.4-1.4M4.9 19.1l1.4-1.4m0-11.4L4.9 4.9m14.2 14.2-1.4-1.4M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><path class="lua" d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/></svg>`)),
	)
}

// Alternates renders the <link rel="alternate" hreflang> tags of the current
// page, when the page declared its translations (SetAlternate).
func Alternates(c *trilha.Ctx) h.Node {
	origin, b := Origin(), c.Base()
	var links []h.Node
	for _, l := range docs.Locales {
		p := Alternate(c, l.Code)
		if p == "" {
			return h.Nil // every locale or none
		}
		links = append(links, h.Link(h.Rel("alternate"), h.Attr("hreflang", l.Lang), h.Href(origin+b+p)))
	}
	// The default locale is what a visitor with no matching language gets.
	links = append(links, h.Link(h.Rel("alternate"), h.Attr("hreflang", "x-default"), h.Href(origin+b+Alternate(c, docs.Locales[0].Code))))
	return h.Fragment(links...)
}

// Footer renders the bottom of every page.
func Footer(c *trilha.Ctx) h.Node {
	return h.Footer(h.Class("rodape"),
		h.P(h.Text(T(c, "footer.license")), h.A(h.Href(Repo), h.Rel("noopener"), h.Text(T(c, "footer.repo"))), h.Text(".")),
		h.P(h.Small(h.Text(T(c, "footer.built")), h.Code(h.Text("trilha export")), h.Text("."))),
		AnalyticsNote(c),
	)
}

// Sidebar renders the section navigation for docs pages.
func Sidebar(c *trilha.Ctx, current docs.Page) h.Node {
	b := c.Base()
	loc := docs.LocaleOf(current.Locale)
	return h.Nav(h.Class("lateral"), h.Aria("label", T(c, "chapters")),
		h.Map(loc.Sections, func(s docs.Section) h.Node {
			return h.Fragment(
				h.H3(h.Text(s.Title)),
				h.Ol(h.Map(s.Slugs, func(slug string) h.Node {
					p, _ := docs.Get(loc.Code, s.Key, slug)
					return h.Li(h.A(h.Href(b+p.Path()), h.If(p.Path() == current.Path(), h.Aria("current", "page")), h.Text(p.Title)))
				})),
			)
		}),
	)
}

// TOC renders the in-page table of contents from h2/h3 headings.
func TOC(c *trilha.Ctx, r docs.Rendered) h.Node {
	var items []md.Heading
	for _, hd := range r.Headings {
		if hd.Level == 2 || hd.Level == 3 {
			items = append(items, hd)
		}
	}
	if len(items) < 2 {
		return h.Nil
	}
	return h.Nav(h.Class("sumario"), h.Aria("label", T(c, "toc")),
		h.H3(h.Text(T(c, "toc"))),
		h.Ul(h.Map(items, func(hd md.Heading) h.Node {
			return h.Li(h.Class("n"+itoa(hd.Level)), h.A(h.Href("#"+hd.ID), h.Text(hd.Text)))
		})),
	)
}

// DocPage renders a chapter or reference page inside the docs shell.
func DocPage(c *trilha.Ctx, p docs.Page) (h.Node, error) {
	c.SetTitle(p.Title)
	for _, l := range docs.Locales {
		if t, ok := docs.Translation(p, l.Code); ok {
			SetAlternate(c, l.Code, t.Path())
		}
	}
	r := docs.Render(p, c.Base(), demos.Renderer(p.Locale))
	prev, next := docs.Neighbors(p)
	b := c.Base()
	return h.Div(h.Class("docs"),
		h.Details(h.Class("lateral-movel"), h.Summary(h.Text(T(c, "chapters"))), Sidebar(c, p)),
		h.Div(h.Class("lateral-fixa"), Sidebar(c, p)),
		h.Article(h.Class("conteudo"),
			h.P(h.Class("secao"), h.Text(sectionTitle(p))),
			h.H1(h.Text(p.Title)),
			h.If(p.Description != "", h.P(h.Class("descricao"), h.Text(p.Description))),
			h.Raw(r.HTML),
			h.Nav(h.Class("vizinhos"), h.Aria("label", T(c, "neighbors")),
				h.If(prev != nil, h.A(h.Class("anterior"), h.Href(b+safePath(prev)), h.Small(h.Text(T(c, "prev"))), h.Span(h.Text(safeTitle(prev))))),
				h.If(next != nil, h.A(h.Class("proximo"), h.Href(b+safePath(next)), h.Small(h.Text(T(c, "next"))), h.Span(h.Text(safeTitle(next))))),
			),
		),
		h.Aside(h.Class("sumario-coluna"), TOC(c, r)),
	), nil
}

// Legacy redirects an old (pre-i18n) Portuguese path to its home under /pt.
func Legacy(c *trilha.Ctx, path string) (h.Node, error) {
	return nil, trilha.RedirectCode(c.Base()+"/pt"+path, 301)
}

func safePath(p *docs.Page) string {
	if p == nil {
		return ""
	}
	return p.Path()
}

func safeTitle(p *docs.Page) string {
	if p == nil {
		return ""
	}
	return p.Title
}

func sectionTitle(p docs.Page) string {
	for _, s := range docs.LocaleOf(p.Locale).Sections {
		if s.Key == p.Section {
			return s.Title
		}
	}
	return p.Section
}

func itoa(n int) string { return string(rune('0' + n)) }
