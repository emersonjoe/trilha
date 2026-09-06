---
title: Database
description: One pool for the process, queries that carry the request's context, a transaction that rolls back on its own, and migrations applied in order.
---

Trilha does not open your database. What it gives you is the two moments that matter: `Setup`,
which runs once before the server starts, and the request's context, which is what makes a
query stop when the visitor gives up.

## The pool

`database/sql` is already a pool. One per process — a pool per package is four connection
ceilings nobody added up, and a pool per request is a connection storm on the first busy
minute.

```go
// OpenDB opens the pool and proves it works. sql.Open does not connect, so
// a wrong password only shows up on the first query — usually a visitor's.
// The ping moves that failure to the start of the process, where a deploy
// can still be rolled back.
func OpenDB(driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	// The database has a connection limit and it is smaller than you think.
	// Max open is what one instance may hold; idle equal to it keeps the
	// pool from opening and closing a connection per burst.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: %w", driver, err)
	}
	return db, nil
}
```

The `import` that makes `"pgx"` a real name is the one line this file cannot have, because the
repository has no external dependency:

```text
import _ "github.com/jackc/pgx/v5/stdlib"   // driver "pgx"
import _ "modernc.org/sqlite"               // driver "sqlite", no cgo
```

For SQLite there is one more thing to say, and it is not optional: `_pragma=journal_mode(WAL)`
in the DSN, plus `db.SetMaxOpenConns(1)` for writes. Without WAL, the second concurrent write
gets `database is locked`, and it will happen in production and not in your tests.

## Where it is opened

```go
// SetupDB is what app/setup.go does with the pool: open it, hand it to the
// packages that query, tell the health probe about it, and close it on the
// way out.
func SetupDB(a *trilha.App) error {
	db, err := OpenDB("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	DB = db
	a.Check("db", func(ctx context.Context) error { return db.PingContext(ctx) })
	a.OnShutdown(func(*trilha.App) error { return db.Close() })
	return nil
}
```

Three things in six lines, and the last two are the ones people forget. `a.Check` makes the
pool part of `/_trilha/health/ready`, so an instance that lost the database stops receiving
traffic instead of answering 500 to everyone. `a.OnShutdown` closes it after the last request,
not during it.

## Reading

```go
// ArticleBySlug reads one row. sql.ErrNoRows is not a failure of the
// server: it is the page not existing, and a handler that lets it through
// answers 500 to something that deserved a 404.
func ArticleBySlug(ctx context.Context, slug string) (Article, error) {
	var a Article
	err := DB.QueryRowContext(ctx, `SELECT id, slug, title, published_at FROM articles WHERE slug = $1`, slug).
		Scan(&a.ID, &a.Slug, &a.Title, &a.Published)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Article{}, trilha.ErrNotFound
	case err != nil:
		return Article{}, fmt.Errorf("article %q: %w", slug, err)
	}
	return a, nil
}
```

`sql.ErrNoRows` is the most common bug in this file. It is not a failure of the server: it is
the page not existing. Returning `trilha.ErrNotFound` turns it into the 404 the visitor
deserves — and, for an `/api` route, into a `problem+json` body with the right status.

A list is the same, with the rows closed by a `defer` and `rows.Err()` checked at the end,
because a broken connection halfway through looks exactly like the end of the list:

```go
// Articles reads a list. The context is the request's: when the visitor
// gives up, the query is cancelled instead of holding a connection for a
// page nobody will read.
func Articles(ctx context.Context, limit int) ([]Article, error) {
	rows, err := DB.QueryContext(ctx, `SELECT id, slug, title, published_at FROM articles ORDER BY published_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
```

:::warning
The context comes from `c.Context()`, always. A query started with `context.Background()`
inside a handler keeps running after the visitor closes the tab, holding a connection for an
answer nobody will read.
:::

## Writing, and undoing

```go
// InTx runs fn inside a transaction. The rollback is deferred without a
// condition because rolling back a committed transaction does nothing: that
// is what keeps a panic in the middle from leaving the transaction open.
func InTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
```

The unconditional `defer tx.Rollback()` is the point: rolling back a transaction that already
committed does nothing, so the deferred call is free in the happy path and is the only thing
that closes the transaction when the code between panics.

## Migrations

A migration tool is a fine choice. It is also a dependency, a binary in the image and a step in
the deploy — and the whole thing is thirty lines with `embed`:

```go
// Migrate applies every file in migrations/ the database has not seen, in
// name order, each one with its record in the same transaction. Either the
// migration and its receipt land together or neither does.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL)`); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		var applied int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE name = $1`, name).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		err = InTx(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (name, applied_at) VALUES ($1, $2)`, name, time.Now().UTC())
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
```

The file and its receipt land in the same transaction: either both or neither. Call it from
`Setup` before the server listens, or from a separate command if your deploy applies
migrations before rolling the new version — which is the better shape once there is more than
one instance.

## sqlc

Everything above writes the `Scan` by hand. [sqlc](https://sqlc.dev) generates it from the SQL
you already wrote: a `.sql` file in, a typed method out, and a compile error when a column
changes name.

```yaml
version: "2"
sql:
  - engine: postgresql
    queries: internal/db/query.sql
    schema: internal/db/migrations
    gen:
      go:
        package: db
        out: internal/db
```

It fits the framework without any adapter, because what comes out is ordinary Go: the
generated `*db.Queries` goes in `Setup` exactly where `DB` goes above. The trade is the
generator in the loop — one more command to run when the SQL changes, and one more thing to
explain to whoever joins.

:::note
sqlc runs at build time, so it is not a runtime dependency of your app. That distinction is
what makes it a different decision from adding an ORM.
:::
