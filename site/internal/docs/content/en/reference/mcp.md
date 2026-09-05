---
title: mcp
description: Cliente e servidor do Model Context Protocol (stdio e Streamable HTTP).
---

`import "github.com/emersonjoe/trilha/ai/mcp"` — JSON-RPC 2.0, revisão `2025-03-26`, sem
dependências externas. Cobre o recurso *tools* (listar e chamar).

## Cliente

```go
func Dial(ctx, dial Dialer) (*Client, error)     // abre o transporte e faz initialize
func Stdio(name string, args ...string) Dialer    // processo filho, JSON por linha
func HTTP(url string, headers map[string]string) Dialer  // Streamable HTTP (POST por mensagem)
```

| Método | Papel |
|---|---|
| `ListTools(ctx) ([]ToolInfo, error)` | segue a paginação (`nextCursor`) |
| `CallTool(ctx, name, args) (CallResult, error)` | `CallResult.Text()` junta os itens de texto |
| `Tools(ctx) ([]*ai.Tool, error)` | ferramentas prontas para um `ai.Agent`; `isError` vira erro |
| `Server.Name/Version/ProtocolVersion` | preenchidos pelo `initialize` |
| `Close()` | fecha o transporte e encerra o processo filho |

O cliente HTTP guarda o `Mcp-Session-Id` recebido no `initialize` e o envia nas mensagens
seguintes; aceita respostas JSON ou `text/event-stream`.

## Servidor

```go
func NewServer(name, version string, tools ...*ai.Tool) *Server
func (s *Server) ServeHTTP(c *trilha.Ctx) error         // em app/.../route.go: POST
func (s *Server) Handler() http.Handler                  // fora do Trilha
func (s *Server) ServeStdio(ctx, r io.Reader, w io.Writer) error
```

Métodos atendidos: `initialize`, `ping`, `tools/list`, `tools/call`; notificações são
aceitas sem resposta (`202`). Em HTTP, `initialize` emite `Mcp-Session-Id`; mensagens sem
sessão válida recebem `404`; sessões expiram após `SessionTTL` (1 h) sem uso. Só `POST` é
aceito (`405` com `Allow: POST` para o resto). Corpo limitado a 4 MiB.

Erros e pânicos de ferramentas viram resultado com `isError: true`, como manda o protocolo;
ferramenta desconhecida é erro JSON-RPC `-32602`.

## Transporte próprio

`Transport` é uma interface (`Send`, `Recv`, `Close`). `Pipe(r, w, closer)` monta o
transporte de linha sobre qualquer par leitor/escritor, o que os testes usam com `io.Pipe`.
