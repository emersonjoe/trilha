# Spec 120 — Conexões externas: cadastro, segredo selado, testar

- **Issue**: [#153](https://github.com/emersonjoe/trilha/issues/153) — a issue é a fonte do escopo.
- **Branch**: `120-conexoes`
- **Versão**: 0.99.0

## Por quê

O Acervo tem a mesma tela três vezes — integrações, servidores MCP, provedor de LLM — porque
"coisa externa com que este app fala" é um padrão e ninguém o escreveu uma vez: nome, URL,
tipo de autenticação, segredo mascarado que "vazio no update mantém", botão Testar e o último
resultado. O Trilha já tem as peças difíceis (`Secret`, `Seal/Open`, `ui.SecretField`,
`RedirectExternal`); falta a lista de conexões com o teste.

## O que muda

**`trilha.Connections`** (núcleo, `connections.go`). Uma conexão é um registro:

```go
type Connection struct {
	ID, Tenant, Kind, Name, URL string
	Auth     string            // none | bearer | header | basic
	Username string            // basic
	Header   string            // header: o nome do cabeçalho
	Secret   Secret            // token, valor do cabeçalho ou senha — selado no store, mascarado fora
	Headers  map[string]string // cabeçalhos fixos, sem segredo
	LastTest *ConnectionTest   // At, OK, Message
	CreatedAt, UpdatedAt time.Time
}
```

`NewConnections(ConnectionsOpts{Store, Kinds, Timeout})`; `ConnectionKind{Key, Label, Auth,
Test}`; `ConnectionStore` (List/Get/Save/Delete por tenant) mais `ConnectionMemory()`. Métodos,
todos com `*Ctx` (tenant do `Actor`, auditoria, ambiente): `List`, `Get`, `Save`, `Delete`,
`Test`, `Client`, `Kinds`.

- `Save` valida: tipo declarado, `Auth` entre os do tipo, nome, `ValidateExternalURL(url, env)`
  — `http`/`https`, host obrigatório, sem loopback/rede privada/link-local em `Prod`; em dev a
  URL privada passa com aviso no log. Segredo vazio num update mantém o anterior; o registro de
  auditoria é `connection.save`, sem o segredo.
- `Test` roda o `Test` do tipo com prazo (`Timeout`, 30 s por padrão), grava
  `LastTest{At, OK, Message}` e audita `connection.test`. `TestHTTP(method, path)` é o teste
  pronto para APIs: qualquer 2xx/3xx passa, o resto vira a mensagem.
- `Client(c, id) (*http.Client, error)` devolve um cliente com `Timeout` e um transporte que
  monta a autenticação e os cabeçalhos fixos — **e que só fala com o host da URL**: um
  redirect para outro lugar não leva o segredo junto. `Connection.Client(timeout)` é o mesmo
  cliente para quem escreve um `Test`.
- Segredo: `Secret` já mascara em `%v`, JSON e `slog`; o store recebe o valor e é responsável
  por selar (`Secret.Value()` faz isso sozinho num `database/sql`). Nada do pacote escreve o
  segredo em HTML — `ui.SecretField` renderiza vazio.

**`mcp.HTTPWith(url, client)`** — o `Dialer` HTTP com um cliente pronto, para o teste de uma
conexão do tipo `mcp` usar o cliente autenticado.

**`ui.ConnectionsPanel(c, conns, opts)`** — a tela: lista agrupada por tipo (nome, URL,
autenticação, badge do último teste com `ui.Status`), formulário de nova/edição com
`ui.SecretField`, botões Testar e Apagar. Tudo sem JavaScript: cada botão é um `POST` para
`opts.Path` com `_action` (`save`, `test`, `delete`) e `id`; a página redireciona com `Flash`.

**Receita `trilha add connections`**: `internal/conexoes` (tipos `api` e `mcp`, store em
memória, `Setup`), a página em `{{.At}}conexoes` com as três ações, e os testes. `trilha
check` fica verde depois.

## Fora de escopo

- Store SQL no núcleo: como no `Search` (116) e no `api-usage` (117), store é interface mais
  memória; a forma da tabela vai na documentação e o SQL mora no app.
- Tipo `smtp`: `mail` não tem teste de conexão hoje; a issue entra com `api` e `mcp`.
- OAuth2 client-credentials; o `/assist` do Acervo — ambos listados como fora na issue.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `net/http`, `net/url`, `net/netip`, `context` |
| VI — teste primeiro | `TestConnections*` no núcleo, `TestConnectionsPanel*` no `ui`, receita na suíte de receitas |
| VII — segurança por padrão | segredo selado/mascarado; URL privada recusada em prod; cliente preso ao host; timeout obrigatório |
| Docs EN/PT no mesmo commit | referência `connections`/`conexoes`, `ui`, `cli`; receita "Talking to a third-party API" |

## Tarefas

- [x] T001 Testes que falham: `ValidateExternalURL`, `Save` (validação, segredo mantido), `Test` (resultado e auditoria), `Client` (auth, timeout, host preso), segredo ausente em JSON/log
- [x] T002 Núcleo: `connections.go` com tipos, store em memória, `TestHTTP`; `mcp.HTTPWith`
- [x] T003 `ui.ConnectionsPanel` com teste (segredo não aparece no HTML; badge do teste)
- [x] T004 Receita `connections` (`internal/recipes/connections.go`) e teste da receita
- [x] T005 `api/current.txt`, documentação EN + PT (referência, `ui`, `cli`; cookbook)
- [x] T006 `CHANGELOG.md`, `ROADMAP.md`, `version`, `make test`, `scripts/release.sh 0.99.0 --issues 153`

## Aceitação

- **SC-001** Criar, editar e testar uma conexão com o store em memória; o segredo não aparece
  no JSON, no log nem no HTML do painel (o teste procura a string).
- **SC-002** Update com segredo vazio mantém o anterior; com valor, troca.
- **SC-003** `http://127.0.0.1` é recusada em `Prod` e aceita em dev, com aviso no log.
- **SC-004** `Client()` sempre tem `Timeout`; uma requisição a outro host é recusada pelo
  transporte.
- **SC-005** `trilha add connections` num projeto novo deixa `trilha check` verde.
