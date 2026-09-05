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
