---
title: Your API as agent tools
description: Publishing the routes under /api/ as MCP tools, with the key deciding what each agent sees.
---

An agent that speaks MCP — Claude Desktop, Claude Code, an editor — needs three things from
your API: what the operations are, what each one takes, and a way to call it with its own
credential. A `route.go` under `app/api/` already answers all three; this recipe is the four
files that let the agent read the answer.

`examples/blog` is this recipe, whole and running: `GET /api/posts` is `getApiPosts`.

## 1. The document, next to the route

The runtime keeps routes and handlers, not the doc comments and the argument types that
make a tool usable. Those are in the OpenAPI document `trilha openapi` deduces from the
source — so the document is generated into the package that will embed it:

```bash
trilha openapi -o app/mcp/openapi.json
```

It is committed, like `trilha_gen.go`, and `trilha check` fails the day a route changes and
nobody regenerated it — the same gate the root `openapi.json` already has.

## 2. The server, built from the routes

In `app/setup.go`, the document is embedded from the package next door:

```go
// openAPI is the document `trilha openapi -o app/mcp/openapi.json` writes.
// It is what gives the MCP tools below their descriptions and argument
// schemas; `trilha check` fails when it falls behind the routes.
//
//go:embed mcp/openapi.json
var openAPI []byte
```

and `Setup` files the server as a dependency, like the store:

```go
	// A /api também é um conjunto de ferramentas MCP, em /mcp: as mesmas
	// rotas, o mesmo limite, o mesmo JSON. Um agente que só fala o protocolo
	// lista e cria posts sem que ninguém escreva uma segunda API para ele.
	trilha.Provide(a, mcp.FromRoutes(a, mcp.FromRoutesOpts{Name: "blog", Version: "1.0", OpenAPI: openAPI}))
```

`Setup` runs before the routes are registered, and that is fine: the tool table is built on
the first message, when they exist. Without `Include`, everything under `/api/` that is an
API — pages are never tools — goes in; `Include: []string{"/api/v1/*"}` narrows it, and
`Exclude` removes a route the agent should not know about.

## 3. The route

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

That is `POST /mcp`, Streamable HTTP, with the session id the protocol requires. Point the
client at it:

```json
{
  "mcpServers": {
    "blog": { "url": "https://blog.example.com/mcp", "headers": { "Authorization": "Bearer ak_…" } }
  }
}
```

## 4. The key decides the list

There is no fourth file. The tools go through the route's own chain — the rate limit in
`app/api/middleware.go`, the `Keys.Require("docs:write")` on the `POST` — so the header the
client sends is the header the route checks, and the audit record of a post created this way
says `Via: "mcp"` next to the key's subject.

Before listing, the server probes each route with the caller's headers. A key with
`docs:read` gets `getApiPosts` and `getApiPostsId`; `postApiPosts` is not a tool it can see,
not a tool that refuses. That is what keeps a model from planning around an operation it
will never be allowed to run.

```text
$ trilha mcp --from-routes
tools mcp.FromRoutes would expose (include: /api/*):
  deleteApiPostsId   DELETE /api/posts/{id}   DELETE removes a post.
  getApiPosts        GET /api/posts           GET lists posts.
  getApiPostsId      GET /api/posts/{id}      GET returns one post by slug.
  postApiPosts       POST /api/posts          POST creates a post from JSON.
```

## What a call looks like from inside

`postApiPosts` with `{"title": "Hello", "body": "…"}` is `POST /api/posts` with that JSON
body; `getApiPostsId` with `{"id": "hello"}` is `GET /api/posts/hello`. A query parameter the
document declares travels in the query; a path parameter in the path; the rest in the body.
The handler cannot tell the difference, and neither can the test you already have for it.

A `422` from `BindJSON` comes back as a tool result with `isError: true` and the handler's
own `{"fields": …}` as the text — which is what a model needs to fix its call, and what a
bare "error" would hide.

## What stays out

- Pages, and any route outside `Include`.
- `OPTIONS` and `HEAD`.
- A handler that reads `multipart/form-data`: the bridge sends JSON, and the log says which
  route was left out and why. Upload routes stay an HTTP call.

See [mcp](/reference/mcp#your-api-as-tools) for `FromRoutesOpts` and `Preview`, and
[App](/reference/app#probing-a-route) for `Probe`, the primitive the per-caller list is
built on.

When the agent is yours and lives in the app, the same route becomes a tool the same way —
one `ai.Tool` that probes and calls it, with no second declaration. That is step 1 of the
[AI agent recipe](/cookbook/ai-agent).
