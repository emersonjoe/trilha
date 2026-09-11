# Spec 126 — O store em SQL: a decisão que a #115 e a #117 esperavam

- **Issues**: [#115](https://github.com/emersonjoe/trilha/issues/115) e
  [#117](https://github.com/emersonjoe/trilha/issues/117) — as issues são a fonte do escopo.
  Nenhuma das duas fecha aqui; o que fecha é o que travava as duas.
- **Branch**: `claude/relaxed-keller-v1pwuf`
- **Versão**: 0.105.0

## Por quê

As duas issues pararam no mesmo lugar, e o comentário da #115 nomeou por quê:

> `--store sqlite|postgres` e o `migrations/NNN_*.sql`. Um store SQL gerado teria de escolher
> dialeto de placeholder e ser dono de um DDL — a coisa que este framework não faz em nenhum
> outro lugar. É uma decisão separada e maior que esta.

Enquanto ela não fosse tomada, nenhuma das duas andava: a #115 não pode emitir um store sem
saber contra o quê, e a #117 não pode ter `migrations/001_init.sql` sem quem as aplique.

## O que muda

**A decisão, e ela já estava meio tomada neste repositório.** O comentário da 0.95.0, sobre o
`Search`, diz: *"store é interface mais memória e o SQL mora na receita, como no `Versioned` e
no `approval` — um driver dentro de um pacote com zero dependências não é uma decisão que este
repositório pode tomar"*. A spec 126 leva isso um passo adiante: **a receita passa a ser dona
do dialeto, da convenção de DDL e do runner**, para que um gerador possa emitir um store contra
eles. O framework continua biblioteca padrão; o projeto escolhe o driver, em um arquivo:

```go
// internal/store/driver.go
package store

import _ "modernc.org/sqlite" // ou _ "github.com/jackc/pgx/v5/stdlib"
```

**`trilha add store`** escreve `internal/store/` (quatro arquivos e os testes) e `migrations/`,
e acrescenta uma linha ao `app/setup.go`. Três dos quatro arquivos existem por causa de um jeito
de quebrar aplicação, e não por conveniência:

| Arquivo | O que ele impede | Referência |
|---|---|---|
| `consulta.go` | o `ORDER BY` montado com o que veio na URL; o limite sem teto; o `%` da caixa de busca | OWASP A03 (injeção), ASVS V5.3.4; A04 quanto ao teto |
| `migrar.go` | a migração editada depois de aplicada; duas réplicas migrando ao mesmo tempo | NIST SP 800-53 CM-3 (controle de mudança de baseline) |
| `store.go` | o DSN — e a senha dentro dele — chegando a um log | OWASP A09; NIST SP 800-53 AU-9 |

O `OrderBy` é o centro: o que chega de fora **escolhe** de uma tabela que o código declarou, e
nada digitado é concatenado. A coluna é conferida na saída mesmo tendo vindo do mapa — um
guarda que só confia em quem o chama é um guarda que funciona até alguém montar o mapa a partir
de um arquivo de configuração.

**Os testes que a receita escreve rodam contra um `database/sql/driver` de mentira.** É a parte
que torna as afirmações acima verificáveis em vez de declaradas: a asserção é sobre **o comando
que saiu** — que nenhum valor foi concatenado, que o nome do arquivo de migração viajou como
argumento, que toda string de ataque passada pelo `OrderBy` volta como o padrão. Tudo
biblioteca padrão, então provar não custa dependência.

## Fora de escopo

- **`trilha generate crud --store sqlite|postgres` (#115) e `--with store` no template (#117).**
  São o que as duas issues chamam de pequeno depois que a base existir, e agora ela existe. Uma
  observação para quem for fazer: a interface que o `generate crud` emite hoje é
  `List(q Query)`, sem `context.Context`. Emitir SQL contra ela repetiria a
  [#165](https://github.com/emersonjoe/trilha/issues/165) — o contexto tem de entrar na
  interface antes.
- **A coluna de tenant em toda consulta.** É OWASP A01 e é a mais importante depois da injeção,
  mas amarrar duas receitas é o que o `Insert.If` faz, e é trabalho da receita `tenant`, não
  desta.
- **`SessionLister` com contexto** — [#176](https://github.com/emersonjoe/trilha/issues/176).

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | `database/sql`, `crypto/sha256`, `embed`. Nenhum driver, aqui ou no que a receita escreve: o `Next` manda o projeto rodar o `go get`, e o e2e confere que a receita **não** escreveu um `driver.go`. |
| III — coerente com Go | `Dialect` é um tipo e não um `if` espalhado; o runner recebe `fs.FS`; o teste usa `database/sql/driver` e `testing/fstest`. |
| VI — teste primeiro | Nove testes na receita, contra um driver falso, mais o e2e que gera o projeto, roda o `check` e nomeia os cinco que não podem sumir. |
| VII — segurança por padrão | A tabela acima. E o padrão é o seguro nos três: sem `DATABASE_URL` o app recusa subir em vez de correr sem banco; migração alterada é recusa e não aviso; o teto do `Paginate` vale sem ninguém pedir. |

## Tarefas

- [x] T001 A receita `store`: pool, dialeto, kit de consulta, runner
- [x] T002 Os testes que ela escreve, sobre um `database/sql/driver` falso
- [x] T003 `migrations/` como pacote — o `go:embed` não atravessa `..`
- [x] T004 e2e em `cmd/trilha`: gera, aplica, `check` verde, os cinco testes nomeados
- [x] T005 Referência `/reference/store` en + pt, no menu das duas
- [x] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`
- [x] T007 `make test` verde e `scripts/release.sh 0.105.0`

## Aceitação

- **SC-001** `trilha new x && cd x && trilha add store && trilha check` fica verde sem ninguém
  editar nada, e sem driver nenhum instalado.
- **SC-002** Toda string de ataque passada ao `OrderBy` — `; DROP TABLE`, `--`, `' OR '1'='1`,
  subconsulta — volta como a ordenação padrão, e uma tabela de colunas mal escrita não produz
  cláusula nenhuma.
- **SC-003** Uma migração editada depois de aplicada é recusada por nome, dizendo o checksum
  gravado e o do disco, e nada é aplicado por cima.
- **SC-004** Nenhum comando que este pacote monta leva valor dentro do texto: o driver falso vê
  o nome do arquivo e o checksum como argumento.
- **SC-005** Um erro de conexão não carrega o DSN nem a senha, e ainda diz o motivo.
- **SC-006** A receita não escolhe driver: não há `internal/store/driver.go` depois dela, e o
  próximo passo diz qual `go get` rodar.
