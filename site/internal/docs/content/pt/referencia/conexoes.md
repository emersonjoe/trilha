---
title: connections
description: Connections, Connection, ConnectionKind, ConnectionStore, ValidateExternalURL e TestHTTP — os serviços externos com que uma aplicação fala, com o segredo selado e o botão Testar.
---

Uma aplicação de gestão tem a mesma tela três vezes sob três nomes — integrações, servidores
MCP, o provedor de LLM — porque "coisa de fora com que este app fala" é um padrão que ninguém
escreveu uma vez: nome, URL, tipo de autenticação, segredo mascarado que "vazio no update
mantém", botão Testar e o último resultado.

`trilha.Connections` é essa lista. Está no runtime porque precisa do que o runtime tem — a
requisição, o tenant do ator, o ambiente, a trilha de auditoria — e porque o
[`Secret`](/pt/referencia/seguranca) que ela guarda já mora lá.

## Declarar os tipos

```go
var Conexoes = trilha.NewConnections(trilha.ConnectionsOpts{Kinds: []trilha.ConnectionKind{
	{Key: "api", Label: "APIs", Auth: []string{"none", "bearer", "header", "basic"},
		Test: trilha.TestHTTP("GET", "/")},
	{Key: "mcp", Label: "Servidores MCP", Auth: []string{"none", "bearer"}, Test: testarMCP},
}})
```

Um tipo diz quais autenticações aceita e como é testado. `Auth` é um subconjunto de
`ConnectionAuths` — `none`, `bearer`, `header` (um cabeçalho com nome e o segredo como valor),
`basic` (usuário e o segredo como senha) — e um `Save` com uma autenticação que o tipo não
declarou é `FieldErrors` em `auth`. Tipo sem `Test` não tem botão Testar.

`Store` é memória por padrão; `Timeout` (30 s) limita um teste e todo cliente.

## O registro

```go
type Connection struct {
	ID, Tenant, Kind, Name, URL string
	Auth     string            // none | bearer | header | basic
	Username string            // basic
	Header   string            // header: o nome do cabeçalho
	Secret   Secret            // token, valor do cabeçalho ou senha
	Headers  map[string]string // cabeçalhos fixos, nunca um segredo
	LastTest *ConnectionTest   // At, OK, Message
	CreatedAt, UpdatedAt time.Time
}
```

| Método | Papel |
|---|---|
| `List(c)`, `Get(c, id)` | as do tenant, por tipo e depois nome; `ErrNotFound` |
| `Save(c, conn) (Connection, error)` | valida, cria ou atualiza, audita `connection.save` |
| `Delete(c, id)` | audita `connection.delete` |
| `Test(c, id) (ConnectionTest, error)` | roda o teste do tipo sob `Timeout`, grava `LastTest`, audita `connection.test` |
| `Client(c, id) (*http.Client, error)` | o cliente autenticado, preso ao host da URL |
| `Kinds()`, `Kind(key)` | o que foi declarado |

Todo método recebe o `*Ctx` porque o tenant vem de `c.Actor()`, o ambiente de `c.Env()` e o
registro de auditoria de `c.Audit` — as mesmas três coisas que o resto do runtime lê, e nenhuma
delas um argumento que alguém possa esquecer.

## O que o Save confere

- **A URL é externa.** `ValidateExternalURL(url, env)` quer `http` ou `https`, um host, nenhuma
  credencial no endereço e — em `Prod` — nada de loopback, rede privada, link-local ou terminado
  em `.local`, `.internal`, `.lan`. Em `Dev` o endereço privado passa com aviso no log, porque é
  lá que o serviço em teste mora.
- **Segredo vazio num update mantém o anterior.** O formulário renderiza o campo vazio
  ([`ui.SecretField`](/pt/referencia/ui)) e quem edita o nome não é obrigado a redigitar o token.
  Um valor novo substitui; o antigo não fica em lugar nenhum.
- **`Authorization` não é cabeçalho fixo.** É a autenticação, e um cabeçalho que levasse
  credencial às claras seria o segredo guardado fora do campo selado.
- **Trocar a URL, a autenticação ou o segredo limpa o `LastTest`.** Um badge verde numa conexão
  cujo token foi trocado é um badge sobre outra conexão.

O registro de auditoria leva tipo, nome, URL e autenticação. Nunca o segredo.

## O cliente

```go
cli, err := Conexoes.Client(c, id)
resp, err := cli.Do(req)   // req.URL em outro host → erro, antes de qualquer byte sair
```

O transporte é o único lugar em que o segredo é lido: ele põe o `Authorization` (ou o cabeçalho
nomeado, ou o par basic) e os cabeçalhos fixos, e **recusa qualquer requisição cujo host não seja
o da conexão**. Um redirect para outro domínio, portanto, não leva o token junto — o
acompanhamento é recusado pela mesma regra. `Connection.Client(timeout)` é o mesmo cliente para
quem escreve um `Test`.

`TestHTTP(method, path)` é o teste pronto para APIs: qualquer resposta abaixo de 400 passa, o
resto vira a mensagem (`HTTP 401 Unauthorized`). Para um servidor MCP, disque com
[`mcp.HTTPWith`](/pt/referencia/mcp#cliente) sobre o cliente da conexão e liste as ferramentas.

## O segredo

`Secret` se mascara em `%v`, em JSON e no `slog`; a tela renderiza o campo vazio; o registro de
auditoria não o leva. O que sobra é o store, e lá a regra é a mesma do resto do Trilha:
`Secret.Value()` sela com a chave do app na entrada do `database/sql`, `Scan` abre na saída. Um
store em memória o guarda às claras, como memória guarda.

## O store

```go
type ConnectionStore interface {
	List(ctx context.Context, tenant string) ([]Connection, error)
	Get(ctx context.Context, tenant, id string) (Connection, error)
	Save(ctx context.Context, conn Connection) error
	Delete(ctx context.Context, tenant, id string) error
}
```

Memória é o padrão. Uma tabela é uma linha por conexão com `headers` em JSON e `secret` como o
blob selado — e, como na [busca](/pt/referencia/search), o SQL mora no seu projeto: o framework
não tem driver.

## A tela e a receita

[`ui.ConnectionsPanel`](/pt/referencia/ui#connectionspanel) é a lista agrupada por tipo, o badge do
último teste e o formulário. Cada botão é um `POST` para um caminho só com um `_action` oculto
(`save`, `test`, `delete`), então funciona sem script.

```
trilha add connections
```

escreve o `internal/conexoes` com os tipos `api` e `mcp`, a página `/conexoes` com as três ações
e os testes — veja [Falar com uma API de terceiros](/pt/receitas/api-de-terceiros).
