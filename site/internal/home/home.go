// Package home renders the landing page of the site in each locale.
package home

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/demos"
	"github.com/emersonjoe/trilha/site/internal/docs"
	"github.com/emersonjoe/trilha/site/internal/md"
	"github.com/emersonjoe/trilha/site/internal/ui"
)

const treePT = `app/
├── layout.go             ← <html> de todas as páginas
├── page.go               ← GET /
├── eventos/
│   ├── layout.go         ← envolve /eventos/**
│   ├── page.go           ← GET /eventos
│   ├── novo/page.go      ← GET /eventos/novo + POST do formulário
│   └── slug_/page.go     ← GET /eventos/{slug}
├── organizador-/         ← grupo: não entra na URL
│   ├── middleware.go     ← exige login
│   └── painel/page.go    ← GET /painel
└── api/eventos/route.go  ← GET e POST /api/eventos`

const treeEN = `app/
├── layout.go             ← <html> of every page
├── page.go               ← GET /
├── events/
│   ├── layout.go         ← wraps /events/**
│   ├── page.go           ← GET /events
│   ├── new/page.go       ← GET /events/new + the form's POST
│   └── slug_/page.go     ← GET /events/{slug}
├── organizer-/           ← group: not part of the URL
│   ├── middleware.go     ← requires login
│   └── dashboard/page.go ← GET /dashboard
└── api/events/route.go   ← GET and POST /api/events`

const install = "go install github.com/emersonjoe/trilha/cmd/trilha@latest\ntrilha new agenda && cd agenda && trilha dev"

func code(lang, src string) h.Node {
	body := src
	if lang == "go" {
		body = md.HighlightGo(src)
	} else {
		body = escape(src)
	}
	return h.Div(h.Class("codigo"), h.Data("lang", lang), h.Pre(h.Code(h.Class("lang-"+lang), h.Raw(body))))
}

// Page renders the home page in the given locale.
func Page(c *trilha.Ctx, locale string) (h.Node, error) {
	for _, l := range docs.Locales {
		ui.SetAlternate(c, l.Code, l.Home())
	}
	if locale == "pt" {
		return pt(c), nil
	}
	return en(c), nil
}

func demo(locale, name string) h.Node {
	d, _ := demos.Get(locale, name)
	return demos.Card(locale, d)
}

func en(c *trilha.Ctx) h.Node {
	b := c.Base()
	return h.Fragment(
		h.Section(h.Class("heroi"),
			h.H1(h.Text("Go pages out of folders.")),
			h.P(h.Class("lema"), h.Text("Trilha is a web framework for Go: create a file under "), h.Code(h.Text("app/")), h.Text(" and it becomes a route. Nested layouts, forms, APIs, middleware, a dev server with live reload and a single binary. Nothing beyond the standard library.")),
			h.Div(h.Class("acoes"),
				h.A(h.Class("botao primario"), h.Href(b+"/learn"), h.Text("Learn Trilha")),
				h.A(h.Class("botao"), h.Href(b+"/reference"), h.Text("API reference")),
			),
			code("bash", install),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("One folder, one route")),
			h.P(h.Text("There is no route table. The "), h.Code(h.Text("app/")), h.Text(" tree is the table: "), h.Code(h.Text("page.go")), h.Text(" answers a page, "), h.Code(h.Text("route.go")), h.Text(" answers JSON, "), h.Code(h.Text("layout.go")), h.Text(" wraps everything below it and "), h.Code(h.Text("middleware.go")), h.Text(" intercepts the subtree. Dynamic segments use the "), h.Code(h.Text("_")), h.Text(" suffix and route groups the "), h.Code(h.Text("-")), h.Text(" suffix, because "), h.Code(h.Text("[slug]")), h.Text(" is not a valid import path in Go.")),
			code("text", treeEN),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("HTML in Go, escaped by default")),
			h.P(h.Text("The "), h.Code(h.Text("h")), h.Text(" package is a typed DSL: elements and attributes are functions, "), h.Code(h.Text("h.Map")), h.Text(" and "), h.Code(h.Text("h.If")), h.Text(" cover control flow and every text is escaped. The result on the right was not written by hand: it is the code that ran while this site was built.")),
			demo("en", "lista"),
			demo("en", "escape"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Layouts that nest")),
			h.P(h.Text("Every folder may have a "), h.Code(h.Text("layout.go")), h.Text(". The page renders first and is handed to the innermost layout, which hands it to the outer one, up to the root "), h.Code(h.Text("<html>")), h.Text(". The title the page sets with "), h.Code(h.Text("c.SetTitle")), h.Text(" reaches all of them.")),
			demo("en", "layout"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Forms and APIs without ceremony")),
			h.P(h.Text("Export "), h.Code(h.Text("POST")), h.Text(" in the same "), h.Code(h.Text("page.go")), h.Text(" and the form arrives with its CSRF token verified. In "), h.Code(h.Text("route.go")), h.Text(", each HTTP method is a function. Errors are values: "), h.Code(h.Text("trilha.ErrNotFound")), h.Text(" becomes a 404, "), h.Code(h.Text("c.Redirect")), h.Text(" a 303, and any other error a 500 with a stack trace only in development.")),
			demo("en", "form"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Save, see.")),
			h.P(h.Code(h.Text("trilha dev")), h.Text(" watches "), h.Code(h.Text("app/")), h.Text(", regenerates the route registry, recompiles, swaps the process and tells the browser. A cycle takes about a second; CSS changes land in milliseconds, without recompiling. A build error becomes a page in the browser that goes away when you fix it.")),
			h.Div(h.Class("grade-3"),
				h.Div(h.Class("cartao"), h.H3(h.Text("Zero dependencies")), h.P(h.Text("Runtime and CLI use only the standard library. The router is Go 1.22's "), h.Code(h.Text("http.ServeMux")), h.Text("."))),
				h.Div(h.Class("cartao"), h.H3(h.Text("No runtime magic")), h.P(h.Text("The CLI generates a Go file with the route registry. The compiler checks every signature. No "), h.Code(h.Text("reflect")), h.Text("."))),
				h.Div(h.Class("cartao"), h.H3(h.Text("One binary")), h.P(h.Code(h.Text("trilha build")), h.Text(" embeds "), h.Code(h.Text("public/")), h.Text(" and produces an executable that runs on its own. "), h.Code(h.Text("trilha export")), h.Text(" produces a static site, like this one."))),
			),
		),

		h.Section(h.Class("bloco chamada"),
			h.H2(h.Text("Start with the first page")),
			h.P(h.Text("The learning trail goes from "), h.Code(h.Text("trilha new")), h.Text(" to deployment, building an events agenda. Every chapter ends with a challenge.")),
			h.A(h.Class("botao primario"), h.Href(b+"/learn"), h.Text("Learn Trilha")),
		),
	)
}

func pt(c *trilha.Ctx) h.Node {
	b := c.Base()
	return h.Fragment(
		h.Section(h.Class("heroi"),
			h.H1(h.Text("Páginas Go a partir de pastas.")),
			h.P(h.Class("lema"), h.Text("Trilha é um framework web para Go: crie um arquivo em "), h.Code(h.Text("app/")), h.Text(" e ele vira uma rota. Layouts aninhados, formulários, APIs, middleware, servidor de desenvolvimento com recarga e um binário só. Nada além da biblioteca padrão.")),
			h.Div(h.Class("acoes"),
				h.A(h.Class("botao primario"), h.Href(b+"/pt/aprender"), h.Text("Aprender Trilha")),
				h.A(h.Class("botao"), h.Href(b+"/pt/referencia"), h.Text("Referência da API")),
			),
			code("bash", install),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Uma pasta, uma rota")),
			h.P(h.Text("Não existe tabela de rotas. A árvore de "), h.Code(h.Text("app/")), h.Text(" é a tabela: "), h.Code(h.Text("page.go")), h.Text(" responde uma página, "), h.Code(h.Text("route.go")), h.Text(" responde JSON, "), h.Code(h.Text("layout.go")), h.Text(" envolve tudo que está abaixo e "), h.Code(h.Text("middleware.go")), h.Text(" intercepta a subárvore. Segmentos dinâmicos usam o sufixo "), h.Code(h.Text("_")), h.Text(" e grupos de rota o sufixo "), h.Code(h.Text("-")), h.Text(", porque "), h.Code(h.Text("[slug]")), h.Text(" não é um caminho de import válido em Go.")),
			code("text", treePT),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("HTML em Go, com escape por padrão")),
			h.P(h.Text("O pacote "), h.Code(h.Text("h")), h.Text(" é um DSL tipado: elementos e atributos são funções, "), h.Code(h.Text("h.Map")), h.Text(" e "), h.Code(h.Text("h.If")), h.Text(" cobrem o fluxo de controle e todo texto é escapado. O resultado ao lado não foi escrito à mão: é o código executado durante a construção deste site.")),
			demo("pt", "lista"),
			demo("pt", "escape"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Layouts que se encaixam")),
			h.P(h.Text("Cada pasta pode ter um "), h.Code(h.Text("layout.go")), h.Text(". A página é renderizada primeiro e passa para o layout mais interno, que passa para o de fora, até o "), h.Code(h.Text("<html>")), h.Text(" raiz. O título definido pela página com "), h.Code(h.Text("c.SetTitle")), h.Text(" chega a todos eles.")),
			demo("pt", "layout"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Formulários e APIs sem cerimônia")),
			h.P(h.Text("Exporte "), h.Code(h.Text("POST")), h.Text(" no mesmo "), h.Code(h.Text("page.go")), h.Text(" e o formulário chega com token CSRF verificado. Em "), h.Code(h.Text("route.go")), h.Text(", cada método HTTP é uma função. Erros são valores: "), h.Code(h.Text("trilha.ErrNotFound")), h.Text(" vira 404, "), h.Code(h.Text("c.Redirect")), h.Text(" vira 303, qualquer outro erro vira 500 com stack só em desenvolvimento.")),
			demo("pt", "form"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Salvou, viu.")),
			h.P(h.Code(h.Text("trilha dev")), h.Text(" observa "), h.Code(h.Text("app/")), h.Text(", regenera o registro de rotas, recompila, troca o processo e avisa o navegador. Um ciclo leva cerca de um segundo; mudanças em CSS chegam em milissegundos, sem recompilar. Erro de compilação vira uma página no navegador que some quando você corrige.")),
			h.Div(h.Class("grade-3"),
				h.Div(h.Class("cartao"), h.H3(h.Text("Zero dependências")), h.P(h.Text("Runtime e CLI usam só a biblioteca padrão. O roteador é o "), h.Code(h.Text("http.ServeMux")), h.Text(" do Go 1.22."))),
				h.Div(h.Class("cartao"), h.H3(h.Text("Sem mágica em runtime")), h.P(h.Text("A CLI gera um arquivo Go com o registro de rotas. O compilador confere cada assinatura. Sem "), h.Code(h.Text("reflect")), h.Text("."))),
				h.Div(h.Class("cartao"), h.H3(h.Text("Um binário")), h.P(h.Code(h.Text("trilha build")), h.Text(" embute "), h.Code(h.Text("public/")), h.Text(" e produz um executável que sobe sozinho. "), h.Code(h.Text("trilha export")), h.Text(" gera um site estático, como este."))),
			),
		),

		h.Section(h.Class("bloco chamada"),
			h.H2(h.Text("Comece pela primeira página")),
			h.P(h.Text("A trilha de aprendizado vai do "), h.Code(h.Text("trilha new")), h.Text(" até o deploy, construindo uma agenda de eventos. Cada capítulo termina com um desafio.")),
			h.A(h.Class("botao primario"), h.Href(b+"/pt/aprender"), h.Text("Aprender Trilha")),
		),
	)
}

func escape(s string) string {
	var out []rune
	for _, ch := range s {
		switch ch {
		case '&':
			out = append(out, []rune("&amp;")...)
		case '<':
			out = append(out, []rune("&lt;")...)
		case '>':
			out = append(out, []rune("&gt;")...)
		default:
			out = append(out, ch)
		}
	}
	return string(out)
}
