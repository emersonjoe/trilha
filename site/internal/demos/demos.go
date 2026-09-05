// Package demos holds the "code → result" examples used by the home page and
// by chapters. Each demo carries its source as shown to the reader and the
// same code executed for real, so the result is never hand-written. Demos
// exist once per locale: the text inside the code is part of the lesson.
package demos

import (
	"sort"
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

var byLocale = map[string]map[string]Demo{"en": {}, "pt": {}}

// add registers a demo for one locale.
func add(locale string, d Demo) { byLocale[locale][d.Name] = d }

// Get returns a demo by locale and name.
func Get(locale, name string) (Demo, bool) {
	d, ok := byLocale[locale][name]
	return d, ok
}

// Names lists the demo names of a locale, sorted.
func Names(locale string) []string {
	var out []string
	for n := range byLocale[locale] {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Renderer returns the "@demo name" renderer for a locale (for md.Options).
func Renderer(locale string) func(name string) string {
	return func(name string) string {
		d, ok := Get(locale, name)
		if !ok {
			return ""
		}
		out, err := h.Render(Card(locale, d))
		if err != nil {
			return ""
		}
		return out
	}
}

// Card renders the two-pane demo: source on the left, live result on the right.
func Card(locale string, d Demo) h.Node {
	result := "result"
	if locale == "pt" {
		result = "resultado"
	}
	return h.Div(h.Class("demo"),
		h.Div(h.Class("demo-codigo"),
			h.Div(h.Class("demo-rotulo"), h.Text("app/…/page.go")),
			h.Div(h.Class("codigo"), h.Data("lang", "go"), h.Pre(h.Code(h.Class("lang-go"), h.Raw(highlight(d.Source)))))),
		h.Div(h.Class("demo-resultado"),
			h.Div(h.Class("demo-rotulo"), h.Text(result)),
			h.Div(h.Class("demo-saida"), d.Node())),
	)
}

// highlight is injected by the ui package to avoid an import cycle.
var highlight = func(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "&", "&amp;"), "<", "&lt;") }

// SetHighlighter sets the Go highlighter used by Card.
func SetHighlighter(f func(string) string) { highlight = f }

// The "agenda" example app used throughout the docs, in both languages.
type evento struct {
	Nome, Cidade string
	Vagas        int
}

var eventos = []evento{
	{"Encontro Go Campinas", "Campinas", 12},
	{"Oficina de HTTP", "Recife", 0},
	{"Noite de código", "Curitiba", 4},
}

type event struct {
	Name, City string
	Seats      int
}

var events = []event{
	{"Go Meetup Campinas", "Campinas", 12},
	{"HTTP Workshop", "Recife", 0},
	{"Code Night", "Curitiba", 4},
}

// formDemo is the static stand-in for the CSRF form: tema.js intercepts the
// submit and shows the flow the server would run. Without JavaScript the form
// only reloads the page (method="get") instead of attempting a POST that a
// static host would refuse. No inline handlers: the default CSP blocks them.
func formDemo(post, target, label, button, note string) h.Node {
	return h.Fragment(
		h.Form(h.Method("get"), h.Action("#"), h.Data("demo", "form"), h.Data("demo-post", post), h.Data("demo-target", target),
			h.Input(h.Type("hidden"), h.Name("_csrf"), h.Value("token-generated-per-request")),
			h.Label(h.For("nome"), h.Text(label)),
			h.Input(h.ID("nome"), h.Name("nome"), h.Required()),
			h.Button(h.Type("submit"), h.Text(button)),
		),
		h.Output(h.Class("demo-nota"), h.Data("demo-saida", ""), h.Text(note)),
	)
}

func init() {
	// ---- pt ----
	add("pt", Demo{
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
	})
	add("pt", Demo{
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
	})
	add("pt", Demo{
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
	})
	add("pt", Demo{
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
			return formDemo("/eventos/novo", "/eventos/", "Nome do evento", "Publicar",
				"Envie para ver o que o servidor faria (envio simulado no navegador; o POST real está no código ao lado).")
		},
	})

	// ---- en ----
	add("en", Demo{
		Name:  "lista",
		Title: "A list from data",
		Source: `func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Ul(h.Class("events"),
		h.Map(events, func(e Event) h.Node {
			return h.Li(
				h.Strong(h.Text(e.Name)),
				h.Textf(" · %s · ", e.City),
				h.IfElse(e.Seats > 0,
					h.Textf("%d seats", e.Seats),
					h.Em(h.Text("sold out"))),
			)
		}),
	), nil
}`,
		Node: func() h.Node {
			return h.Ul(h.Class("events"),
				h.Map(events, func(e event) h.Node {
					return h.Li(
						h.Strong(h.Text(e.Name)),
						h.Textf(" · %s · ", e.City),
						h.IfElse(e.Seats > 0,
							h.Textf("%d seats", e.Seats),
							h.Em(h.Text("sold out"))),
					)
				}),
			)
		},
	})
	add("en", Demo{
		Name:  "escape",
		Title: "Text is escaped by default",
		Source: `name := "<script>alert('hi')</script>"

return h.P(
	h.Text("Hello, "),
	h.Strong(h.Text(name)),
), nil`,
		Node: func() h.Node {
			name := "<script>alert('hi')</script>"
			return h.P(h.Text("Hello, "), h.Strong(h.Text(name)))
		},
	})
	add("en", Demo{
		Name:  "layout",
		Title: "Layouts receive the children already rendered",
		Source: `// app/events/layout.go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("agenda"),
		h.Nav(h.A(h.Href("/events"), h.Text("All")),
		      h.A(h.Href("/events/new"), h.Text("New"))),
		children,
	), nil
}

// app/events/page.go
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Events")
	return h.H2(h.Text("Upcoming events")), nil
}`,
		Node: func() h.Node {
			children := h.H2(h.Text("Upcoming events"))
			return h.Section(h.Class("agenda"),
				h.Nav(h.A(h.Href("#"), h.Text("All")), h.A(h.Href("#"), h.Text("New"))),
				children,
			)
		},
	})
	add("en", Demo{
		Name:  "form",
		Title: "A form with CSRF in one line",
		Source: `func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Form(h.Method("post"), h.Action("/events/new"),
		trilha.CSRFInput(c),
		h.Label(h.For("name"), h.Text("Event name")),
		h.Input(h.ID("name"), h.Name("name"), h.Required()),
		h.Button(h.Type("submit"), h.Text("Publish")),
	), nil
}

func POST(c *trilha.Ctx) error {
	ev := agenda.Create(c.Form("name"))
	return c.Redirect("/events/" + ev.Slug)
}`,
		Node: func() h.Node {
			return formDemo("/events/new", "/events/", "Event name", "Publish",
				"Submit to see what the server would do (simulated in the browser; the real POST is in the code on the left).")
		},
	})
}
