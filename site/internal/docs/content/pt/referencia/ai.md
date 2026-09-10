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

## Chat por HTTP

```go
func Serve(c *trilha.Ctx, cli *Client, agent *Agent) error
func (o ServeOpts) Serve(c *trilha.Ctx, cli *Client, agent *Agent) error
```

`Serve` é a metade de rota do [`ui.Chat`](/pt/referencia/ui#chat): lê a mensagem, roda o agente
e responde ao pedido.

```go
// app/api/chat/route.go
func POST(c *trilha.Ctx) error {
	return ai.ServeOpts{HTML: ui.ChatHTML}.Serve(c, cliente, assistente)
}
```

O pedido traz `{"message": "…", "history": [...]}` em JSON, ou um campo `message` de formulário.
Uma lista `{"messages": [...]}` também é lida: o último turno `user` é a mensagem e o que vem
antes é o histórico.

Quando o cliente pede `Accept: text/event-stream`, a resposta é um fluxo de eventos com nome —
o [`Stream`](/pt/referencia/ctx) do framework, com o contrato fixo:

| Evento | Dado |
|---|---|
| `text` | o pedaço de texto, como está |
| `tool_call`, `tool_result`, `handoff` | `{agent, tool, call_id, arguments, output, to, error}` |
| `done` | `{agent, output, html, history, usage}` |
| `error` | `{message}` |

Sem o cabeçalho a resposta inteira vem de uma vez — o pedido que chega quando o JavaScript não
está lá. É o mesmo conteúdo do `done` como corpo JSON, ou o que o `ServeOpts.Page` renderizar.

| Campo de `ServeOpts` | O que faz |
|---|---|
| `MaxHistory` | quantas mensagens antigas voltam para o modelo (padrão 40; negativo não guarda nenhuma). O histórico vem do navegador, então o teto é do servidor |
| `MaxInput` | maior corpo de pedido aceito (padrão 256 KB) |
| `HTML` | renderiza a resposta pronta para o navegador; `ui.ChatHTML` é o que combina com `ui.Chat`. Sem ele a resposta fica em texto |
| `Page` | responde ao pedido que não pediu fluxo, para o app desenhar a página com a mensagem dentro |
| `Context` | recebe os campos `ctx.*` que a página mandou junto com a mensagem — o id do registro aberto, a rota em que a pessoa está — e o que devolve entra na frente do histórico |

O `Context` é onde o [`ui.ChatOpts.Context`](/pt/referencia/ui#chat) chega, pelos dois caminhos: o
JSON que o script manda e os campos escondidos de um formulário que submeteu sozinho.

```go
ai.ServeOpts{Context: func(c *trilha.Ctx, campos map[string]string) []ai.Message {
	return []ai.Message{{Role: "user", Content: "A folha aberta é a " + campos["folha"] + "."}}
}}
```

Os campos vêm do pedido, então são o que a pessoa mandou e não o que o servidor sabe: confira ali
o que um id dá acesso, do mesmo jeito que se confere um parâmetro de query.

Falha antes do primeiro byte é erro comum (um [problema](/pt/referencia/erros) com o status
dele). Depois dele a linha de status já foi: a falha viaja como evento `error` e o fluxo fecha.

O histórico é do app — o framework não guarda sessão de conversa.
