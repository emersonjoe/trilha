package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/demos"
	"github.com/emersonjoe/trilha/site/internal/md"
)

const arvore = `app/
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

// Page renders the home page.
func Page(c *trilha.Ctx) (h.Node, error) {
	b := c.Base()
	code := func(lang, src string) h.Node {
		body := src
		if lang == "go" {
			body = md.HighlightGo(src)
		} else {
			body = escape(src)
		}
		return h.Div(h.Class("codigo"), h.Data("lang", lang), h.Pre(h.Code(h.Class("lang-"+lang), h.Raw(body))))
	}
	return h.Fragment(
		h.Section(h.Class("heroi"),
			h.H1(h.Text("Páginas Go a partir de pastas.")),
			h.P(h.Class("lema"), h.Text("Trilha é um framework web para Go: crie um arquivo em "), h.Code(h.Text("app/")), h.Text(" e ele vira uma rota. Layouts aninhados, formulários, APIs, middleware, servidor de desenvolvimento com recarga e um binário só. Nada além da biblioteca padrão.")),
			h.Div(h.Class("acoes"),
				h.A(h.Class("botao primario"), h.Href(b+"/aprender"), h.Text("Aprender Trilha")),
				h.A(h.Class("botao"), h.Href(b+"/referencia"), h.Text("Referência da API")),
			),
			code("bash", "go install github.com/emersonjoe/trilha/cmd/trilha@latest\ntrilha new agenda && cd agenda && trilha dev"),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Uma pasta, uma rota")),
			h.P(h.Text("Não existe tabela de rotas. A árvore de "), h.Code(h.Text("app/")), h.Text(" é a tabela: "), h.Code(h.Text("page.go")), h.Text(" responde uma página, "), h.Code(h.Text("route.go")), h.Text(" responde JSON, "), h.Code(h.Text("layout.go")), h.Text(" envolve tudo que está abaixo e "), h.Code(h.Text("middleware.go")), h.Text(" intercepta a subárvore. Segmentos dinâmicos usam o sufixo "), h.Code(h.Text("_")), h.Text(" e grupos de rota o sufixo "), h.Code(h.Text("-")), h.Text(", porque "), h.Code(h.Text("[slug]")), h.Text(" não é um caminho de import válido em Go.")),
			code("text", arvore),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("HTML em Go, com escape por padrão")),
			h.P(h.Text("O pacote "), h.Code(h.Text("h")), h.Text(" é um DSL tipado: elementos e atributos são funções, "), h.Code(h.Text("h.Map")), h.Text(" e "), h.Code(h.Text("h.If")), h.Text(" cobrem o fluxo de controle e todo texto é escapado. O resultado ao lado não foi escrito à mão: é o código executado durante a construção deste site.")),
			demos.Card(demos.All["lista"]),
			demos.Card(demos.All["escape"]),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Layouts que se encaixam")),
			h.P(h.Text("Cada pasta pode ter um "), h.Code(h.Text("layout.go")), h.Text(". A página é renderizada primeiro e passa para o layout mais interno, que passa para o de fora, até o "), h.Code(h.Text("<html>")), h.Text(" raiz. O título definido pela página com "), h.Code(h.Text("c.SetTitle")), h.Text(" chega a todos eles.")),
			demos.Card(demos.All["layout"]),
		),

		h.Section(h.Class("bloco"),
			h.H2(h.Text("Formulários e APIs sem cerimônia")),
			h.P(h.Text("Exporte "), h.Code(h.Text("POST")), h.Text(" no mesmo "), h.Code(h.Text("page.go")), h.Text(" e o formulário chega com token CSRF verificado. Em "), h.Code(h.Text("route.go")), h.Text(", cada método HTTP é uma função. Erros são valores: "), h.Code(h.Text("trilha.ErrNotFound")), h.Text(" vira 404, "), h.Code(h.Text("c.Redirect")), h.Text(" vira 303, qualquer outro erro vira 500 com stack só em desenvolvimento.")),
			demos.Card(demos.All["form"]),
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
			h.A(h.Class("botao primario"), h.Href(b+"/aprender"), h.Text("Aprender Trilha")),
		),
	), nil
}

func escape(s string) string {
	r := []rune(s)
	var out []rune
	for _, ch := range r {
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
