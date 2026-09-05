# Implementation Plan: IA

**Branch**: `005-ia` | **Spec**: [spec.md](spec.md)

## Constitution Check
| Princípio | Atende |
|---|---|
| II stdlib | `net/http`, `encoding/json`, `bufio` (SSE), `os/exec` (stdio MCP), `sync` |
| IV contrato | `c.Stream()` é um método novo de `Ctx`; nada muda nas assinaturas |
| VI teste primeiro | servidor OpenAI falso em `ai/fake_test.go`; MCP em pipes e httptest |
| VII segurança | chave só em cabeçalho; nunca em log/erro; ferramentas MCP remotas são código de terceiros: documentado |

## Estrutura
```text
ai/
├── client.go     Client, NewFromEnv, Chat, Stream, Error, SSE parser
├── types.go      Message, Tool (definição), ToolCall, Request, Response, Usage, Delta, helpers User/System/Assistant/ToolResult
├── tool.go       Tool (com Func), NewTool, Schema, Toolset, execução com erro capturado
├── agent.go      Agent, Run, RunStream, Result, Step, Event, ErrMaxTurns, handoff
├── multi.go      AsTool, Parallel, Chain
└── mcp/
    ├── jsonrpc.go  tipos JSON-RPC 2.0
    ├── transport.go Transport interface; stdio (io.Reader/Writer, exec); HTTP client
    ├── client.go   Client: Dial, Initialize, Tools (→ []*ai.Tool), CallTool, Close
    └── server.go   Server: NewServer, ServeHTTP (trilha.Ctx e http.Handler), ServeStdio
stream.go (raiz)   Ctx.Stream() → *Stream{Send, JSON, Flush}
examples/assistente/ app/{layout,page,setup}.go, app/api/chat/route.go (stream), app/mcp/route.go, internal/ferramentas
```

## Decisões
- Streaming OpenAI: parser SSE próprio (linhas `data:`, `[DONE]`); tool calls acumuladas por `index`.
- `Request.Extra map[string]any` é mesclado no JSON de saída para campos específicos de provedor.
- Handoff: ferramenta sintética `transfer_to_<slug>`; ao acionar, `Run` troca o agente corrente, substitui a mensagem de sistema e continua o laço; evento `handoff`.
- Ferramentas paralelas: `sync.WaitGroup`, resultados na ordem das chamadas.
- MCP HTTP: cliente envia `Accept: application/json, text/event-stream`; aceita resposta JSON ou SSE (primeiro `data:` com o resultado). Servidor responde JSON; `Mcp-Session-Id` emitido no `initialize` e exigido depois (sessões em memória com expiração).
- MCP stdio: JSON por linha; `Dial` com `exec.Cmd` (stdin/stdout), `Stdio(cmd, args...)`.
- `c.Stream()`: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`, `NoWriteDeadline`, flush por evento.
