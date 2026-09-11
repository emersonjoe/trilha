# Spec 127 — O CRUD em SQL: o contexto na interface, e o `--store`

- **Issue**: [#115](https://github.com/emersonjoe/trilha/issues/115) — a issue é a fonte do
  escopo. Ela não fecha aqui: fecham as duas linhas que a 0.105.0 deixou prontas.
- **Branch**: `claude/happy-fermi-l0427v`
- **Versão**: 0.106.0

## Por quê

A 0.105.0 tomou a decisão que travava a #115: a receita `store` passou a ser dona do dialeto,
da convenção de migração e do runner, **para que um gerador pudesse emitir contra eles**. O que
falta é o gerador — e o comentário que registrou isso também registrou o obstáculo:

> A interface que o gerador emite hoje é `List(q Query)` / `Get(id) (T, bool)`. Sem
> `context.Context`, e com o `Get` sem `error`. Emitir SQL contra ela repetiria inteira a
> [#165](https://github.com/emersonjoe/trilha/issues/165), que fechou na 0.103.0: sem contexto o
> store não honra o prazo da requisição nem cancela quando o navegador desliga, e sem erro um
> banco fora do ar vira "não existe".

São duas coisas, e a ordem entre elas não é opcional. Um `Get(id) (Tipo, bool)` implementado em
`database/sql` tem de escolher entre engolir o erro — e responder 404 para um banco caído, que
é a mentira mais cara que uma tela pode contar — ou entrar em pânico. E uma consulta sem
contexto continua rodando depois que quem pediu foi embora: numa listagem com busca, é uma
varredura por visitante que desistiu.

Por isso o contexto é a primeira metade desta spec, e não um detalhe dela. Ele muda a interface,
o store de memória, as três telas e o teste gerado — todo mundo que o gerador escreve.

## O que muda

### 1. A interface que o gerador emite

```go
type TipoStore interface {
	List(ctx context.Context, q TipoQuery) ([]Tipo, int, error)
	Get(ctx context.Context, id string) (Tipo, error)
	Create(ctx context.Context, v Tipo) (Tipo, error)
	Update(ctx context.Context, id string, v Tipo) (Tipo, error)
	Delete(ctx context.Context, id string) error
}
```

O `bool` de "achou" vira `error`, e "não achou" é `trilha.ErrNotFound` — o mesmo erro que a
receita devolve no `store.NotFound`, e que o framework já transforma na página de 404 do app.
Assim o handler fica com uma linha, `return err`, e o banco fora do ar sobe como 500 em vez de
virar "não existe".

O contexto vem do `c.Context()`, que é o da requisição: quando o navegador desliga, a consulta
é cancelada; quando o `deadline` da rota estoura, ela para junto.

### 2. `--store sqlite|postgres`

```
$ trilha generate crud docs.Tipo --at app/admin/tipos --store sqlite
  + internal/docs/tipo_store.go       TipoStore: a interface e a memória atrás
  + internal/docs/tipo_store_sql.go   a mesma interface em database/sql, contra a receita store
  + internal/docs/tipo_store_sql_test.go
  + migrations/0002_tipos.sql         a tabela, no dialeto pedido
  + app/admin/tipos/page.go           …as três telas e o teste, como antes
  + app/setup.go                      o store SQL, depois do store.Setup
```

O store SQL não inventa nada: ele é escrito **contra o kit da receita**, que é o que a 126
existe para ser. `store.Sortable` + `store.OrderBy` para o `ORDER BY` que veio da URL,
`store.Paginate` para o teto de página, `store.Like` para a caixa de busca, `d.Arg(n)` para
todo valor, `store.NotFound` para o `sql.ErrNoRows`. Nenhum valor é concatenado em nenhum
comando, e a listagem gerada é a tela que a 126 nomeia como o convite à injeção.

Sem a receita no projeto, o `--store sqlite` é **recusa antes da primeira escrita**, dizendo
`trilha add store`. Um gerador que escreve contra um pacote que não existe entrega um projeto
que não compila.

A memória continua sendo escrita, e é de propósito: é ela que o teste de unidade de quem usa o
store pega sem subir banco, e é a implementação de referência das cinco assinaturas.

### 3. O que o `app/setup.go` ganha

Com `--store sqlite`, a linha é

```go
trilha.Provide[docs.TipoStore](a, docs.NewTipoSQL(store.DB, store.D))
```

e ela entra **no fim** do `Setup`, não no começo: `store.DB` só existe depois do
`store.Setup(a)`. Um `Provide` antes dele guardaria um pool nulo, e o erro apareceria na
primeira requisição — que é exatamente o modo de falhar que o `wireSetup` foi escrito para
evitar. Um `Setup` sem a chamada da receita é recusa que diz qual linha falta.

### 4. O teste gerado, quando o store é SQL

O teste do CRUD sobe o app inteiro, e um app com a receita `store` não sobe sem `DATABASE_URL`
nem sem driver — e driver é `go get` que o framework não dá. Então o teste gerado começa com

```go
if os.Getenv("DATABASE_URL") == "" {
	t.Skip("DATABASE_URL não está definida: …")
}
```

pelo mesmo motivo do `Skip` que ele já carrega sob pasta fechada: um teste que falha por algo
fora dele ensina a lição errada. Com a variável definida e o driver instalado, ele roda
inteiro. O que roda sempre é o `tipo_store_sql_test.go`: ele não precisa de banco, porque o que
ele afirma é sobre **o comando que sai** — a string de ataque no `sort` volta como a ordenação
padrão, a busca viaja como argumento, o teto de página vale.

## Fora de escopo

- **`--tenant` e `--policy`** — a issue diz que são uma linha em cada arquivo, e são, mas as duas
  dependem de adivinhar como o projeto configurou o `auth`. Não é o que esta spec destrava.
- **`--schema`** (a versão compacta com `ui.SchemaForm`) e o cenário *"adicione um cadastro de
  X"* no `bench/agent` — continuam na issue, e nenhum dos dois depende do que entra aqui.
- **`--store none`** — a interface sem implementação nenhuma. Com a memória sempre escrita, o
  que ela entregaria é um arquivo a menos, e não uma decisão.
- **O store do `--template app`** (`internal/store/store.go`, o `items/`) — é tipo concreto e
  não a interface do gerador; mexer nele é a #117.
- **A coluna de tenant em toda consulta** — como na 126, é trabalho da receita `tenant`.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | O gerado usa `context`, `database/sql`, `crypto/rand` e a receita. Nenhum driver: o `--store` não escreve `driver.go`, e o e2e confere isso rodando `check` sem nenhum `go get`. |
| III — coerente com Go | Contexto como primeiro parâmetro, `error` como último; "não achou" é um erro sentinela com `errors.Is`, e não um `bool` paralelo. |
| VI — teste primeiro | Testes de unidade no `scaffold` antes de cada metade, mais o e2e que gera num projeto com a receita e roda `trilha check`. |
| VII — segurança por padrão | O store SQL não tem caminho para montar SQL com o que veio da URL: ordenação por tabela declarada, teto de página, `LIKE` escapado, todo valor em placeholder. O teste gerado afirma isso no projeto de quem gerou. |

## Tarefas

- [x] T001 Teste que falha: a interface emitida tem `ctx` e `error`, e as telas passam `c.Context()`
- [x] T002 Implementação da primeira metade: interface, memória, três telas, teste gerado
- [x] T003 Teste que falha: `--store sqlite|postgres` escreve o store SQL e a migração; sem a receita, recusa
- [x] T004 Implementação do `--store`: `tipo_store_sql.go`, `migrations/NNNN_*.sql`, o `Provide` no fim do `Setup`
- [x] T005 e2e em `cmd/trilha`: `new` + `add store` + `generate crud --store sqlite` + `check` verde
- [x] T006 Documentação `reference/cli` e `reference/store`, en + pt
- [x] T007 `CHANGELOG.md`, `version`, `ROADMAP.md`
- [x] T008 `make test` verde
- [ ] T009 `scripts/release.sh 0.106.0` — fecha a spec; a #115 continua aberta pelas bandeiras
  que ela ainda lista, então a release não fecha issue nenhuma

## Aceitação

- **SC-001** A interface emitida é a de cima, e nenhuma tela gerada chama um método do store sem
  passar o contexto da requisição.
- **SC-002** `Get` de um id que não existe volta `trilha.ErrNotFound` e a tela responde 404; um
  erro de banco sobe como erro e não como 404.
- **SC-003** `trilha new x && cd x && trilha add store && trilha generate crud docs.Tipo --store
  sqlite && trilha check` fica verde, sem editar nada e sem driver instalado.
- **SC-004** `--store sqlite` num projeto sem `internal/store` recusa nomeando `trilha add store`,
  e não escreve arquivo nenhum.
- **SC-005** Toda string de ataque no `sort` da URL volta como a ordenação padrão, e a busca
  chega ao banco como argumento — afirmado pelo teste que o gerador escreve, sem banco.
- **SC-006** A linha do `Provide` do store SQL está depois do `store.Setup(a)` no `app/setup.go`.
