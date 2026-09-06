---
title: Banco de dados
description: Um pool para o processo, consultas que carregam o contexto da requisição, uma transação que desfaz sozinha e migrações aplicadas em ordem.
---

Trilha não abre o seu banco. O que ele dá são os dois momentos que importam: o `Setup`, que
roda uma vez antes de o servidor subir, e o contexto da requisição, que é o que faz uma
consulta parar quando o visitante desiste.

## O pool

`database/sql` já é um pool. Um por processo — um pool por pacote é um teto de conexões que
ninguém somou, e um pool por requisição é uma tempestade de conexões no primeiro minuto
movimentado.

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

O `import` que faz de `"pgx"` um nome de verdade é a única linha que este arquivo não pode
ter, porque o repositório não tem dependência externa:

```text
import _ "github.com/jackc/pgx/v5/stdlib"   // driver "pgx"
import _ "modernc.org/sqlite"               // driver "sqlite", sem cgo
```

Para SQLite há mais uma coisa a dizer, e ela não é opcional: `_pragma=journal_mode(WAL)` no
DSN, mais `db.SetMaxOpenConns(1)` para escrita. Sem WAL, a segunda escrita concorrente recebe
`database is locked` — e isso vai acontecer em produção, não nos seus testes.

## Onde ele é aberto

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

Três coisas em seis linhas, e as duas últimas são as que se esquecem. `a.Check` coloca o pool
dentro de `/_trilha/health/ready`, então uma instância que perdeu o banco para de receber
tráfego em vez de responder 500 para todo mundo. `a.OnShutdown` fecha o pool depois da última
requisição, não durante ela.

## Lendo

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

`sql.ErrNoRows` é o bug mais comum deste arquivo. Ele não é uma falha do servidor: é a página
não existir. Devolver `trilha.ErrNotFound` transforma isso no 404 que o visitante merece — e,
numa rota `/api`, num corpo `problem+json` com o status certo.

Uma lista é a mesma coisa, com as linhas fechadas por um `defer` e o `rows.Err()` conferido no
fim, porque uma conexão que caiu no meio se parece exatamente com o fim da lista:

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

:::atencao
O contexto vem de `c.Context()`, sempre. Uma consulta iniciada com `context.Background()`
dentro de um handler continua rodando depois que o visitante fecha a aba, segurando uma
conexão por uma resposta que ninguém vai ler.
:::

## Escrevendo, e desfazendo

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

O `defer tx.Rollback()` sem condição é o ponto: desfazer uma transação que já foi confirmada
não faz nada, então a chamada adiada é de graça no caminho feliz e é a única coisa que fecha a
transação quando o código do meio entra em pânico.

## Migrações

Uma ferramenta de migração é uma escolha boa. Ela também é uma dependência, um binário na
imagem e um passo no deploy — e a coisa toda são trinta linhas com `embed`:

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

O arquivo e o recibo dele caem na mesma transação: ou os dois, ou nenhum. Chame isso do
`Setup` antes de o servidor escutar, ou de um comando separado se o seu deploy aplica as
migrações antes de subir a versão nova — que é o formato melhor a partir da segunda instância.

## sqlc

Tudo acima escreve o `Scan` à mão. O [sqlc](https://sqlc.dev) gera esse código a partir do SQL
que você já escreveu: entra um arquivo `.sql`, sai um método tipado, e uma coluna que muda de
nome vira erro de compilação.

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

Ele encaixa no framework sem adaptador nenhum, porque o que sai é Go comum: o `*db.Queries`
gerado vai no `Setup` exatamente onde o `DB` vai acima. A troca é o gerador no ciclo — mais um
comando para rodar quando o SQL muda, e mais uma coisa para explicar a quem chega.

:::nota
O sqlc roda em tempo de build, então ele não é dependência de execução do seu app. É essa
distinção que faz dele uma decisão diferente de acrescentar um ORM.
:::
