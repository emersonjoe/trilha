---
title: Middleware
description: Interceptar uma subárvore de rotas, passar valores para as páginas e proteger áreas.
---

Um `middleware.go` roda antes de qualquer rota da sua pasta e das pastas abaixo. O da raiz
roda em toda requisição; o de um grupo, só nas rotas do grupo.

## A assinatura

```go
func Middleware(c *trilha.Ctx, next trilha.Next) error
```

Chame `next()` para seguir. Não chame para interromper. Devolva um erro para que o
tratamento padrão responda (redirecionamento, 404, 500).

## Medir tempo em todas as rotas

`app/middleware.go`:

```go
package app

import (
	"time"

	"github.com/emersonjoe/trilha"
)

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	inicio := time.Now()
	err := next()
	c.Header("Server-Timing", "app;dur="+time.Since(inicio).String())
	return err
}
```

O cabeçalho é escrito depois de `next()` mas antes de a resposta ser enviada, porque
páginas são renderizadas em memória e escritas de uma vez.

## Proteger a área do organizador

Um grupo de rota é o lugar natural para exigir login sem poluir a URL:

```text
app/organizador-/middleware.go
app/organizador-/painel/page.go     → /painel
app/organizador-/relatorio/page.go  → /relatorio
```

```go
package organizador

import "github.com/emersonjoe/trilha"

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	ck, err := c.Cookie("sessao")
	if err != nil || !sessao.Valida(ck.Value) {
		return trilha.RedirectCode("/entrar?next="+c.Request().URL.Path, 302)
	}
	c.Set("usuario", sessao.Usuario(ck.Value))
	return next()
}
```

Na página, `c.Get("usuario")` devolve o valor. Valores vivem só durante a requisição.

## Uma regra para um método só

Uma pasta costuma ter dois papéis: um `GET` que qualquer um da área lê e um `POST` que só um
editor manda. Colocar a permissão na primeira linha do handler funciona — até a décima
primeira rota, onde alguém esquece. O `middleware.go` aceita o método no nome:

```go
package organizador

import (
	"net/http"

	"github.com/emersonjoe/trilha"
)

// Quem chegou até aqui lê.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	c.Set("area", "organizador")
	return next()
}

// Só editor escreve, nesta pasta e nas de baixo.
func MiddlewarePOST(c *trilha.Ctx, next trilha.Next) error {
	if c.Get("papel") != "editor" {
		return trilha.Errorf(http.StatusForbidden, "só editores mudam a meta")
	}
	return next()
}
```

Valem `MiddlewareGET`, `MiddlewarePOST`, `MiddlewarePUT`, `MiddlewarePATCH` e
`MiddlewareDELETE`. Eles herdam pela subárvore igual ao `Middleware`, e rodam por dentro
dele — a rota decide primeiro, o método refina depois. Um `MiddlewareX` que não alcança
nenhuma rota com aquele método é erro de geração (`E_UNUSED_METHOD_MIDDLEWARE`): uma
permissão que não guarda nada é exatamente a falha que esta convenção existe para evitar.

O 403 acima sai pelo `app/error.go`, com o layout do app; veja [Erros](/pt/referencia/erros).

## Ordem

Para `GET /painel`:

```text
middleware(app) → middleware(app/organizador-)
  → middlewareGET(app) → middlewareGET(app/organizador-)
  → Page → layouts
```

De fora para dentro, a cadeia da rota antes da cadeia do método. Se um middleware não chamar `next()`, os de dentro e a página não rodam,
mas os de fora terminam normalmente (o de medição acima ainda escreve o cabeçalho).

## Curto-circuito com resposta própria

Um middleware pode responder diretamente e devolver `nil`:

```go
if c.Request().Header.Get("X-Manutencao") == "1" {
	return c.Text(503, "em manutenção")
}
```

Como a resposta já começou, o Trilha não tenta escrever outra.

## Desafio

Crie `app/api/middleware.go` que exija o cabeçalho `Authorization: Bearer <chave>` em toda a
API e responda 401 em JSON quando faltar, sem afetar as páginas HTML.

:::solucao
```go
package api

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
)

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || !chaves.Valida(strings.TrimPrefix(auth, "Bearer ")) {
		return trilha.Errorf(http.StatusUnauthorized, "chave inválida")
	}
	return next()
}
```

Como a pasta é `app/api/`, só as rotas de API passam por ele, e o erro sai em JSON porque a
rota é `route.go`.
:::
