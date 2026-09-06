package cookbook

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"time"

	"github.com/emersonjoe/trilha"
)

// DB is the pool every query in the app shares. One pool per process: a
// pool per request is a connection storm, and a pool per package is four
// ceilings nobody added up.
var DB *sql.DB

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

// Article is one row of the articles table.
type Article struct {
	ID        int64
	Slug      string
	Title     string
	Published time.Time
}

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

//go:embed migrations/*.sql
var migrations embed.FS

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
