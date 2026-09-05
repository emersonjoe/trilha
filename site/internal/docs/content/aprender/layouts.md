---
title: Layouts aninhados
description: Um layout por pasta, do mais interno ao mais externo, e como o título viaja entre eles.
---

Um `layout.go` envolve todas as páginas da sua pasta e das pastas abaixo. O layout de
`app/` é o raiz e normalmente é o único que escreve `<html>`.

## A assinatura

```go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error)
```

`children` é a página já renderizada como nó, ou o layout mais interno já aplicado. Você
decide onde colocá-la.

## Um layout para a agenda

Crie `app/eventos/layout.go`:

```go
package eventos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("agenda"),
		h.Nav(
			h.A(h.Href("/eventos"), h.Text("Todos")),
			h.A(h.Href("/eventos/novo"), h.Text("Novo evento")),
		),
		children,
	), nil
}
```

Agora `/eventos`, `/eventos/novo` e `/eventos/qualquer` aparecem dentro dessa `<section>`,
que por sua vez aparece dentro do `<main>` do layout raiz.

@demo layout

## A ordem de execução

Para `GET /eventos/encontro-go`:

1. `app/eventos/slug_/page.go` → `Page` produz o nó da página.
2. `app/eventos/layout.go` → recebe esse nó como `children`.
3. `app/layout.go` → recebe o resultado do passo 2.

De dentro para fora. Uma pasta sem `layout.go` simplesmente não participa.

## Título e outros dados da página para o layout

A página roda **antes** dos layouts. Por isso `c.SetTitle("Eventos")` na página funciona no
layout raiz, que lê `c.Title()` para montar o `<title>`. O mesmo vale para qualquer valor
que você guardar com `c.Set(chave, valor)` e ler com `c.Get(chave)`.

```go
// na página
c.SetTitle("Encontro Go")
c.Set("descricao", "Uma noite de palestras em Campinas")

// no layout raiz
h.Title(h.Text(c.Title())),
h.Meta(h.Name("description"), h.Content(str(c.Get("descricao")))),
```

:::dica
Se não existir `app/layout.go`, o Trilha embrulha a página em um `<html>` mínimo. Útil nos
primeiros minutos; crie o seu assim que quiser CSS.
:::

## Layouts em grupos de rota

Um grupo (`organizador-/`) pode ter layout. Ele vale para as páginas do grupo e conta como
um nível na ordem: `página → layout do grupo → layout raiz`.

## Desafio

Faça o layout de `app/eventos/` mostrar, abaixo da navegação, um `<p>` com o título da
página atual, para confirmar que o título definido em `Page` já está disponível ali.

:::solucao
```go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Section(h.Class("agenda"),
		h.Nav(
			h.A(h.Href("/eventos"), h.Text("Todos")),
			h.A(h.Href("/eventos/novo"), h.Text("Novo evento")),
		),
		h.P(h.Class("migalha"), h.Text(c.Title())),
		children,
	), nil
}
```
:::
