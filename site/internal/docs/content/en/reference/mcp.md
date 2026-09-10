---
title: mcp
description: Model Context Protocol client and server (stdio and Streamable HTTP).
---

`import "github.com/emersonjoe/trilha/ai/mcp"` — JSON-RPC 2.0, revision `2025-03-26`, no
external dependencies. Covers the *tools* capability (list and call).

## trilha mcp — the project as tools

An agent with a shell does not need this: `trilha ctx --json` and `trilha check --json` are
already the answer. This is for the agent that has no shell — a chat client, an editor that
speaks MCP and nothing else — which otherwise has to guess what your project contains.

```bash
cd my-project
trilha mcp
```

It speaks MCP over stdin/stdout and answers about **the project it was started in**. Point
your client at it:

```json
{
  "mcpServers": {
    "trilha": { "command": "trilha", "args": ["mcp"], "cwd": "/path/to/my-project" }
  }
}
```

That shape works in Claude Desktop, Claude Code and any client that launches a stdio server.
`cwd` is what decides the project, and nothing the model sends can change it.

### The tools

| Tool | What it answers | Behind it |
|---|---|---|
| `describe_project` | routes, API, types and setup, as JSON | `trilha ctx --json` |
| `routes` | the route table in precedence order | `trilha routes` |
| `ui_describe` | the `ui` catalogue: signature, purpose, an example that compiles | the catalogue in the binary — no project needed |
| `check` | gen, `gofmt`, `vet`, tests, audit and OpenAPI, as JSON | `trilha check --json` |
| `generate` | writes a page, route, test or component | `trilha generate` — **only with `--write`** |

Each one is a wrapper: the tool answers what the command answers, byte for byte, and a test
holds it to that. There is no second implementation to drift.

### What it will not do

**It is read-only unless you say otherwise.** Without `--write`, the tool that writes files is
not registered — it is not in `tools/list`, so a model cannot ask for it and there is no
refusal to argue with. Start it with `trilha mcp --write` when you want the agent to scaffold,
and the tool says *Writes files* in the first line of its own description, which is what the
person approving the call reads.

**It never uses a shell.** Every command runs as a program plus an argument slice. A value from
the model is never concatenated into a command line.

**It checks arguments against an allowlist**, not against a list of bad things. A route address
has to look like a route address; a component name has to be an identifier; a method has to be
one of the six. `/../../etc/passwd`, `/x; rm -rf /` and `--force` are refused with the reason,
before anything runs.

**It cannot leave the project.** The working directory is decided when the server starts.

**One command at a time**, each with a deadline (10 minutes for `check`, which runs your test
suite; one minute for the rest) and a cap of 1 MiB of output, so a tool call cannot become a
fork bomb or eat memory.

**It opens no network connection**, and it writes nothing to stdout except the protocol —
every log line goes to stderr, where you can watch what the agent asked for:

```
trilha mcp 0.43.0 · /path/to/my-project · read-only unless --write; no shell; arguments allow-listed
tools: describe_project, check, routes, ui_describe
→ trilha ctx --json
```

### trilha mcp --from-routes

The other thing the CLI answers is *what would my API look like as tools*: the list
[`FromRoutes`](#your-api-as-tools) would publish, derived from `app/` and the OpenAPI document,
printed before any server exists.

```text
$ trilha mcp --from-routes
tools mcp.FromRoutes would expose (include: /api/*):
  deleteApiPostsId   DELETE /api/posts/{id}   DELETE removes a post.
  getApiPosts        GET /api/posts           GET lists posts.
  getApiPostsId      GET /api/posts/{id}      GET returns one post by slug.
  postApiPosts       POST /api/posts          POST creates a post from JSON.
```

`--include` takes the same patterns as `FromRoutesOpts.Include`. A route left out — a
multipart upload, for one — is printed with the reason, the same line the server logs.

### What is not here yet

Recipes and reference pages are not tools of this server: they are the same Markdown the
documentation site is built from, about a megabyte of it, and putting that in the CLI binary
to answer questions about a project it is not part of is the wrong trade. That is the job of a
hosted documentation server, which is still
[#50](https://github.com/emersonjoe/trilha/issues/50).

## Client

```go
func Dial(ctx, dial Dialer) (*Client, error)     // opens the transport and runs initialize
func Stdio(name string, args ...string) Dialer    // child process, JSON per line
func HTTP(url string, headers map[string]string) Dialer  // Streamable HTTP (POST per message)
```

| Method | Role |
|---|---|
| `ListTools(ctx) ([]ToolInfo, error)` | follows pagination (`nextCursor`) |
| `CallTool(ctx, name, args) (CallResult, error)` | `CallResult.Text()` joins the text items |
| `Tools(ctx) ([]*ai.Tool, error)` | tools ready for an `ai.Agent`; `isError` becomes an error |
| `Server.Name/Version/ProtocolVersion` | filled by `initialize` |
| `Close()` | closes the transport and ends the child process |

The HTTP client keeps the `Mcp-Session-Id` received on `initialize` and sends it on the
following messages; it accepts JSON or `text/event-stream` responses.

## Server

```go
func NewServer(name, version string, tools ...*ai.Tool) *Server
func (s *Server) ServeHTTP(c *trilha.Ctx) error         // in app/.../route.go: POST
func (s *Server) Handler() http.Handler                  // outside Trilha
func (s *Server) ServeStdio(ctx, r io.Reader, w io.Writer) error
```

Methods served: `initialize`, `ping`, `tools/list`, `tools/call`; notifications are accepted
without a response (`202`). Over HTTP, `initialize` emits `Mcp-Session-Id`; messages
without a valid session get `404`; sessions expire after `SessionTTL` (1 h) without use.
Only `POST` is accepted (`405` with `Allow: POST` for the rest). Body limited to 4 MiB.

Tool errors and panics become a result with `isError: true`, as the protocol requires; an
unknown tool is JSON-RPC error `-32602`.

## Your API as tools

```go
func FromRoutes(app *trilha.App, o FromRoutesOpts) *Server
func Preview(openAPI []byte, routes map[string][]string, o FromRoutesOpts) ([]ToolInfo, []string, error)

type FromRoutesOpts struct {
	Name, Version string   // what initialize answers
	Include       []string // patterns to expose; default /api/*
	Exclude       []string // patterns to hide, checked after Include
	OpenAPI       []byte   // the document trilha openapi writes; required
}
```

An API written under `app/api/` is already what an agent needs: a name, a description, an
argument schema, a handler. `FromRoutes` publishes it as MCP tools without a second
declaration — one tool per (method, route), named by the operation id of the document
(`getApiPosts`, `postApiPosts`), described by the handler's own doc comment, with the path
parameters, the query and the body flattened into one input schema.

```go
//go:embed mcp/openapi.json
var openAPI []byte

func Setup(a *trilha.App) error {
	trilha.Provide(a, mcp.FromRoutes(a, mcp.FromRoutesOpts{Name: "blog", Version: "1.0", OpenAPI: openAPI}))
	return nil
}
```

```go
// app/mcp/route.go
func POST(c *trilha.Ctx) error { return trilha.Use[*mcp.Server](c).ServeHTTP(c) }
```

The document is what gives the tools their descriptions and schemas — the runtime does not
keep them — so it lives next to the route, at `app/mcp/openapi.json`, written by
`trilha openapi -o app/mcp/openapi.json` and embedded. `trilha check` compares every
`openapi.json` under `app/` with the routes, so a copy that fell behind fails the build, not
the agent. Without a document `FromRoutes` panics: a tool with no description is a tool a
model will misuse.

**A call is the route.** The tool builds the request — `{id}` into the path, `q` into the
query, the rest as the JSON body — and runs it through `app.Handler()`, chain included: the
rate limit, the API key, the audit and the log see the same request a `curl` would send. The
caller's `Authorization`, `X-Forwarded-For`, `X-Request-ID` and `Accept-Language` travel with
it; the actor of the audit record says `Via: "mcp"`. A `4xx`/`5xx` becomes a tool result with
`isError: true` and the body as the text, so the model reads the `422` the handler wrote.

**`tools/list` is per caller.** Before listing a tool the server *probes* its route with the
caller's headers — the chain runs, the handler does not — and a route that would answer `401`
or `403` is not in the list. A key with `docs:read` sees the reads; the writes are not hidden
behind a refusal, they are absent. The probe takes no rate-limit token and counts no usage
(see [`App.Probe`](/reference/app#probing-a-route)).

**What is left out**, with the reason in the log and in `trilha mcp --from-routes`: pages
(`page.go`), routes outside `Include`, `OPTIONS`/`HEAD`, and a handler whose body is
multipart — the bridge sends JSON. `Include` and `Exclude` are prefixes with a trailing `*`
or exact paths.

`Preview` is the same plan without an app: the document, the routes and the options in, the
list and the warnings out — what the CLI prints.

## Your own transport

`Transport` is an interface (`Send`, `Recv`, `Close`). `Pipe(r, w, closer)` builds the line
transport over any reader/writer pair, which the tests use with `io.Pipe`.
