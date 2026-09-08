---
title: mcp
description: Cliente e servidor do Model Context Protocol (stdio e Streamable HTTP).
---

`import "github.com/emersonjoe/trilha/ai/mcp"` — JSON-RPC 2.0, revisão `2025-03-26`, sem
dependências externas. Cobre o recurso *tools* (listar e chamar).

## trilha mcp — o projeto como ferramentas

Agente com shell não precisa disto: `trilha ctx --json` e `trilha check --json` já são a
resposta. Isto é para o agente **sem** shell — um cliente de chat, um editor que fala MCP e
mais nada — que de outro jeito precisa adivinhar o que o seu projeto tem dentro.

```bash
cd meu-projeto
trilha mcp
```

Ele fala MCP por stdin/stdout e responde sobre **o projeto em que foi iniciado**. Aponte seu
cliente para ele:

```json
{
  "mcpServers": {
    "trilha": { "command": "trilha", "args": ["mcp"], "cwd": "/caminho/do/meu-projeto" }
  }
}
```

Esse formato vale no Claude Desktop, no Claude Code e em qualquer cliente que suba um servidor
por stdio. O `cwd` é quem decide o projeto, e nada que o modelo mande muda isso.

### As ferramentas

| Ferramenta | O que responde | O que está por trás |
|---|---|---|
| `describe_project` | rotas, API, tipos e setup, em JSON | `trilha ctx --json` |
| `routes` | a tabela de rotas em ordem de precedência | `trilha routes` |
| `ui_describe` | o catálogo do `ui`: assinatura, para que serve, exemplo que compila | o catálogo que vem no binário — dispensa projeto |
| `check` | gen, `gofmt`, `vet`, testes, auditoria e OpenAPI, em JSON | `trilha check --json` |
| `generate` | grava página, rota, teste ou componente | `trilha generate` — **só com `--write`** |

Cada uma é um invólucro: a ferramenta responde o que o comando responde, byte a byte, e há um
teste que segura isso. Não existe uma segunda implementação para sair de sincronia.

### O que ele não faz

**É somente leitura até você dizer o contrário.** Sem `--write`, a ferramenta que grava não é
registrada — ela não aparece no `tools/list`, então o modelo não tem como pedi-la e não há
recusa para discutir. Suba com `trilha mcp --write` quando quiser que o agente gere código; a
ferramenta diz *Writes files* na primeira linha da própria descrição, que é o que a pessoa que
aprova a chamada lê.

**Nunca usa shell.** Todo comando roda como programa mais uma lista de argumentos. Valor vindo
do modelo nunca é concatenado numa linha de comando.

**Confere argumento contra lista do que é permitido**, não contra lista do que é proibido.
Endereço de rota tem que parecer endereço de rota; nome de componente tem que ser
identificador; método tem que ser um dos seis. `/../../etc/passwd`, `/x; rm -rf /` e `--force`
são recusados com o motivo, antes de qualquer coisa rodar.

**Não sai do projeto.** O diretório de trabalho é decidido quando o servidor sobe.

**Um comando por vez**, cada um com prazo (10 minutos para o `check`, que roda a sua suíte; um
minuto para os outros) e teto de 1 MiB de saída, para que uma chamada de ferramenta não vire
bomba de processos nem coma memória.

**Não abre conexão de rede**, e não escreve nada no stdout além do protocolo — toda linha de
log vai para o stderr, onde você acompanha o que o agente pediu:

```
trilha mcp 0.43.0 · /caminho/do/meu-projeto · read-only unless --write; no shell; arguments allow-listed
tools: describe_project, check, routes, ui_describe
→ trilha ctx --json
```

### O que ainda não está aqui

Receitas e páginas de referência não são ferramentas deste servidor: são o mesmo Markdown de
que o site é feito, cerca de um megabyte, e pôr isso dentro do binário da CLI para responder
sobre um projeto do qual ele não faz parte é a troca errada. Esse é o trabalho de um servidor
de documentação hospedado, que continua sendo a
[#50](https://github.com/emersonjoe/trilha/issues/50).

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
