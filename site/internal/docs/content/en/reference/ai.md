---
title: ai
description: OpenAI-compatible client, tools, agents, handoffs and composition.
---

`import "github.com/emersonjoe/trilha/ai"` — no external dependencies.

## Client

| Field / function | Role |
|---|---|
| `NewFromEnv() *Client` | reads `OPENAI_API_KEY`, `OPENAI_BASE_URL` (default `https://api.openai.com/v1`) and `TRILHA_AI_MODEL` (or `OPENAI_MODEL`; default `gpt-4o-mini`) |
| `BaseURL, APIKey, Model string` | direct configuration |
| `Headers map[string]string` | extra headers (OpenRouter, Azure...) |
| `HTTPClient *http.Client` | HTTP client (default with a 2 min timeout) |
| `Chat(ctx, Request) (*Response, error)` | one call; `Response.Text()` and `Response.ToolCalls()` |
| `Stream(ctx, Request, func(Delta) error) error` | chunked response; `Delta.Content`, `Delta.ToolCalls`, `Delta.Usage` at the end |

Non-2xx responses become `*ai.Error{Status, Code, Message}`.

## Request and messages

`Request{Model, Messages, Tools, ToolChoice, Temperature, MaxTokens, ResponseFormat, Extra}`.
`Extra map[string]any` is merged into the JSON sent, for provider-specific parameters.
`ResponseFormat{Type: "json_schema", JSONSchema: ...}` asks for structured output.

Constructors: `ai.System(s)`, `ai.User(s)`, `ai.Assistant(s)`, `ai.ToolResult(callID, s)`.

## Tool

```go
func NewTool(name, description string, schema json.RawMessage, fn ToolFunc) *Tool
type ToolFunc func(ctx context.Context, args json.RawMessage) (string, error)
func Schema(s string) json.RawMessage           // validates the JSON; panics at startup if invalid
func Typed[T any](fn func(ctx, in T) (string, error)) ToolFunc
```

`schema == nil` means "no arguments". Errors and panics from the function become text for
the model (`error: ...`) and show up in `Step.Err`.

## Agent

| Field | Role |
|---|---|
| `Name` | identifies the agent in `Step.Agent` and in handoffs (`transfer_to_<slug>`) |
| `Instructions` | `system` message |
| `Model` | overrides the client's model |
| `Tools []*Tool` | tools |
| `Handoffs []*Agent` | agents this one may transfer the conversation to |
| `MaxTurns` | limit of model calls per `Run` (default 10; exceeded → `ErrMaxTurns`) |
| `Temperature *float64`, `ResponseFormat` | passed on every request |

```go
func Run(ctx, cli *Client, agent *Agent, input string, history ...Message) (*Result, error)
func RunStream(ctx, cli *Client, agent *Agent, input string, fn func(Event), history ...Message) (*Result, error)
```

`Result{Output, Agent, Messages, Steps, Usage, Turns}`. `Messages` serves as history for the
next call (`system` messages from the history are ignored; the current agent's apply).

`Event.Type`: `text` (`Text`), `tool_call` and `tool_result` (`Step`), `handoff`
(`Step.HandoffTo`, `Agent` = new agent), `done` (`Result`), `error` (`Err`).

Tools of the same round run in parallel; the order of results in the history is the order
the model asked for them. A handoff swaps the `system` message, keeps the history and
continues the loop with the target agent.

## Composition

```go
func (a *Agent) AsTool(cli *Client, description string) *Tool   // {"input": "..."} → text
func Parallel(ctx, cli, input string, agents ...*Agent) ([]*Result, error)
func Chain(ctx, cli, input string, agents ...*Agent) (*Result, error)
```

`Parallel` returns in the agents' order and propagates the first error; `Chain` passes one's
`Output` as the next one's `input`.

## Chat over HTTP

```go
func Serve(c *trilha.Ctx, cli *Client, agent *Agent) error
func (o ServeOpts) Serve(c *trilha.Ctx, cli *Client, agent *Agent) error
```

`Serve` is the route half of [`ui.Chat`](/reference/ui#chat): it reads the message, runs the
agent and answers the request.

```go
// app/api/chat/route.go
func POST(c *trilha.Ctx) error {
	return ai.ServeOpts{HTML: ui.ChatHTML}.Serve(c, client, assistant)
}
```

The request carries `{"message": "…", "history": [...]}` as JSON, or a `message` form field.
A `{"messages": [...]}` list is also read: the last `user` turn is the message and everything
before it is the history.

When the client asks for `Accept: text/event-stream`, the answer is a stream of named events —
the [`Stream`](/reference/ctx) of the framework, with the contract fixed:

| Event | Data |
|---|---|
| `text` | the piece of text, as it is |
| `tool_call`, `tool_result`, `handoff` | `{agent, tool, call_id, arguments, output, to, error}` |
| `done` | `{agent, output, html, history, usage}` |
| `error` | `{message}` |

Without the header the whole answer comes at once — the request that arrives when JavaScript is
not there. That is the `done` payload as a JSON body, or whatever `ServeOpts.Page` renders.

| Field of `ServeOpts` | What it does |
|---|---|
| `MaxHistory` | how many past messages travel back into the model (default 40; negative keeps none). The history comes from the browser, so the ceiling is the server's |
| `MaxInput` | the largest request body accepted (default 256 KB) |
| `HTML` | renders the finished answer for the browser; `ui.ChatHTML` is the one that matches `ui.Chat`. Without it the answer stays text |
| `Page` | answers a request that did not ask for a stream, so the app can render the page with the message in it |

A failure before the first byte is an ordinary error (a [problem](/reference/errors) with its
status). After it, the status line is gone: the failure travels as an `error` event and the
stream closes.

The history is the app's — the framework keeps no chat session.
