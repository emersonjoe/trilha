# Feature Specification: IA — cliente OpenAI-compatível, agentes, multiagentes e MCP

**Branch**: `005-ia` | **Created**: 2026-09-05 | **Status**: Draft
**Input**: "implemente integração com IA, padrão OpenAI api, multi agentes, MCP"

## Princípio de desenho

Tudo na biblioteca padrão (constituição II). O pacote `ai` fala o **protocolo** da API de
chat da OpenAI (`POST /v1/chat/completions`), que hoje é o padrão de fato implementado por
OpenAI, Azure OpenAI, Anthropic (compat), Google (compat), Mistral, Groq, OpenRouter, Ollama,
LM Studio, vLLM e llama.cpp. Trocar de provedor é trocar `OPENAI_BASE_URL`. O pacote
`ai/mcp` fala o **Model Context Protocol** (JSON-RPC 2.0 sobre stdio e HTTP), nas duas
pontas: cliente (usar ferramentas de servidores MCP em agentes) e servidor (expor
ferramentas do seu app para Claude, Cursor, VS Code e outros).

## User Scenarios & Testing

### US1 - Chamar um modelo (P1)

```go
cli := ai.NewFromEnv() // OPENAI_API_KEY, OPENAI_BASE_URL (padrão api.openai.com/v1), TRILHA_AI_MODEL
resp, err := cli.Chat(ctx, ai.Request{Messages: []ai.Message{ai.User("Resuma isto: ...")}})
fmt.Println(resp.Text())
```

Streaming entrega deltas conforme chegam; `ResponseFormat` pede JSON com schema;
`Tools` declara funções e a resposta traz `ToolCalls`.

**Acceptance**: contra um servidor falso local: (1) requisição correta (modelo, mensagens,
cabeçalho Authorization, cabeçalhos extras); (2) `Stream` reconstrói texto e tool calls a
partir dos chunks `data:` e termina em `[DONE]`; (3) erro HTTP vira `*ai.Error{Status,
Code, Message}`; (4) `Usage` preenchido; (5) `ctx` cancelado interrompe o stream.

### US2 - Agente com ferramentas (P1)

```go
clima := ai.NewTool("clima", "Temperatura atual de uma cidade",
	ai.Schema(`{"type":"object","properties":{"cidade":{"type":"string"}},"required":["cidade"]}`),
	func(ctx context.Context, args json.RawMessage) (string, error) { ... })

agente := &ai.Agent{Name: "assistente", Instructions: "Responda em português.", Tools: []*ai.Tool{clima}}
res, err := ai.Run(ctx, cli, agente, "Como está o tempo em Recife?")
```

`Run` executa o laço modelo → ferramentas → modelo até a resposta final ou `MaxTurns`.
Ferramentas de uma mesma rodada rodam em paralelo. `RunStream` emite eventos (`text`,
`tool_call`, `tool_result`, `handoff`, `done`).

**Acceptance**: servidor falso que pede a ferramenta na 1ª rodada e responde na 2ª: o
resultado final contém o texto, `Result.Messages` tem a sequência completa, `Steps` lista a
chamada de ferramenta com argumentos e saída; erro da ferramenta é devolvido ao modelo como
mensagem de erro (não derruba o laço); `MaxTurns` estourado devolve `ErrMaxTurns`.

### US3 - Multiagentes (P1)

- **Handoff**: `Agent.Handoffs []*Agent` gera ferramentas `transfer_to_<nome>`; ao ser
  chamada, o agente alvo assume a conversa (instruções trocadas, histórico mantido).
- **Agente como ferramenta**: `agente.AsTool("descrição")` devolve um `*ai.Tool` que roda o
  agente com `{"input": "..."}` e devolve o texto.
- **Orquestração**: `ai.Parallel(ctx, cli, input, agentes...)` roda vários e devolve os
  resultados; `ai.Chain(ctx, cli, input, agentes...)` encadeia a saída de um como entrada do
  próximo.

**Acceptance**: triagem → especialista via handoff (o servidor falso chama
`transfer_to_financeiro` e a 2ª rodada recebe as instruções do financeiro); `AsTool`
executa o subagente; `Parallel` devolve na ordem e propaga o primeiro erro; `Chain` passa a
saída adiante.

### US4 - MCP cliente (P2)

```go
srv, err := mcp.Dial(ctx, mcp.Stdio("npx", "-y", "@modelcontextprotocol/server-filesystem", "."))
tools, err := srv.Tools(ctx)        // []*ai.Tool prontos para um Agent
```

Também `mcp.HTTP("https://host/mcp", headers)`. `initialize` → `notifications/initialized` →
`tools/list` → `tools/call`. Resultado com `isError` vira erro da ferramenta.

**Acceptance**: cliente e servidor do próprio pacote conversando por `io.Pipe` (stdio) e por
`httptest` (HTTP): lista, chama, propaga `isError`, respeita `Mcp-Session-Id`.

### US5 - MCP servidor (P2)

```go
// app/mcp/route.go
var server = mcp.NewServer("agenda", "1.0.0", listarEventos, criarEvento)
func POST(c *trilha.Ctx) error { return server.ServeHTTP(c) }
```

Também `server.ServeStdio(ctx, os.Stdin, os.Stdout)` para rodar como processo local.
Ferramentas são os mesmos `*ai.Tool` usados nos agentes.

**Acceptance**: o exemplo expõe `/mcp`; um cliente `mcp.HTTP` lista e chama as ferramentas;
`GET /mcp` responde 405; sessões são criadas no `initialize`.

### US6 - Integração com o Trilha e exemplo (P1)

`c.Stream()` abre uma resposta SSE (`Send(evento, dados)`, `JSON(evento, v)`) para levar
deltas ao navegador. Exemplo `examples/assistente`: página de chat que envia a pergunta,
recebe a resposta em streaming, agente com duas ferramentas locais, handoff para um
especialista, servidor MCP em `/mcp`. Funciona com qualquer provedor via
`OPENAI_BASE_URL` (documentado com Ollama para uso sem chave).

### Edge Cases
- Provedor que não envia `usage` no stream: `Usage` zero, sem erro.
- `tool_calls` com argumentos fragmentados em vários chunks: concatenar por `index`.
- Ferramenta desconhecida pedida pelo modelo: mensagem de erro de volta ao modelo.
- Argumentos JSON inválidos: idem, com o erro de parse.
- Handoff em cadeia (A → B → C) e ciclo (A → B → A): limitado por `MaxTurns`.
- MCP `tools/list` paginado (`nextCursor`): seguir até o fim.
- Stdio com linhas inválidas: ignoradas com log; EOF encerra.

## Requirements
- **FR-001** `ai.Client` com `Chat`, `Stream`, `NewFromEnv`, cabeçalhos extras, timeout e `*ai.Error`.
- **FR-002** Tipos do protocolo: `Message`, `Tool`, `ToolCall`, `Request` (`Model`, `Messages`, `Tools`, `ToolChoice`, `Temperature`, `MaxTokens`, `ResponseFormat`, `Extra` para campos do provedor), `Response`, `Usage`, `Delta`.
- **FR-003** `ai.Tool` com schema JSON e função Go; `ai.Agent`; `ai.Run`/`ai.RunStream` com `MaxTurns` (padrão 10) e ferramentas em paralelo.
- **FR-004** Handoffs, `AsTool`, `Parallel`, `Chain`.
- **FR-005** `ai/mcp`: `Client` (stdio, HTTP), `Server` (stdio, HTTP), conversão para `ai.Tool`, protocolo `2025-03-26`.
- **FR-006** `trilha.Ctx.Stream()` para SSE.
- **FR-007** Exemplo `examples/assistente` com testes contra servidor falso; docs (capítulo "IA e agentes", referência `ai` e `mcp`).
- **FR-008** Nenhuma chave ou prompt registrado em log; `Authorization` nunca aparece em erros.

## Success Criteria
- **SC-001** `go test ./ai/...` cobre chat, stream, tools, agente, handoff, MCP (ambos transportes) sem rede.
- **SC-002** Exemplo roda contra um provedor real ou local trocando só variáveis de ambiente.
- **SC-003** Zero dependências mantido.
