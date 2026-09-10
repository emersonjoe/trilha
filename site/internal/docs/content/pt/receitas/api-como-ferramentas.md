---
title: A sua API como ferramentas de agente
description: Publicar as rotas sob /api/ como ferramentas MCP, com a chave decidindo o que cada agente vê.
---

Um agente que fala MCP — Claude Desktop, Claude Code, um editor — precisa de três coisas da
sua API: quais são as operações, o que cada uma recebe e um jeito de chamá-la com a própria
credencial. Um `route.go` em `app/api/` já responde as três; esta receita são os quatro
arquivos que deixam o agente ler a resposta.

`examples/blog` é esta receita, inteira e rodando: `GET /api/posts` é `getApiPosts`.

## 1. O documento, ao lado da rota

O runtime guarda rotas e handlers, não os comentários e os tipos de argumento que tornam uma
ferramenta usável. Esses estão no documento OpenAPI que o `trilha openapi` deduz da fonte —
então o documento é gerado dentro do pacote que vai embuti-lo:

```bash
trilha openapi -o app/mcp/openapi.json
```

Ele é commitado, como o `trilha_gen.go`, e o `trilha check` falha no dia em que uma rota muda
e ninguém regenerou — o mesmo portão que o `openapi.json` da raiz já tem.

## 2. O servidor, montado das rotas

Em `app/setup.go`, o documento é embutido do pacote vizinho:

```go
// openAPI is the document `trilha openapi -o app/mcp/openapi.json` writes.
// It is what gives the MCP tools below their descriptions and argument
// schemas; `trilha check` fails when it falls behind the routes.
//
//go:embed mcp/openapi.json
var openAPI []byte
```

e o `Setup` registra o servidor como dependência, como o store:

```go
	// A /api também é um conjunto de ferramentas MCP, em /mcp: as mesmas
	// rotas, o mesmo limite, o mesmo JSON. Um agente que só fala o protocolo
	// lista e cria posts sem que ninguém escreva uma segunda API para ele.
	trilha.Provide(a, mcp.FromRoutes(a, mcp.FromRoutesOpts{Name: "blog", Version: "1.0", OpenAPI: openAPI}))
```

O `Setup` roda antes de as rotas serem registradas, e não tem problema: a tabela de
ferramentas é montada na primeira mensagem, quando elas existem. Sem `Include`, entra tudo
sob `/api/` que é API — página nunca vira ferramenta; `Include: []string{"/api/v1/*"}`
estreita, e `Exclude` tira uma rota que o agente não deve conhecer.

## 3. A rota

```go
// Package mcp serves the /api of this blog as MCP tools, so an agent that
// speaks the protocol and nothing else can list and create posts.
package mcp

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai/mcp"
)

// POST is the Streamable HTTP endpoint. The server is built once in
// app/setup.go from the routes of this app and the OpenAPI document next to
// this file; here it only answers.
func POST(c *trilha.Ctx) error {
	return trilha.Use[*mcp.Server](c).ServeHTTP(c)
}
```

Isso é `POST /mcp`, Streamable HTTP, com o id de sessão que o protocolo exige. Aponte o
cliente para lá:

```json
{
  "mcpServers": {
    "blog": { "url": "https://blog.exemplo.com/mcp", "headers": { "Authorization": "Bearer ak_…" } }
  }
}
```

## 4. A chave decide a lista

Não há quarto arquivo. As ferramentas passam pela cadeia da própria rota — o limite de taxa
em `app/api/middleware.go`, o `Keys.Require("docs:write")` do `POST` —, então o cabeçalho
que o cliente manda é o cabeçalho que a rota confere, e o registro de auditoria de um post
criado assim diz `Via: "mcp"` ao lado do sujeito da chave.

Antes de listar, o servidor sonda cada rota com os cabeçalhos de quem chamou. Uma chave com
`docs:read` recebe `getApiPosts` e `getApiPostsId`; `postApiPosts` não é uma ferramenta que
ela vê, não é uma ferramenta que recusa. É isso que impede um modelo de planejar em cima de
uma operação que nunca vai poder rodar.

```text
$ trilha mcp --from-routes
ferramentas que mcp.FromRoutes exporia (include: /api/*):
  deleteApiPostsId   DELETE /api/posts/{id}   DELETE removes a post.
  getApiPosts        GET /api/posts           GET lists posts.
  getApiPostsId      GET /api/posts/{id}      GET returns one post by slug.
  postApiPosts       POST /api/posts          POST creates a post from JSON.
```

## Como é uma chamada por dentro

`postApiPosts` com `{"title": "Olá", "body": "…"}` é `POST /api/posts` com esse corpo JSON;
`getApiPostsId` com `{"id": "ola"}` é `GET /api/posts/ola`. Parâmetro de query que o
documento declara viaja na query; parâmetro de caminho, no caminho; o resto, no corpo. O
handler não nota a diferença — nem o teste que você já tem para ele.

Um `422` do `BindJSON` volta como resultado com `isError: true` e o `{"fields": …}` do
próprio handler como texto — que é o que um modelo precisa para corrigir a chamada, e o que
um "erro" seco esconderia.

## O que fica de fora

- Páginas, e qualquer rota fora do `Include`.
- `OPTIONS` e `HEAD`.
- Handler que lê `multipart/form-data`: a ponte manda JSON, e o log diz qual rota ficou de
  fora e por quê. Rotas de upload continuam sendo uma chamada HTTP.

Veja [mcp](/pt/referencia/mcp#a-sua-api-como-ferramentas) para `FromRoutesOpts` e `Preview`, e
[App](/pt/referencia/app#sondar-uma-rota) para o `Probe`, a primitiva sobre a qual a lista
por chamador é montada.
