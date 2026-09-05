// Package app is the documentation site of Trilha, built with Trilha.
package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

// Layout is the document shell shared by every page.
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	title := "Trilha — framework web para Go com roteamento por arquivos"
	if t := c.Title(); t != "" {
		title = t + " · Trilha"
	}
	b := c.Base()
	active, _ := c.Get("secao").(string)
	return h.Html(h.Lang("pt-BR"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Meta(h.Name("color-scheme"), h.Content("light dark")),
			h.Meta(h.Name("description"), h.Content("Trilha: páginas, layouts, rotas de API e middleware descobertos a partir da pasta app/. Go puro, zero dependências.")),
			h.Title(h.Text(title)),
			h.Link(h.Rel("icon"), h.Href(b+"/favicon.svg"), h.Type("image/svg+xml")),
			h.Link(h.Rel("preconnect"), h.Href("https://fonts.googleapis.com")),
			h.Link(h.Rel("preconnect"), h.Href("https://fonts.gstatic.com"), h.Attr("crossorigin", "")),
			h.Link(h.Rel("stylesheet"), h.Href("https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&family=JetBrains+Mono:wght@400;600&display=swap")),
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/site.css"))),
			ui.AnalyticsScript(c),
			// Kit ui, used by the live demos (classes are prefixed, so the site is untouched).
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/ui.theme.css"))),
			h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/ui.css"))),
			// Applies the saved theme before first paint to avoid a flash (and mirrors it to the kit's .dark).
			h.Script(trilha.NonceAttr(c), h.Raw(`try{var t=localStorage.getItem("tema");if(t)document.documentElement.setAttribute("data-tema",t);var d=t?t==="escuro":matchMedia("(prefers-color-scheme: dark)").matches;document.documentElement.classList.toggle("dark",d)}catch(e){}`)),
		),
		h.Body(
			h.A(h.Class("pular"), h.Href("#principal"), h.Text("Ir para o conteúdo")),
			ui.Header(c, active),
			h.Main(h.ID("principal"), children),
			ui.Footer(c),
			h.Script(h.Src(c.Asset("/tema.js")), h.Defer()),
			h.Script(h.Src(c.Asset("/ui.js")), h.Defer()),
		),
	), nil
}
