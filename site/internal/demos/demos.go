// Package demos holds the "código → resultado" examples used by the home
// page and by chapters. Each demo carries its source as shown to the reader
// and the same code executed for real, so the result is never hand-written.
package demos

import (
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// Demo is a code snippet and the node it produces.
type Demo struct {
	Name   string
	Title  string
	Source string
	Node   func() h.Node
}

// A page in the "agenda" example app used throughout the docs.
type evento struct {
	Nome, Cidade string
	Vagas        int
}

var eventos = []evento{
	{"Encontro Go Campinas", "Campinas", 12},
	{"Oficina de HTTP", "Recife", 0},
	{"Noite de código", "Curitiba", 4},
}

// All lists the demos by name.
var All = map[string]Demo{
	"lista": {
		Name:  "lista",
		Title: "Uma lista a partir de dados",
		Source: `func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Ul(h.Class("eventos"),
		h.Map(eventos, func(e Evento) h.Node {
			return h.Li(
				h.Strong(h.Text(e.Nome)),
				h.Textf(" · %s · ", e.Cidade),
				h.IfElse(e.Vagas > 0,
					h.Textf("%d vagas", e.Vagas),
					h.Em(h.Text("lotado"))),
			)
		}),
	), nil
}`,
		Node: func() h.Node {
			return h.Ul(h.Class("eventos"),
				h.Map(eventos, func(e evento) h.Node {
					return h.Li(
						h.Strong(h.Text(e.Nome)),
						h.Textf(" · %s · ", e.Cidade),
						h.IfElse(e.Vagas > 0,
							h.Textf("%d vagas", e.Vagas),
							h.Em(h.Text("lotado"))),
					)
				}),
			)
		},
	},
	"escape": {
		Name:  "escape",
		Title: "Texto é escapado por padrão",
		Source: `nome := "<script>alert('oi')</script>"

return h.P(
	h.Text("Olá, "),
	h.Strong(h.Text(nome)),
), nil`,
		Node: func() h.Node {
			nome := "<script>alert('oi')</script>"
			return h.P(h.Text("Olá, "), h.Strong(h.Text(nome)))
		},
	},
	"layout": {
		Name:  "layout",
		Title: "Layouts recebem os filhos já renderizados",
		Source: `// app/eventos/layout.go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("agenda"),
		h.Nav(h.A(h.Href("/eventos"), h.Text("Todos")),
		      h.A(h.Href("/eventos/novo"), h.Text("Novo"))),
		children,
	), nil
}

// app/eventos/page.go
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Eventos")
	return h.H2(h.Text("Próximos eventos")), nil
}`,
		Node: func() h.Node {
			children := h.H2(h.Text("Próximos eventos"))
			return h.Section(h.Class("agenda"),
				h.Nav(h.A(h.Href("#"), h.Text("Todos")), h.A(h.Href("#"), h.Text("Novo"))),
				children,
			)
		},
	},
	"form": {
		Name:  "form",
		Title: "Formulário com CSRF em uma linha",
		Source: `func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Form(h.Method("post"), h.Action("/eventos/novo"),
		trilha.CSRFInput(c),
		h.Label(h.For("nome"), h.Text("Nome do evento")),
		h.Input(h.ID("nome"), h.Name("nome"), h.Required()),
		h.Button(h.Type("submit"), h.Text("Publicar")),
	), nil
}

func POST(c *trilha.Ctx) error {
	ev := agenda.Criar(c.Form("nome"))
	return c.Redirect("/eventos/" + ev.Slug)
}`,
		Node: func() h.Node {
			// O site é estático: tema.js intercepta o envio e mostra o fluxo que
			// o servidor faria. Sem JavaScript o formulário apenas recarrega a
			// página (method="get"), em vez de tentar um POST que o GitHub Pages
			// recusaria. Nada de manipulador inline: a CSP padrão os bloqueia.
			return h.Fragment(
				h.Form(h.Method("get"), h.Action("#"), h.Data("demo", "form"),
					h.Input(h.Type("hidden"), h.Name("_csrf"), h.Value("token-gerado-por-requisicao")),
					h.Label(h.For("nome"), h.Text("Nome do evento")),
					h.Input(h.ID("nome"), h.Name("nome"), h.Required()),
					h.Button(h.Type("submit"), h.Text("Publicar")),
				),
				h.Output(h.Class("demo-nota"), h.Data("demo-saida", ""),
					h.Text("Envie para ver o que o servidor faria (envio simulado no navegador; o POST real está no código ao lado).")),
			)
		},
	},
}

// Render returns the HTML of a demo card, or "" if unknown.
func Render(name string) string {
	d, ok := All[name]
	if !ok {
		return ""
	}
	out, err := h.Render(Card(d))
	if err != nil {
		return ""
	}
	return out
}

// Card renders the two-pane demo: source on the left, live result on the right.
func Card(d Demo) h.Node {
	return h.Div(h.Class("demo"),
		h.Div(h.Class("demo-codigo"),
			h.Div(h.Class("demo-rotulo"), h.Text("app/…/page.go")),
			h.Div(h.Class("codigo"), h.Data("lang", "go"), h.Pre(h.Code(h.Class("lang-go"), h.Raw(highlight(d.Source)))))),
		h.Div(h.Class("demo-resultado"),
			h.Div(h.Class("demo-rotulo"), h.Text("resultado")),
			h.Div(h.Class("demo-saida"), d.Node())),
	)
}

// highlight is injected by the ui package to avoid an import cycle.
var highlight = func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "&", "&amp;"), "<", "&lt;") }

// SetHighlighter sets the Go highlighter used by Card.
func SetHighlighter(f func(string) string) { highlight = f }
