---
title: store
description: The SQL database as a recipe — pool, dialect, migrations applied once and checked, and the kit that keeps a listing from building SQL out of the URL.
---

Trilha has no driver and no opinion about which database you use: the framework is standard
library only, and it stays that way. What it has is the machinery around `database/sql` that
every application writes again, and gets subtly wrong in the same three places.

```bash
trilha add store
```

That writes `internal/store/` and `migrations/`, and adds one line to `app/setup.go`. Then you
pick a driver — one file, one import:

```go
// internal/store/driver.go
package store

import _ "modernc.org/sqlite" // ou _ "github.com/jackc/pgx/v5/stdlib"
```

One file and one import, so the day you change database it is the only place that knows which
one you were on. `DATABASE_URL` says where; a `postgres://` URL picks the Postgres dialect and
anything else picks SQLite.

## What the four files are for

Three of them exist because of a way applications get broken, not because of convenience.

### `consulta.go` — the listing that does not build SQL

A table needs two things from the URL: which column to order by and which direction. Neither
can be a placeholder, because an identifier is not a value — and so the tempting line is

```go
"ORDER BY " + c.Query("sort") // não
```

which is the whole of injection in one expression. The answer is to not build SQL out of input
at all. What arrives from outside **selects** from a table the code declared:

```go
var docsOrdenaveis = store.Sortable{"nome": "nome", "tamanho": "bytes", "criado": "criado_em"}

q := "SELECT id, nome, bytes FROM documentos WHERE nome LIKE " + store.SQLite.Arg(1) +
	store.OrderBy(docsOrdenaveis, c.Query("sort"), c.Query("dir"), "nome") +
	" LIMIT " + ...
```

A key that is not in the map falls back, silently — a listing that answers 400 to a stale
bookmark punishes the wrong person. The column is checked again on the way out even though it
came from the map, because the map is written by hand and this is the one function that turns
a string into SQL.

`Paginate(limit, offset, def)` bounds what came from the URL: a limit nobody caps is a way to
ask one query to read the whole table. `Like(q)` escapes `%` and `_`, so a person typing `%`
does not ask for every row.

### `migrar.go` — the schema you can reason about

Files in `migrations/`, named `NNNN_what-it-does.sql`, applied in name order, one transaction
each, with the receipt in the same transaction. Beyond a loop over the files it does three
things, and each is a way a schema goes wrong quietly:

- **it records a checksum and compares it.** A migration edited after it was applied means the
  database in front of you is not the one the file describes, and every environment has a
  different one. It is refused by name, telling you to write a new migration instead.
- **it takes a lock.** Two instances of a deploy boot together, both find the same migration
  unapplied, and both run it; on Postgres that is two `CREATE TABLE`s racing and a crashed
  deploy. On SQLite there is one writer by construction and nothing to take.
- **it runs from `Setup`,** before the first request. A deploy that cannot migrate cannot
  serve, which is the failure you want — the alternative is an instance answering against a
  schema it does not have.

### `store.go` — the pool, and the DSN that stays out of the log

`sql.Open` does not connect, so a wrong password surfaces on the first query, usually a
visitor's; the ping moves that to the start of the process, where a deploy can still be rolled
back. The pool has a ceiling, an idle count equal to it, and a connection lifetime — a database
has a connection limit and it is smaller than you think.

And the connection string carries a password. A driver that cannot connect is exactly the
driver that puts the whole string in the error, which is how a password reaches a log
aggregator that many more people can read than can read the environment. Every error out of
this package goes through `redact`.

`NotFound(err)` turns `sql.ErrNoRows` into `trilha.ErrNotFound`: a handler that lets it through
answers 500 to something that deserved a 404.

### `dialeto.go` — the little that differs

`Arg(n)` and `Args(n)` are the placeholders (`$1` or `?`), `Now()` is the UTC clock, `Driver()`
is the name to register under. It is a type and not an `if` scattered through the queries,
because the day a third database appears the compiler says where to look.

## The tests it writes

`trilha add store` writes a fake `database/sql/driver` and the tests over it. The point of a
fake driver is that an assertion can be about **the statement that left**, not about the code
that wrote it: that no value is ever concatenated, that the migration name travels as an
argument, that every attack string put through `OrderBy` comes back as the fallback. It is all
standard library, so proving it costs no dependency.

## The generator writes against this

`trilha generate crud <Type> --store sqlite|postgres` emits the SQL half of a CRUD — the store,
its test and the migration for its table — and it emits it **against this recipe**: the dialect
for the placeholders, `Sortable`/`OrderBy` for the ordering that came from the URL, `Paginate`
for the page ceiling, `Like` for the search box, `NotFound` for `sql.ErrNoRows`. That is what
this recipe is for: a generator cannot own a DDL or pick a placeholder dialect, and here both
already have an owner. See [`trilha generate crud`](/reference/cli#trilha-generate-crud).

The generated store also numbers its migration one past the highest on disk, because name order
is apply order — and without this recipe in the project, the flag is a refusal that names
`trilha add store`.

## What is yours

The schema. This recipe owns how migrations are applied, not what is in them — `migrations/0001_init.sql`
is a placeholder to replace with your first table. And the queries: there is no ORM here, and a
`WHERE` this package wrote would be a `WHERE` nobody could read — with one exception, and it is
the generator above, which writes them into your project for you to read and edit.
