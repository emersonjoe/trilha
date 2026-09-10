# Spec 119 — A API como ferramentas MCP, com a chave de quem chama

- **Issue**: [#152](https://github.com/emersonjoe/trilha/issues/152) — a issue é a fonte do escopo.
- **Branch**: `119-mcp-das-rotas`
- **Versão**: 0.98.0

## Por quê

Quem já tem uma API por chave e quer oferecê-la a um agente escreve hoje a mesma cola que o
Acervo escreveu em Python: uma função por rota que chama a própria API por HTTP repassando o
cabeçalho. O Trilha tem as três peças — `trilha openapi` descreve as rotas, `auth.APIKeys`
autentica, `mcp.NewServer` serve ferramentas — e não tem a cola: transformar rota em `*ai.Tool`
e fazer a chave do cliente MCP ser o `Actor` da rota, sem sair do processo.

## O que muda

**`mcp.FromRoutes(app, opts) *Server`.** Uma ferramenta por (rota de API, método) incluída. A
chamada não vai pela rede: o `Server` monta uma requisição em processo com o `Authorization`, o
`Host` e o endereço de quem chamou o MCP e a entrega ao `app.Handler()`, então a cadeia da rota —
`Keys.Require`, `RequirePolicy`, limite, auditoria — roda como para qualquer cliente. Resposta 2xx
vira o texto do resultado; 4xx/5xx vira `isError` com o corpo (o `Problem`) como texto.

```go
// app/setup.go
//go:embed mcp/openapi.json
var openAPI []byte

func Setup(a *trilha.App) error {
	trilha.Provide(a, mcp.FromRoutes(a, mcp.FromRoutesOpts{
		Name: "acervo", Version: "1.0",
		Include: []string{"/api/v1/*"},
		OpenAPI: openAPI,
	}))
	return nil
}

// app/mcp/route.go
func POST(c *trilha.Ctx) error { return trilha.Use[*mcp.Server](c).ServeHTTP(c) }
```

- `FromRoutesOpts{Name, Version, Include, Exclude []string, OpenAPI []byte}`. `Include` vazio é
  `/api/*`; um padrão termina em `*` para pegar a subárvore. Só rotas de API entram (`Kind`
  API, ou `route.go` sem `Kind`); `OPTIONS` e `HEAD` não viram ferramenta.
- **Com o documento** do `trilha openapi`: o nome é o `operationId`, a descrição é o `summary`
  (e a `description`), o schema de entrada é um objeto com os parâmetros de caminho, os de
  query e as propriedades do corpo, achatados; `$ref` aos componentes vira `$defs` dentro do
  próprio schema. Operação com corpo `multipart/form-data` fica de fora e o `FromRoutes` avisa
  no log do app. **Sem o documento**: mesmo nome, descrição `MÉTODO /caminho`, schema só com os
  parâmetros de caminho e `additionalProperties` (o resto vai para a query em GET/DELETE e para o
  corpo JSON nos demais).
- **`tools/list` é por chamador**: antes de listar, cada rota é sondada com o mesmo
  `Authorization` (`App.Probe`), e a que a cadeia recusa não aparece — nem pode ser chamada.
- **`Actor.Via` é `"mcp"`** na trilha de auditoria de tudo o que a ferramenta fizer, com o
  `Subject` da chave; o `Ctx.Actor()` de dentro da rota diz o mesmo.
- `trilha mcp --from-routes` imprime as ferramentas que seriam expostas (nome, método e caminho,
  resumo) e os avisos, a partir do código, sem subir nada.

**No núcleo**, o que a cola precisa e que uma aplicação também pode usar:

- `App.Route(pattern) (Route, bool)` — a rota registrada, para quem precisa do `Kind` e dos
  métodos e não só da lista de padrões.
- `App.Probe(req) bool` — roda a cadeia de middleware da rota que atenderia `req` e diz se o
  handler seria alcançado; o handler não roda, nada é registrado no log de acesso nem nas
  métricas, e `Ctx.Probing()` é verdadeiro lá dentro. `auth.Keys.Require` não gasta limite nem
  conta uso numa sonda.
- `trilha.WithVia(req, via) *http.Request` — marca por onde a requisição chegou; o
  `SetActor` respeita a marca. É o que faz a chave reconhecida dentro da ponte dizer `mcp` e
  não `api_key`.

`trilha check` passa a conferir também um `openapi.json` sob `app/` (é o que a rota embute),
não só o da raiz.

## Fora de escopo

- Servir por stdio a partir das rotas: `ServeStdio` já existe; sem requisição não há chave, e
  toda rota guardada responde 401 — a issue diz que a cola é HTTP.
- Recursos e prompts MCP: só tools, como o resto do pacote.
- Gerar o documento em runtime: o `openapi` lê código-fonte; embute-se o arquivo.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `encoding/json`, `net/http`; o recorder é próprio |
| VI — teste primeiro | `TestFromRoutes*` com três rotas e chaves antes do código |
| VII — segurança por padrão | a chave passa pela mesma cadeia; sonda não conta uso; só API entra |
| Docs EN/PT no mesmo commit | referência `mcp`, `cli`, `app`; receita "API as agent tools" |

## Tarefas

- [x] T001 Testes que falham: `App.Probe`/`Ctx.Probing`, `WithVia`, `App.Route`
- [x] T002 Núcleo: os três símbolos; `auth.Keys.Require` respeita a sonda
- [x] T003 Testes que falham em `ai/mcp`: três rotas + chaves → três ferramentas; chave inválida; chave sem escopo; `Via: mcp`; multipart de fora; schema do documento
- [x] T004 `mcp.FromRoutes`, `FromRoutesOpts`, listagem por chamador no `Server`
- [x] T005 `trilha mcp --from-routes`; `check` confere `app/**/openapi.json`
- [x] T006 Uso em `examples/blog` (`app/mcp`), `api/current.txt`
- [x] T007 Documentação EN + PT (referência `mcp`, `app`, `cli`; cookbook)
- [x] T008 `CHANGELOG.md`, `ROADMAP.md`, `version`, `make test`, `scripts/release.sh 0.98.0 --issues 152`

## Aceitação

- **SC-001** App com três rotas `/api/v1` atrás de `Keys.Require`: `ListTools` com chave válida
  devolve três; `CallTool` devolve o mesmo corpo que a rota responde por HTTP.
- **SC-002** Chave inválida: `CallTool` volta `isError` com 401 e o handler não roda.
- **SC-003** Chave sem o escopo de uma rota: a ferramenta não está em `ListTools`.
- **SC-004** O registro de auditoria da chamada traz `Via: "mcp"` e o `Subject` da chave.
- **SC-005** Operação `multipart` não vira ferramenta e o log do app avisa.
- **SC-006** `trilha mcp --from-routes` no projeto de teste lista as operações da API.
