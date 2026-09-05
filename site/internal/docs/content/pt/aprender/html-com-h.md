---
title: HTML com o pacote h
description: Elementos como funções, escape por padrão, condicionais, listas e quando usar templates.
---

O pacote `h` gera HTML sem arquivos de template: cada elemento é uma função Go que aceita
atributos e filhos em qualquer ordem. Tudo é verificado pelo compilador e escapado na saída.

## Elementos, atributos e texto

```go
h.Article(h.Class("evento", "destaque"),
	h.H2(h.Text(ev.Nome)),
	h.P(h.Textf("%s, %d vagas", ev.Cidade, ev.Vagas)),
	h.A(h.Href("/eventos/"+ev.Slug), h.Text("Detalhes")),
)
```

- `h.Text` e `h.Textf` escapam. `h.Raw` não escapa e é a única porta para HTML pronto.
- Atributos (`h.Class`, `h.Href`, `h.ID`, `h.Data("x", v)`, `h.Attr("nome", v)`) podem vir
  depois dos filhos; eles sempre acabam na tag de abertura.
- Elementos vazios (`h.Br`, `h.Img`, `h.Input`, `h.Meta`) não fecham.
- Atributos booleanos são funções sem argumento: `h.Required()`, `h.Disabled()`.
- Quando o nome colide com um elemento, o atributo ganha o sufixo `Attr`: `h.StyleAttr`,
  `h.TitleAttr`, `h.LabelAttr`.

@demo escape

## Condicionais e listas

```go
h.Ul(
	h.If(len(eventos) == 0, h.Li(h.Em(h.Text("nenhum evento")))),
	h.Map(eventos, func(ev Evento) h.Node {
		return h.Li(h.Text(ev.Nome))
	}),
)
```

`h.If` devolve um nó vazio quando a condição é falsa; `h.IfElse` escolhe entre dois;
`h.Map` aplica uma função a cada item; `h.Fragment` agrupa vários nós sem elemento em volta.
`nil` como filho é ignorado, então um `func() h.Node` que devolve `nil` também é seguro.

@demo lista

## Componentes são funções

Não existe um tipo "componente". Uma função que devolve `h.Node` já é um:

```go
func CartaoEvento(ev Evento) h.Node {
	return h.Article(h.Class("cartao"),
		h.H3(h.Text(ev.Nome)),
		h.P(h.Text(ev.Cidade)),
	)
}

// na página
h.Div(h.Class("grade"), h.Map(eventos, CartaoEvento))
```

## Prefere templates?

O pacote `tmpl` encaixa `html/template` no mesmo pipeline. Os arquivos ficam ao lado da
página e são embutidos no binário:

```go
package relatorio

import (
	"embed"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/tmpl"
)

//go:embed relatorio.html
var arquivos embed.FS

var t = tmpl.Must(arquivos, "*.html") // falha na subida, nunca no request

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Relatório")
	return tmpl.Node(t, "relatorio", dados), nil
}
```

Layouts, título e erros funcionam igual. O escape é o contextual do próprio `html/template`.

## Desafio

Escreva um componente `Vagas(n int) h.Node` que mostre "lotado" em itálico quando `n == 0`,
"1 vaga" no singular e "N vagas" no plural, e use-o na lista de eventos.

:::solucao
```go
func Vagas(n int) h.Node {
	switch {
	case n == 0:
		return h.Em(h.Text("lotado"))
	case n == 1:
		return h.Text("1 vaga")
	default:
		return h.Textf("%d vagas", n)
	}
}
```
:::
