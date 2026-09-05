---
title: Rotas de API
description: route.go com uma função por método HTTP, JSON de entrada e saída e erros com status.
---

Uma pasta com `route.go` responde JSON em vez de HTML. Cada método HTTP é uma função
exportada com o mesmo formato de sempre: `func(c *trilha.Ctx) error`.

## Listar e criar

`app/api/eventos/route.go`:

```go
package eventos

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"agenda/internal/eventos"
)

func GET(c *trilha.Ctx) error {
	return c.JSON(http.StatusOK, eventos.Todos())
}

func POST(c *trilha.Ctx) error {
	var in struct {
		Nome   string `json:"nome"`
		Cidade string `json:"cidade"`
	}
	if err := c.BindJSON(&in); err != nil {
		return err // 400 em JSON inválido, 413 acima do limite
	}
	if strings.TrimSpace(in.Nome) == "" {
		return trilha.Errorf(http.StatusUnprocessableEntity, "nome é obrigatório")
	}
	ev := eventos.Criar(in.Nome, in.Cidade)
	c.Header("Location", "/api/eventos/"+ev.Slug)
	return c.JSON(http.StatusCreated, ev)
}
```

```bash
curl -s localhost:3000/api/eventos
curl -s -X POST localhost:3000/api/eventos -d '{"nome":"Oficina de HTTP","cidade":"Recife"}'
curl -s -X PUT localhost:3000/api/eventos     # 405 com Allow: GET, POST
```

## Um recurso por slug

`app/api/eventos/slug_/route.go` responde `/api/eventos/{slug}`:

```go
func GET(c *trilha.Ctx) error {
	ev, ok := eventos.Buscar(c.Param("slug"))
	if !ok {
		return trilha.ErrNotFound // {"error":"Not Found","status":404}
	}
	return c.JSON(200, ev)
}

func DELETE(c *trilha.Ctx) error {
	if !eventos.Apagar(c.Param("slug")) {
		return trilha.ErrNotFound
	}
	c.Writer().WriteHeader(http.StatusNoContent)
	return nil
}
```

## Erros viram status

| Você devolve | Resposta |
|---|---|
| `nil` | o que você escreveu; 204 se não escreveu nada |
| `trilha.ErrNotFound` | 404 em JSON |
| `trilha.Errorf(422, "msg")` | 422 com `{"error":"msg"}` |
| `c.Redirect(url)` | 303 |
| qualquer outro `error` | 500 com `{"error":"Internal Server Error"}`; a mensagem real vai para o log |

Em rotas de API os erros saem em JSON; em páginas, em HTML. O formato segue o tipo de rota,
não o cabeçalho `Accept`.

## CSRF em APIs

Por padrão, `route.go` **não** exige token CSRF: APIs costumam ser chamadas com token de
sessão ou bearer, e o cookie `SameSite=Lax` já protege contra envio automático pelo
navegador. Se a sua API for chamada pelo próprio site com cookies, ligue
`Config.CSRFForAPI` e mande `X-CSRF-Token`.

## Desafio

Adicione `PATCH` em `/api/eventos/{slug}` que atualize só os campos presentes no JSON e
responda 200 com o evento novo. JSON com campos desconhecidos deve dar 400.

:::solucao
`c.BindJSON` já rejeita campos desconhecidos. Para "só os campos presentes", use ponteiros:

```go
func PATCH(c *trilha.Ctx) error {
	var in struct {
		Nome   *string `json:"nome"`
		Cidade *string `json:"cidade"`
	}
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	ev, ok := eventos.Buscar(c.Param("slug"))
	if !ok {
		return trilha.ErrNotFound
	}
	if in.Nome != nil {
		ev.Nome = *in.Nome
	}
	if in.Cidade != nil {
		ev.Cidade = *in.Cidade
	}
	eventos.Salvar(ev)
	return c.JSON(200, ev)
}
```
:::
