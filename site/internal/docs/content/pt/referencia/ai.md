---
title: ai
description: Cliente OpenAI-compatível, ferramentas, agentes, handoffs e composição.
---

`import "github.com/emersonjoe/trilha/ai"` — sem dependências externas.

## Client

| Campo / função | Papel |
|---|---|
| `NewFromEnv() *Client` | lê `OPENAI_API_KEY`, `OPENAI_BASE_URL` (padrão `https://api.openai.com/v1`) e `TRILHA_AI_MODEL` (ou `OPENAI_MODEL`; padrão `gpt-4o-mini`) |
| `BaseURL, APIKey, Model string` | configuração direta |
| `Headers map[string]string` | cabeçalhos extras (OpenRouter, Azure...) |
| `HTTPClient *http.Client` | cliente HTTP (padrão com timeout de 2 min) |
| `Chat(ctx, Request) (*Response, error)` | uma chamada; `Response.Text()` e `Response.ToolCalls()` |
| `Stream(ctx, Request, func(Delta) error) error` | resposta em pedaços; `Delta.Content`, `Delta.ToolCalls`, `Delta.Usage` no fim |

Respostas não-2xx viram `*ai.Error{Status, Code, Message}`.

## Request e mensagens

`Request{Model, Messages, Tools, ToolChoice, Temperature, MaxTokens, ResponseFormat, Extra}`.
`Extra map[string]any` é mesclado no JSON enviado, para parâmetros específicos do provedor.
`ResponseFormat{Type: "json_schema", JSONSchema: ...}` pede saída estruturada.

Construtores: `ai.System(s)`, `ai.User(s)`, `ai.Assistant(s)`, `ai.ToolResult(callID, s)`.

## Tool

```go
func NewTool(name, description string, schema json.RawMessage, fn ToolFunc) *Tool
type ToolFunc func(ctx context.Context, args json.RawMessage) (string, error)
func Schema(s string) json.RawMessage           // valida o JSON; pânico no início se inválido
func Typed[T any](fn func(ctx, in T) (string, error)) ToolFunc
```

`schema == nil` significa "sem argumentos". Erros e pânicos da função viram texto para o
modelo (`error: ...`) e aparecem em `Step.Err`.

## Agent

| Campo | Papel |
|---|---|
| `Name` | identifica o agente em `Step.Agent` e nos handoffs (`transfer_to_<slug>`) |
| `Instructions` | mensagem `system` |
| `Model` | substitui o modelo do cliente |
| `Tools []*Tool` | ferramentas |
| `Handoffs []*Agent` | agentes para os quais este pode transferir a conversa |
| `MaxTurns` | limite de chamadas ao modelo por `Run` (padrão 10; excedido → `ErrMaxTurns`) |
| `Temperature *float64`, `ResponseFormat` | passados em cada requisição |

```go
func Run(ctx, cli *Client, agent *Agent, input string, history ...Message) (*Result, error)
func RunStream(ctx, cli *Client, agent *Agent, input string, fn func(Event), history ...Message) (*Result, error)
```

`Result{Output, Agent, Messages, Steps, Usage, Turns}`. `Messages` serve de histórico para a
próxima chamada (mensagens `system` do histórico são ignoradas; valem as do agente atual).

`Event.Type`: `text` (`Text`), `tool_call` e `tool_result` (`Step`), `handoff` (`Step.HandoffTo`,
`Agent` = novo agente), `done` (`Result`), `error` (`Err`).

Ferramentas de uma mesma rodada rodam em paralelo; a ordem dos resultados no histórico é a
ordem em que o modelo as pediu. Um handoff troca a mensagem `system`, mantém o histórico e
continua o laço com o agente alvo.

## Composição

```go
func (a *Agent) AsTool(cli *Client, description string) *Tool   // {"input": "..."} → texto
func Parallel(ctx, cli, input string, agents ...*Agent) ([]*Result, error)
func Chain(ctx, cli, input string, agents ...*Agent) (*Result, error)
```

`Parallel` devolve na ordem dos agentes e propaga o primeiro erro; `Chain` passa `Output` de um
como `input` do próximo.
