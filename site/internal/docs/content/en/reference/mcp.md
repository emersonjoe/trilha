---
title: mcp
description: Model Context Protocol client and server (stdio and Streamable HTTP).
---

`import "github.com/emersonjoe/trilha/ai/mcp"` — JSON-RPC 2.0, revision `2025-03-26`, no
external dependencies. Covers the *tools* capability (list and call).

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

## Your own transport

`Transport` is an interface (`Send`, `Recv`, `Close`). `Pipe(r, w, closer)` builds the line
transport over any reader/writer pair, which the tests use with `io.Pipe`.
