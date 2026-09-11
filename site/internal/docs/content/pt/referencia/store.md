---
title: store
description: O banco em SQL como receita — pool, dialeto, migrações aplicadas uma vez e conferidas, e o kit que impede uma listagem de montar SQL com o que veio na URL.
---

O Trilha não tem driver e não tem opinião sobre qual banco você usa: o framework é biblioteca
padrão e continua sendo. O que ele tem é a máquina em volta do `database/sql` que toda
aplicação escreve de novo, e erra sutilmente nos mesmos três lugares.

```bash
trilha add store
```

Isso escreve `internal/store/` e `migrations/`, e acrescenta uma linha ao `app/setup.go`. Depois
você escolhe um driver — um arquivo, um import:

```go
// internal/store/driver.go
package store

import _ "modernc.org/sqlite" // ou _ "github.com/jackc/pgx/v5/stdlib"
```

Um arquivo e um import, para que no dia de trocar de banco esse seja o único lugar que sabe em
qual você estava. O `DATABASE_URL` diz onde; uma URL `postgres://` escolhe o dialeto do
Postgres e qualquer outra coisa escolhe o SQLite.

## Para que servem os quatro arquivos

Três deles existem por causa de um jeito de quebrar aplicação, e não por conveniência.

### `consulta.go` — a listagem que não monta SQL

Uma tabela precisa de duas coisas da URL: por qual coluna ordenar e em que direção. Nenhuma das
duas pode ser um placeholder, porque identificador não é valor — e aí a linha tentadora é

```go
"ORDER BY " + c.Query("sort") // não
```

que é a injeção inteira numa expressão. A resposta é não montar SQL com o que vem de fora. O
que chega **escolhe** de uma tabela que o código declarou:

```go
var docsOrdenaveis = store.Sortable{"nome": "nome", "tamanho": "bytes", "criado": "criado_em"}

q := "SELECT id, nome, bytes FROM documentos WHERE nome LIKE " + store.SQLite.Arg(1) +
	store.OrderBy(docsOrdenaveis, c.Query("sort"), c.Query("dir"), "nome") +
	" LIMIT " + ...
```

Uma chave que não está no mapa cai no padrão, em silêncio — uma listagem que responde 400 a um
favorito velho castiga a pessoa errada. A coluna é conferida na saída mesmo tendo vindo do
mapa, porque o mapa é escrito à mão e esta é a única função que transforma string em SQL.

O `Paginate(limite, offset, padrão)` limita o que veio da URL: um limite sem teto é o jeito de
pedir a tabela inteira numa consulta. O `Like(q)` escapa `%` e `_`, para quem digitar `%` não
pedir todas as linhas.

### `migrar.go` — o schema sobre o qual dá para raciocinar

Arquivos em `migrations/`, com nome `NNNN_o-que-faz.sql`, aplicados em ordem de nome, um por
transação, com o recibo na mesma transação. Além de um laço sobre os arquivos ele faz três
coisas, e cada uma é um jeito de o schema dar errado quieto:

- **grava um checksum e compara.** Uma migração editada depois de aplicada quer dizer que o
  banco à sua frente não é o que o arquivo descreve, e cada ambiente tem um. É recusada por
  nome, mandando escrever a próxima em vez de editar aquela.
- **toma um lock.** Duas instâncias de um deploy sobem juntas, as duas encontram a mesma
  migração pendente e as duas rodam; no Postgres isso é dois `CREATE TABLE` correndo e um
  deploy quebrado. No SQLite há um escritor por construção e não há o que tomar.
- **roda no `Setup`,** antes da primeira requisição. Um deploy que não consegue migrar não
  serve, que é a falha desejável — a alternativa é uma instância respondendo contra um schema
  que ela não tem.

### `store.go` — o pool, e o DSN que não vai para o log

O `sql.Open` não conecta, então uma senha errada aparece na primeira consulta, normalmente a de
um visitante; o ping leva essa falha para o começo do processo, onde um deploy ainda pode
voltar atrás. O pool tem teto, ociosas iguais ao teto e tempo de vida — banco tem limite de
conexões, e ele é menor do que se imagina.

E a string de conexão carrega uma senha. O driver que não consegue conectar é exatamente o que
põe a string inteira na mensagem, que é como uma senha chega a um agregador de logs que muito
mais gente lê do que lê o ambiente. Todo erro que sai deste pacote passa pelo `redact`.

O `NotFound(err)` transforma `sql.ErrNoRows` em `trilha.ErrNotFound`: um handler que o deixa
passar responde 500 a algo que merecia 404.

### `dialeto.go` — o pouco que difere

`Arg(n)` e `Args(n)` são os placeholders (`$1` ou `?`), `Now()` é o relógio em UTC, `Driver()` é
o nome sob o qual registrar. É um tipo e não um `if` espalhado pelas consultas, porque no dia
em que aparecer um terceiro banco o compilador diz onde olhar.

## Os testes que ela escreve

O `trilha add store` escreve um `database/sql/driver` de mentira e os testes em cima dele. A
graça de um driver falso é que a asserção pode ser sobre **o comando que saiu**, e não sobre o
código que o escreveu: que nenhum valor é concatenado, que o nome da migração viaja como
argumento, que toda string de ataque passada pelo `OrderBy` volta como o padrão. É tudo
biblioteca padrão, então provar isso não custa dependência.

## O que é seu

O schema. Esta receita é dona de como as migrações são aplicadas, não do que há dentro delas —
o `migrations/0001_init.sql` é um lugar para a sua primeira tabela. E as consultas: não há ORM
aqui, e um `WHERE` que este pacote escrevesse seria um `WHERE` que ninguém consegue ler.
