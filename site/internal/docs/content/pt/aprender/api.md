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
		return trilha.ErrNotFound // 404 problem+json
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
| `trilha.Errorf(422, "msg")` | 422 com `"detail":"msg"` |
| `c.Redirect(url)` | 303 |
| qualquer outro `error` | 500 com `"title":"Internal Server Error"`; a mensagem real vai para o log |
| `&trilha.Problem{…}` | exatamente o problema que você descreveu |

O corpo é *problem details*, do [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457), enviado
como `application/problem+json` — o formato que cliente gerado, gateway e teste de contrato
já sabem ler:

```json
{"type":"about:blank","title":"Not Found","status":404,
 "instance":"/api/eventos/nao-existe","request_id":"01J…"}
```

O `fields` do 422 continua igual, então o formulário que o lê não muda. E quando o status não
basta, descreva o problema:

```go
return &trilha.Problem{
	Type:   "https://exemplo.com/probs/esgotado",
	Title:  "Ingressos esgotados",
	Status: http.StatusConflict,
	Detail: "O último lugar saiu há 4 minutos.",
	Extra:  map[string]any{"espera": "/api/eventos/" + ev.Slug + "/espera"},
}
```

Qual formato sai segue o tipo da rota, com o `Accept` como desempate: um `route.go` responde
`problem+json`, a não ser que o cliente prefira `text/html` — o navegador na barra de
endereço recebe a página de erro, esteja a rota onde estiver. Veja
[Erros](/pt/referencia/erros).

## Documento OpenAPI

`trilha openapi` escreve o documento OpenAPI 3.1 das suas rotas de API. Não há anotação para
manter em dia: a fonte do documento é o código que responde à requisição.

```bash
trilha openapi                    # escreve openapi.json
trilha openapi -o - | jq .paths   # na saída padrão
trilha openapi --check            # no CI: falha quando o arquivo se descolou do código
```

O que ele lê sozinho:

| No código | No documento |
|---|---|
| a pasta dentro de `app/api/` | o caminho, com `id_` virando parâmetro |
| `GET`, `POST`, `PUT`, `PATCH`, `DELETE` exportados | uma operação para cada |
| o comentário de doc | `summary` (primeira frase) e `description` |
| `c.Bind(&in)` / `c.BindJSON(&in)` | `requestBody` com o schema de `in`, mais um 422 |
| `c.JSON(status, v)` | aquele status com o schema de `v` |
| `c.Writer().WriteHeader(204)` | aquele status sem corpo |
| `c.Header("Content-Type", …)` | o media type da resposta |
| `trilha.ErrNotFound`, `trilha.Errorf(status, …)`, `&trilha.Problem{Status: …}` | aquele status em `problem+json` |
| tags `json` e `validate` | nomes das propriedades, `required`, `maxLength`, `enum`, `format` |

O schema sai da mesma tag `validate` que o `Bind` lê, então o documento não promete o que a
validação recusa. Toda operação leva também a resposta `default` com o schema
[`Problem`](/pt/referencia/erros): desde a 0.21.0 é essa a forma de todo erro de API.

Só rotas de `route.go` entram. Uma página responde HTML para um navegador; ali não há
contrato que um cliente possa cobrar de você.

### Quando a dedução não alcança

Um middleware, um `c.Query` ou uma pasta com ponto no nome estão fora do que a leitura do
handler conta. Escreva no comentário de doc:

```go
// GET escreve o mês em CSV.
//
// openapi:query mes string  mês a exportar, AAAA-MM (padrão: o atual)
// openapi:response 429
// openapi:tag relatorio
func GET(c *trilha.Ctx) error { … }
```

| Diretiva | O que faz |
|---|---|
| `openapi:response <status> [tipo]` | acrescenta a resposta; sem tipo, `problem+json` |
| `openapi:body <tipo>` | o corpo da requisição, quando não vem de um `Bind` |
| `openapi:query <nome> <tipo> [descrição]` | um parâmetro de query |
| `openapi:tag <nome>` | a tag da operação (padrão: o último segmento fixo do caminho) |

Um tipo que ninguém declarou é erro apontando arquivo e handler, não schema vazio publicado
como se estivesse certo.

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
