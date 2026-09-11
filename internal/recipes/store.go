package recipes

// storeRecipe is the decision both #115 and #117 were waiting on: what it
// means for this framework to own SQL without owning a driver.
//
// The answer is the one the rest of the repository already gives — the store
// is an interface plus memory, and the SQL lives in a recipe — carried one
// step further: the recipe owns the *dialect*, the *DDL convention* and the
// *runner*, so that a generator can emit a store against them. The framework
// stays standard library only; the project chooses the driver and says so in
// one line.
//
// What the machinery is for is not convenience. Three of the four files exist
// because of a way applications get broken:
//
//   - query.go — the ORDER BY that comes from the URL is the injection every
//     listing screen invites (OWASP A03). It is answered by not building SQL
//     from input at all: a key selects a column from a table the code wrote.
//   - migrate.go — a migration that changed after it was applied is a schema
//     nobody can reason about, and two replicas booting together race on the
//     same DDL (NIST SP 800-53 CM-3, change control). Both are answered here.
//   - store.go — a DSN carries a password, and a driver error carries the DSN
//     (OWASP A09). It never reaches a log.
func storeRecipe() Recipe {
	return Recipe{
		Name: "store",
		Summary: map[string]string{
			"en": "the SQL database: pool, dialect, migrations applied once and checked, and the kit that keeps a listing from building SQL out of the URL",
			"pt": "o banco em SQL: pool, dialeto, migrações aplicadas uma vez e conferidas, e o kit que impede uma listagem de montar SQL com o que veio na URL",
		},
		Doc: "/reference/store",
		Files: []File{
			{Rel: "internal/store/store.go", Go: true, Body: storePool},
			{Rel: "internal/store/dialeto.go", Go: true, Body: storeDialect},
			{Rel: "internal/store/consulta.go", Go: true, Body: storeQuery},
			{Rel: "internal/store/migrar.go", Go: true, Body: storeMigrate},
			{Rel: "internal/store/consulta_test.go", Go: true, Body: storeQueryTest},
			{Rel: "internal/store/falso_test.go", Go: true, Body: storeFakeDriver},
			{Rel: "internal/store/migrar_test.go", Go: true, Body: storeMigrateTest},
			{Rel: "migrations/migrations.go", Go: true, Body: storeMigrationsPkg},
			{Rel: "migrations/0001_init.sql", Body: storeFirstMigration},
		},
		Setup: []Insert{{
			Marker: "// trilha:add store",
			Line:   "\tif err := store.Setup(a); err != nil {\n\t\treturn err\n\t}\n",
		}},
		Imports: []string{"{{.Module}}/internal/store"},
		Next: map[string]string{
			"en": "Pick a driver and import it in internal/store/driver.go — `go get modernc.org/sqlite` (no cgo) or `go get github.com/jackc/pgx/v5`, then a blank import. Set DATABASE_URL, write your first table in migrations/0001_init.sql, and `trilha dev` applies it on boot.",
			"pt": "Escolha um driver e importe-o em internal/store/driver.go — `go get modernc.org/sqlite` (sem cgo) ou `go get github.com/jackc/pgx/v5`, e um import em branco. Defina DATABASE_URL, escreva a sua primeira tabela em migrations/0001_init.sql, e o `trilha dev` a aplica ao subir.",
		},
	}
}

const storePool = `// Package store is this application's database: one pool, one dialect, and
// the migrations that build the schema.
//
// The framework has no driver and no opinion about which one you use — it is
// standard library only, and this file is ` + "`database/sql`" + `. Choosing the driver
// is one line of yours, in a file of its own:
//
//	// internal/store/driver.go
//	package store
//
//	import _ "modernc.org/sqlite" // ou _ "github.com/jackc/pgx/v5/stdlib"
//
// One file, one import: the day you change database, it is the only place
// that knows which one you were on.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"

	"{{.Module}}/migrations"
)

// DB is the pool every query shares. One per process: a pool per request is a
// connection storm, and a pool per package is four ceilings nobody added up.
var DB *sql.DB

// D is the dialect DB speaks, decided from the DSN. Queries that differ
// between databases ask it instead of guessing.
var D Dialect

// Setup opens the pool, applies the migrations and hands the health probe a
// way to answer. It is what app/setup.go calls.
//
// Migrating on boot is a decision worth naming: it means a deploy that cannot
// migrate cannot serve, which is the failure you want — the alternative is an
// instance answering requests against a schema it does not have.
func Setup(a *trilha.App) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("store: DATABASE_URL is not set")
	}
	db, err := Open(dsn)
	if err != nil {
		return err
	}
	DB, D = db, DialectOf(dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := Migrate(ctx, db, D, migrations.FS); err != nil {
		db.Close()
		return err
	}
	// Readiness, not liveness: an instance whose database is gone should stop
	// receiving traffic and not be restarted, because restarting it fixes
	// nothing.
	a.Check("db", func(ctx context.Context) error { return db.PingContext(ctx) })
	a.OnShutdown(func(*trilha.App) error { return db.Close() })
	return nil
}

// Open opens the pool and proves it works. sql.Open does not connect, so a
// wrong password would otherwise surface on the first query — usually a
// visitor's. The ping moves that failure to the start of the process, where a
// deploy can still be rolled back.
func Open(dsn string) (*sql.DB, error) {
	d := DialectOf(dsn)
	db, err := sql.Open(d.Driver(), dsn)
	if err != nil {
		// "unknown driver" is the one error with a known fix, and the message
		// is the fix.
		if strings.Contains(err.Error(), "unknown driver") {
			return nil, fmt.Errorf("store: no %s driver is registered. Add internal/store/driver.go with %s", d.Driver(), d.GoGet())
		}
		return nil, fmt.Errorf("store: %w", redact(err, dsn))
	}
	// The database has a connection limit and it is smaller than you think.
	// Max open is what one instance may hold; idle equal to it keeps the pool
	// from opening and closing a connection per burst. A lifetime bounds how
	// long a connection can outlive a failover.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: %w", redact(err, dsn))
	}
	return db, nil
}

// redact takes the DSN out of an error before anybody logs it. A connection
// string carries the password, and a driver that cannot connect is exactly
// the driver that puts the whole string in the message — which is how a
// password ends up in a log aggregator that a lot more people can read than
// can read the environment.
func redact(err error, dsn string) error {
	if err == nil || dsn == "" {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), dsn, "$DATABASE_URL")
	// The password on its own too: some drivers print the parsed parts rather
	// than the string they were given.
	if i := strings.Index(dsn, "://"); i >= 0 {
		rest := dsn[i+3:]
		if at := strings.IndexByte(rest, '@'); at > 0 {
			if colon := strings.IndexByte(rest[:at], ':'); colon >= 0 {
				if senha := rest[colon+1 : at]; senha != "" {
					msg = strings.ReplaceAll(msg, senha, "xxxxx")
				}
			}
		}
	}
	return errors.New(msg)
}

// InTx runs fn inside a transaction. The rollback is deferred without a
// condition because rolling back a committed transaction does nothing — which
// is what keeps a panic in the middle from leaving the transaction open.
func InTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // ver acima
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// NotFound turns the one database error that is not a server failure into the
// framework's. A handler that lets sql.ErrNoRows through answers 500 to
// something that deserved a 404.
func NotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return trilha.ErrNotFound
	}
	return err
}
`

const storeDialect = `package store

import "strings"

// Dialect is the little that differs between the two databases this store
// speaks. It is a type and not an if scattered through the queries, because
// the day a third one appears the compiler says where to look.
type Dialect int

// The dialects.
const (
	SQLite Dialect = iota
	Postgres
)

// DialectOf reads the DSN. Anything that is not a Postgres URL is SQLite,
// which is the default a project starts on.
func DialectOf(dsn string) Dialect {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return Postgres
	}
	return SQLite
}

// Driver is the name to register the driver under. It is the name the usual
// driver of each database registers itself as.
func (d Dialect) Driver() string {
	if d == Postgres {
		return "pgx"
	}
	return "sqlite"
}

// GoGet is the line that fixes a missing driver, printed by Open.
func (d Dialect) GoGet() string {
	if d == Postgres {
		return "go get github.com/jackc/pgx/v5 and _ \"github.com/jackc/pgx/v5/stdlib\""
	}
	return "go get modernc.org/sqlite and _ \"modernc.org/sqlite\""
}

// Arg is the placeholder for the n-th argument, one-based. It exists so a
// query is written once: Postgres numbers them and SQLite does not.
//
// It is also the only way an argument ever reaches a query in this package.
// A value concatenated into SQL is the injection; a placeholder is the value
// travelling beside the statement, where the database cannot mistake it for
// code.
func (d Dialect) Arg(n int) string {
	if d == Postgres {
		return "$" + itoa(n)
	}
	return "?"
}

// Args is Arg for a run of them: "$1, $2, $3" or "?, ?, ?".
func (d Dialect) Args(n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = d.Arg(i + 1)
	}
	return strings.Join(out, ", ")
}

// Now is the clock the schema uses. Both are UTC on purpose: a timestamp
// whose zone depends on the server is a timestamp that changes meaning when
// the server moves.
func (d Dialect) Now() string {
	if d == Postgres {
		return "now() at time zone 'utc'"
	}
	return "strftime('%Y-%m-%d %H:%M:%f', 'now')"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
`

const storeQuery = `package store

// The listing screen is where an application invites SQL injection, and it
// does it through the two things a table needs: which column to order by and
// which direction. Both come from the URL, neither can be a placeholder —
// an identifier is not a value — and so the tempting line is
//
//	"ORDER BY " + c.Query("sort")
//
// which is the whole of OWASP A03 in one expression.
//
// What follows is the answer, and the answer is to not build SQL out of input
// at all: what arrives from outside *selects* from a table the code declared,
// and anything that does not select falls back. Nothing a person types is
// ever concatenated.

import "strings"

// Sortable maps the keys a URL may use to the columns they mean. The keys are
// the application's vocabulary and the values are its schema, which is why
// they are declared next to the query and not derived from it — a column
// renamed in a migration should break the build, not silently start ordering
// by something else.
//
//	var docsSortable = store.Sortable{"nome": "nome", "tamanho": "bytes"}
type Sortable map[string]string

// OrderBy is the ORDER BY clause for a sort key and a direction that came
// from outside. The key must be in allowed; the direction must be one of two
// words; anything else is the fallback, silently, because a listing that
// answers 400 to a stale bookmark is a listing that punishes the wrong
// person.
//
// The column is checked again on the way out even though it came from the
// map: the map is written by hand, and this is the one function that turns a
// string into SQL. A guard that only trusts its caller is a guard that works
// until somebody builds the map from a config file.
func OrderBy(allowed Sortable, key, dir, fallback string) string {
	col, ok := allowed[key]
	if !ok || !isIdent(col) {
		col, ok = allowed[fallback]
		if !ok || !isIdent(col) {
			return ""
		}
	}
	if strings.EqualFold(dir, "desc") {
		return " ORDER BY " + col + " DESC"
	}
	return " ORDER BY " + col + " ASC"
}

// isIdent reports whether s is a plain unquoted identifier: lower-case
// letters, digits and underscore, not starting with a digit. It is narrower
// than what SQL allows on purpose — a column that needs quoting is a column
// this helper refuses to build a clause for, and refusing is cheaper than
// getting the quoting right in two dialects.
func isIdent(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// Page is a page of a listing, with the limit already bounded. An offset and
// a limit that come from the URL are a way to ask one query to read the whole
// table, which is a denial of service written by the visitor — the cap is the
// answer, and it is here rather than in each query so there is one number to
// change.
type Page struct {
	Limit  int
	Offset int
}

// MaxLimit is the most rows one page may ask for.
const MaxLimit = 200

// Paginate bounds what came from outside. A limit of zero or less is the
// default; more than MaxLimit is MaxLimit; a negative offset is zero.
func Paginate(limit, offset, def int) Page {
	if def <= 0 || def > MaxLimit {
		def = 20
	}
	if limit <= 0 {
		limit = def
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return Page{Limit: limit, Offset: offset}
}

// Like escapes a search box for a LIKE pattern. Without it a person typing %
// asks for every row, and a person typing _ asks for something they did not
// mean — and both of those are the query doing something the caller did not
// write.
func Like(q string) string {
	r := strings.NewReplacer("\\\\", "\\\\\\\\", "%", "\\\\%", "_", "\\\\_")
	return "%" + r.Replace(q) + "%"
}
`

const storeMigrate = `package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"
)

// Migrate applies every file the database has not seen, in name order, each
// one inside a transaction with its own receipt. Either the migration and the
// record of it land together or neither does.
//
// Three things it does that a loop over the files does not, and each one is a
// way a schema goes wrong quietly:
//
//   - it records a checksum and compares it. A migration edited after it was
//     applied means the schema in front of you is not the schema the file
//     describes, and every environment has a different one. It is refused by
//     name (NIST SP 800-53 CM-3: a change to a baseline is a change you can
//     see).
//   - it takes a lock. Two instances of a deploy boot at the same time, both
//     find the same migration unapplied, and both run it; on Postgres that is
//     two CREATE TABLEs racing, and the loser crashes the deploy.
//   - it runs before the first request, from Setup, so an instance never
//     answers against a schema it does not have.
func Migrate(ctx context.Context, db *sql.DB, d Dialect, files fs.FS) error {
	if err := lock(ctx, db, d); err != nil {
		return err
	}
	defer unlock(context.WithoutCancel(ctx), db, d)

	if _, err := db.ExecContext(ctx, ` + "`" + `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		checksum TEXT NOT NULL,
		applied_at TIMESTAMP NOT NULL
	)` + "`" + `); err != nil {
		return fmt.Errorf("store: schema_migrations: %w", err)
	}
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	// Name order is apply order, which is why the names are numbered. Two
	// migrations written the same day by two people sort by their number and
	// not by when they were merged.
	sort.Strings(names)
	for _, name := range names {
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])

		var gravado string
		err = db.QueryRowContext(ctx,
			"SELECT checksum FROM schema_migrations WHERE name = "+d.Arg(1), name).Scan(&gravado)
		switch {
		case err == nil && gravado == hash:
			continue // já aplicada, e igual ao que está no disco
		case err == nil:
			return fmt.Errorf("store: %s changed after it was applied (recorded %s, on disk %s): write a new migration instead of editing this one",
				name, gravado[:12], hash[:12])
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("store: %s: %w", name, err)
		}
		if err := InTx(ctx, db, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return fmt.Errorf("store: %s: %w", name, err)
			}
			_, err := tx.ExecContext(ctx,
				"INSERT INTO schema_migrations (name, checksum, applied_at) VALUES ("+d.Args(3)+")",
				name, hash, time.Now().UTC())
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// lockKey is an arbitrary constant that identifies this application's
// migration lock. Two different applications on the same Postgres take
// different keys; two instances of this one take the same.
const lockKey = 8004267731

// lock keeps two instances from migrating at once. On Postgres it is an
// advisory lock, which the database releases if the instance dies holding it.
// On SQLite there is one writer by construction, so there is nothing to take.
func lock(ctx context.Context, db *sql.DB, d Dialect) error {
	if d != Postgres {
		return nil
	}
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return fmt.Errorf("store: migration lock: %w", err)
	}
	return nil
}

func unlock(ctx context.Context, db *sql.DB, d Dialect) {
	if d != Postgres {
		return
	}
	_, _ = db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
}
`

const storeMigrationsPkg = `// Package migrations is the schema, in files, in the binary.
//
// It is a package and not a folder because go:embed cannot look upwards: the
// files have to be embedded from beside them. Embedding them at all is what
// makes the deployed artifact carry its own schema — there is no step where
// somebody has to remember to copy .sql files to a server.
package migrations

import "embed"

// FS is every migration, read by store.Migrate in name order.
//
//go:embed *.sql
var FS embed.FS
`

const storeFirstMigration = `-- A primeira migração.
--
-- O nome manda: NNNN_o-que-faz.sql. Elas são aplicadas em ordem de nome, uma
-- vez cada, dentro de uma transação, e o conteúdo é conferido depois — editar
-- uma que já rodou é recusado por nome, porque o banco à sua frente deixaria
-- de ser o que o arquivo descreve.
--
-- Para mudar algo, escreva a próxima. Nunca esta.
--
-- Troque a tabela abaixo pela sua primeira de verdade.

CREATE TABLE IF NOT EXISTS exemplo (
	id         TEXT PRIMARY KEY,
	nome       TEXT NOT NULL,
	criado_em  TIMESTAMP NOT NULL
);
`

const storeFakeDriver = `package store

// A driver of nothing, which is what lets a test read the SQL this package
// builds. It records every statement and every argument, so an assertion can
// be about the query that left — not about the code that wrote it.
//
// It is database/sql/driver, which is the standard library: proving that no
// value is ever concatenated into a statement costs no dependency.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
)

// chamada is one statement as the database saw it.
type chamada struct {
	Query string
	Args  []any
}

type espiao struct {
	mu    sync.Mutex
	feito []chamada
	// linhas é o que o próximo QueryContext devolve, por consulta contida.
	linhas map[string][][]driver.Value
	erro   error
}

func (e *espiao) grava(q string, args []driver.NamedValue) {
	vals := make([]any, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.feito = append(e.feito, chamada{Query: q, Args: vals})
}

func (e *espiao) chamadas() []chamada {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]chamada(nil), e.feito...)
}

func (e *espiao) Open(string) (driver.Conn, error) { return &conexao{e: e}, nil }

type conexao struct{ e *espiao }

func (c *conexao) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *conexao) Close() error                        { return nil }
func (c *conexao) Begin() (driver.Tx, error)           { return transacao{}, nil }

func (c *conexao) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return transacao{}, nil
}

func (c *conexao) Ping(context.Context) error { return c.e.erro }

func (c *conexao) ExecContext(_ context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.e.grava(q, args)
	if c.e.erro != nil {
		return nil, c.e.erro
	}
	return driver.RowsAffected(1), nil
}

func (c *conexao) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.e.grava(q, args)
	if c.e.erro != nil {
		return nil, c.e.erro
	}
	for trecho, linhas := range c.e.linhas {
		if trecho != "" && strings.Contains(q, trecho) {
			return &resultado{vals: linhas}, nil
		}
	}
	return &resultado{}, nil
}

type transacao struct{}

func (transacao) Commit() error   { return nil }
func (transacao) Rollback() error { return nil }

type resultado struct {
	vals [][]driver.Value
	i    int
}

func (r *resultado) Columns() []string { return []string{"c0"} }
func (r *resultado) Close() error      { return nil }

func (r *resultado) Next(dest []driver.Value) error {
	if r.i >= len(r.vals) {
		return io.EOF
	}
	copy(dest, r.vals[r.i])
	r.i++
	return nil
}

// bancoFalso is a *sql.DB over the spy. sql.OpenDB takes a connector, so
// there is no global driver name to register and no state between tests.
func bancoFalso(e *espiao) *sql.DB { return sql.OpenDB(conectorFalso{e}) }

type conectorFalso struct{ e *espiao }

func (c conectorFalso) Connect(context.Context) (driver.Conn, error) { return &conexao{e: c.e}, nil }
func (c conectorFalso) Driver() driver.Driver                        { return c.e }
`

const storeQueryTest = `package store

import (
	"strings"
	"testing"
)

// O ORDER BY que vem da URL é a injeção que toda listagem convida. Este teste
// é a prova de que o que chega de fora escolhe, e nunca escreve.
func TestOrderByNaoDeixaAURLEscreverSQL(t *testing.T) {
	permitido := Sortable{"nome": "nome", "tamanho": "bytes", "criado": "criado_em"}

	if got := OrderBy(permitido, "tamanho", "desc", "nome"); got != " ORDER BY bytes DESC" {
		t.Fatalf("chave conhecida = %q", got)
	}
	if got := OrderBy(permitido, "nome", "qualquer-coisa", "nome"); got != " ORDER BY nome ASC" {
		t.Fatalf("direção desconhecida devia virar ASC: %q", got)
	}

	// O que interessa: nada disto sai como SQL.
	for _, ataque := range []string{
		"bytes; DROP TABLE exemplo",
		"nome, (SELECT senha FROM usuarios)",
		"1",
		"bytes--",
		"nome' OR '1'='1",
		"",
	} {
		got := OrderBy(permitido, ataque, "asc", "nome")
		if got != " ORDER BY nome ASC" {
			t.Errorf("OrderBy(%q) = %q, queria o fallback", ataque, got)
		}
	}
	// Direção também não escreve.
	if got := OrderBy(permitido, "nome", "asc; DROP TABLE exemplo", "nome"); got != " ORDER BY nome ASC" {
		t.Errorf("direção virou SQL: %q", got)
	}
	// E uma tabela de colunas mal escrita não passa: a função confere o que
	// ela mesma leu, porque um mapa pode vir de qualquer lugar.
	ruim := Sortable{"x": "nome; DROP TABLE exemplo"}
	if got := OrderBy(ruim, "x", "asc", "x"); got != "" {
		t.Errorf("coluna que não é identificador passou: %q", got)
	}
	// Sem fallback válido não há cláusula: melhor uma listagem sem ordem que
	// uma ordem que alguém escolheu de fora.
	if got := OrderBy(Sortable{}, "nome", "asc", "nome"); got != "" {
		t.Errorf("sem colunas devia não haver ORDER BY: %q", got)
	}
}

// Um limite que vem da URL é o jeito de pedir a tabela inteira numa consulta.
func TestPaginateLimitaOQueVemDeFora(t *testing.T) {
	for _, caso := range []struct{ limite, offset, quer, querOff int }{
		{0, 0, 20, 0},
		{-5, -3, 20, 0},
		{50, 100, 50, 100},
		{99999, 0, MaxLimit, 0},
	} {
		p := Paginate(caso.limite, caso.offset, 20)
		if p.Limit != caso.quer || p.Offset != caso.querOff {
			t.Errorf("Paginate(%d,%d) = %+v, queria {%d %d}", caso.limite, caso.offset, p, caso.quer, caso.querOff)
		}
	}
}

// O % e o _ digitados na caixa de busca são curingas: sem escapar, quem digita
// "%" pede a tabela toda.
func TestLikeEscapaOsCuringas(t *testing.T) {
	if got := Like("100%"); !strings.Contains(got, "100\\\\%") {
		t.Fatalf("Like(%q) = %q", "100%", got)
	}
	if got := Like("a_b"); !strings.Contains(got, "a\\\\_b") {
		t.Fatalf("Like(%q) = %q", "a_b", got)
	}
}

// O dialeto é o único lugar que numera argumento, e ele numera nos dois.
func TestArgumentosPorDialeto(t *testing.T) {
	if got := Postgres.Args(3); got != "$1, $2, $3" {
		t.Fatalf("postgres = %q", got)
	}
	if got := SQLite.Args(3); got != "?, ?, ?" {
		t.Fatalf("sqlite = %q", got)
	}
	if DialectOf("postgres://u:p@h/db") != Postgres || DialectOf("arquivo.db") != SQLite {
		t.Fatal("o dialeto não saiu do DSN")
	}
}

// A senha do DSN não pode chegar a um log: quem não consegue conectar é
// exatamente quem põe a string inteira na mensagem.
func TestErroNaoCarregaOSegredoDoDSN(t *testing.T) {
	dsn := "postgres://ana:s3nh4-secreta@db.interno:5432/acervo"
	err := redact(errFalso("dial "+dsn+": connection refused"), dsn)
	if strings.Contains(err.Error(), "s3nh4-secreta") || strings.Contains(err.Error(), dsn) {
		t.Fatalf("o segredo sobreviveu: %s", err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("a mensagem perdeu o motivo: %s", err)
	}
}

type errFalso string

func (e errFalso) Error() string { return string(e) }
`

const storeMigrateTest = `package store

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"strings"
	"testing"
	"testing/fstest"
)

// A migração aplicada uma vez, dentro de transação, com recibo.
func TestMigrateAplicaUmaVezEGravaORecibo(t *testing.T) {
	e := &espiao{}
	db := bancoFalso(e)
	defer db.Close()
	arquivos := fstest.MapFS{
		"0001_init.sql": {Data: []byte("CREATE TABLE a (id TEXT)")},
		"0002_mais.sql": {Data: []byte("CREATE TABLE b (id TEXT)")},
	}
	if err := Migrate(context.Background(), db, SQLite, arquivos); err != nil {
		t.Fatal(err)
	}
	var aplicadas, recibos int
	for _, c := range e.chamadas() {
		if strings.HasPrefix(c.Query, "CREATE TABLE a") || strings.HasPrefix(c.Query, "CREATE TABLE b") {
			aplicadas++
		}
		if strings.HasPrefix(c.Query, "INSERT INTO schema_migrations") {
			recibos++
			// O nome e o checksum vão como argumento, nunca no texto.
			if len(c.Args) != 3 {
				t.Fatalf("o recibo foi montado com %d argumentos: %s", len(c.Args), c.Query)
			}
		}
	}
	if aplicadas != 2 || recibos != 2 {
		t.Fatalf("aplicadas=%d recibos=%d, queria 2 e 2", aplicadas, recibos)
	}
	// Ordem de nome é ordem de aplicação.
	if i, j := ondeEsta(e, "CREATE TABLE a"), ondeEsta(e, "CREATE TABLE b"); i > j {
		t.Fatalf("0002 rodou antes de 0001 (%d > %d)", i, j)
	}
}

// Editar uma migração já aplicada é recusa por nome: o banco à frente deixaria
// de ser o que o arquivo descreve, e cada ambiente teria um.
func TestMigrateRecusaArquivoQueMudouDepoisDeAplicado(t *testing.T) {
	// O banco responde que já aplicou, com outro checksum.
	outro := []driver.Value{"0000000000000000000000000000000000000000000000000000000000000000"}
	e := &espiao{linhas: map[string][][]driver.Value{
		"SELECT checksum FROM schema_migrations": {outro},
	}}
	db := bancoFalso(e)
	defer db.Close()
	arquivos := fstest.MapFS{"0001_init.sql": {Data: []byte("CREATE TABLE a (id TEXT)")}}

	err := Migrate(context.Background(), db, SQLite, arquivos)
	if err == nil {
		t.Fatal("uma migração editada depois de aplicada passou")
	}
	if !strings.Contains(err.Error(), "0001_init.sql") || !strings.Contains(err.Error(), "changed after it was applied") {
		t.Fatalf("a recusa não diz o quê nem por quê: %v", err)
	}
	// E não aplicou nada por cima.
	for _, c := range e.chamadas() {
		if strings.HasPrefix(c.Query, "CREATE TABLE a") {
			t.Fatal("aplicou a migração alterada")
		}
	}
}

// Aplicada e igual é passo pulado, e é o caso de toda subida depois da
// primeira: nenhuma escrita, nenhum lock desnecessário no caminho quente.
func TestMigrateNaoReaplicaOQueJaEstaIgual(t *testing.T) {
	corpo := []byte("CREATE TABLE a (id TEXT)")
	sum := sha256.Sum256(corpo)
	igual := []driver.Value{hex.EncodeToString(sum[:])}
	e := &espiao{linhas: map[string][][]driver.Value{
		"SELECT checksum FROM schema_migrations": {igual},
	}}
	db := bancoFalso(e)
	defer db.Close()

	if err := Migrate(context.Background(), db, SQLite, fstest.MapFS{"0001_init.sql": {Data: corpo}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range e.chamadas() {
		if strings.HasPrefix(c.Query, "CREATE TABLE a") || strings.HasPrefix(c.Query, "INSERT INTO schema_migrations") {
			t.Fatalf("reaplicou: %s", c.Query)
		}
	}
}

// Nenhuma consulta deste pacote monta valor dentro do texto: o nome do arquivo
// e o checksum viajam como argumento. É a asserção que o driver falso existe
// para permitir.
func TestMigrateNuncaConcatenaValor(t *testing.T) {
	e := &espiao{}
	db := bancoFalso(e)
	defer db.Close()
	if err := Migrate(context.Background(), db, SQLite, fstest.MapFS{
		"0001_init.sql": {Data: []byte("CREATE TABLE a (id TEXT)")},
	}); err != nil {
		t.Fatal(err)
	}
	for _, c := range e.chamadas() {
		if strings.Contains(c.Query, "0001_init.sql") {
			t.Errorf("o nome do arquivo foi para dentro do SQL: %s", c.Query)
		}
	}
}

func ondeEsta(e *espiao, prefixo string) int {
	for i, c := range e.chamadas() {
		if strings.HasPrefix(c.Query, prefixo) {
			return i
		}
	}
	return -1
}
`
