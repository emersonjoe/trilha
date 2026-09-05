---
title: Início rápido
description: Do zero a uma página no navegador em cinco minutos, e o que aconteceu em cada passo.
---

Nesta trilha você constrói uma **agenda de eventos**: lista, página de detalhe, formulário
para cadastrar, uma API JSON e uma área restrita. Cada capítulo adiciona um pedaço e termina
com um desafio. Este primeiro só coloca o projeto de pé.

## O que você precisa

- Go 1.22 ou mais novo (`go version`).
- A pasta de binários do Go no `PATH`: `~/go/bin` (o Go instala programas ali, e não em
  `/usr/local/go/bin`, que é onde fica o próprio `go`).

```bash
# se `trilha` não for encontrado depois do go install, adicione ao seu ~/.zshrc ou ~/.bashrc:
export PATH="$HOME/go/bin:$PATH"
```

## Instale e crie o projeto

```bash
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha new agenda
cd agenda
trilha dev
```

Abra `http://localhost:3000`. A página inicial já está lá. Deixe o `trilha dev` rodando: ele
recompila e recarrega o navegador a cada arquivo salvo.

## O que foi criado

```text
agenda/
├── go.mod
├── trilha_gen.go          ← gerado pela CLI; commite, não edite
├── public/style.css       ← servido em /style.css
└── app/
    ├── layout.go          ← o <html> de todas as páginas
    ├── page.go            ← GET /
    ├── not_found.go       ← página 404
    └── api/hello/route.go ← GET /api/hello
```

A regra que sustenta tudo: **uma pasta dentro de `app/` é um caminho na URL**. O arquivo
dentro dela diz o que aquele caminho faz.

## Sua primeira página

Crie `app/eventos/page.go`:

```go
package eventos

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Eventos")
	return h.Fragment(
		h.H1(h.Text("Próximos eventos")),
		h.P(h.Text("Ainda não há eventos cadastrados.")),
	), nil
}
```

Salve e visite `/eventos`. Três coisas aconteceram:

1. A CLI viu a pasta nova, regenerou `trilha_gen.go` com a rota `/eventos` e recompilou.
2. `Page` rodou e devolveu um **nó** de HTML, construído com o pacote `h`.
3. O nó foi entregue ao `Layout` de `app/layout.go`, que colocou o `<html>` em volta, e o
   resultado foi enviado com `Content-Type: text/html`.

:::dica
`Page` recebe um único argumento, o `*trilha.Ctx`, e devolve `(h.Node, error)`. Toda função
de rota no Trilha segue este formato: um contexto de entrada, um erro de saída. Você vai ver
o mesmo desenho em layouts, middlewares e rotas de API.
:::

## Desafio

Crie `app/sobre/page.go` que responda `/sobre` com um título e um parágrafo, e adicione um
link para ela na navegação do `app/layout.go`. Quando salvar, a página deve aparecer sem
reiniciar nada.

:::solucao
```go
// app/sobre/page.go
package sobre

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Sobre")
	return h.Fragment(
		h.H1(h.Text("Sobre a agenda")),
		h.P(h.Text("Uma agenda de eventos construída com Trilha.")),
	), nil
}
```

No `app/layout.go`, dentro do `h.Nav(...)`:

```go
h.A(h.Href("/sobre"), h.Text("Sobre")),
```
:::
