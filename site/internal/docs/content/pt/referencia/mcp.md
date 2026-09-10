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

### trilha mcp --from-routes

A outra coisa que a CLI responde é *como a minha API ficaria como ferramentas*: a lista que
[`FromRoutes`](#a-sua-api-como-ferramentas) publicaria, derivada de `app/` e do documento
OpenAPI, impressa antes de existir servidor.

```text
$ trilha mcp --from-routes
ferramentas que mcp.FromRoutes exporia (include: /api/*):
  deleteApiPostsId   DELETE /api/posts/{id}   DELETE removes a post.
  getApiPosts        GET /api/posts           GET lists posts.
  getApiPostsId      GET /api/posts/{id}      GET returns one post by slug.
  postApiPosts       POST /api/posts          POST creates a post from JSON.
```

`--include` aceita os mesmos padrões de `FromRoutesOpts.Include`. Uma rota deixada de fora —
um upload multipart, por exemplo — sai com o motivo, a mesma linha que o servidor registra
no log.

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

## A sua API como ferramentas

```go
func FromRoutes(app *trilha.App, o FromRoutesOpts) *Server
func Preview(openAPI []byte, routes map[string][]string, o FromRoutesOpts) ([]ToolInfo, []string, error)

type FromRoutesOpts struct {
	Name, Version string   // o que o initialize responde
	Include       []string // padrões a expor; padrão /api/*
	Exclude       []string // padrões a esconder, conferidos depois de Include
	OpenAPI       []byte   // o documento que trilha openapi escreve; obrigatório
}
```

Uma API escrita em `app/api/` já é o que um agente precisa: nome, descrição, esquema de
argumentos, handler. `FromRoutes` a publica como ferramentas MCP sem uma segunda declaração —
uma ferramenta por (método, rota), com o nome do operation id do documento (`getApiPosts`,
`postApiPosts`), a descrição do comentário do próprio handler, e os parâmetros do caminho, a
query e o corpo achatados num esquema de entrada só.

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

O documento é o que dá às ferramentas descrição e esquema — o runtime não os guarda —, por
isso ele mora ao lado da rota, em `app/mcp/openapi.json`, escrito por
`trilha openapi -o app/mcp/openapi.json` e embutido. O `trilha check` compara cada
`openapi.json` dentro de `app/` com as rotas: uma cópia que ficou para trás derruba o build,
não o agente. Sem documento, `FromRoutes` entra em pânico: ferramenta sem descrição é
ferramenta que o modelo usa errado.

**Chamar é chamar a rota.** A ferramenta monta a requisição — `{id}` no caminho, `q` na
query, o resto como corpo JSON — e a passa pelo `app.Handler()`, cadeia incluída: o limite de
taxa, a chave de API, a auditoria e o log veem a mesma requisição que um `curl` mandaria. O
`Authorization`, o `X-Forwarded-For`, o `X-Request-ID` e o `Accept-Language` de quem chamou
viajam junto; o ator do registro de auditoria diz `Via: "mcp"`. Um `4xx`/`5xx` vira resultado
com `isError: true` e o corpo como texto, para o modelo ler o `422` que o handler escreveu.

**O `tools/list` é por chamador.** Antes de listar uma ferramenta o servidor *sonda* a rota
com os cabeçalhos de quem chamou — a cadeia roda, o handler não — e uma rota que responderia
`401` ou `403` não entra na lista. Uma chave com `docs:read` vê as leituras; as escritas não
ficam escondidas atrás de uma recusa, elas não existem. A sonda não gasta token do limite de
taxa nem conta uso (ver [`App.Probe`](/pt/referencia/app#sondar-uma-rota)).

**O que fica de fora**, com o motivo no log e no `trilha mcp --from-routes`: páginas
(`page.go`), rotas fora do `Include`, `OPTIONS`/`HEAD` e handler cujo corpo é multipart — a
ponte manda JSON. `Include` e `Exclude` são prefixos com `*` no fim ou caminhos exatos.

`Preview` é o mesmo plano sem app: entram o documento, as rotas e as opções; saem a lista e os
avisos — o que a CLI imprime.

## Transporte próprio

`Transport` é uma interface (`Send`, `Recv`, `Close`). `Pipe(r, w, closer)` monta o
transporte de linha sobre qualquer par leitor/escritor, o que os testes usam com `io.Pipe`.
