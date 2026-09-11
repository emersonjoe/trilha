package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrCrudNoStore is `--store sqlite` in a project that has no SQL store to
// write against. The generator emits against the `store` recipe — its
// dialect, its query kit, its migration runner — and a generator that writes
// against a package that is not there hands somebody a project that does not
// compile.
var ErrCrudNoStore = errors.New("scaffold: --store needs the store recipe; run `trilha add store` first")

// sqlColumn is one field of the struct as a column: the name it has in the
// table and the type it has in the dialect asked for.
type sqlColumn struct {
	Field typeField
	Name  string // nome, criado_em
	DDL   string // TEXT, BIGINT, TIMESTAMP
}

// planSQL decides the table: its name, its columns, and which fields have no
// column at all. It runs for every CRUD — the listing of what could not be
// mapped is worth having even when nobody asked for SQL.
func (p *crudPlan) planSQL() {
	p.Table = plural(strings.ToLower(p.Type))
	add := func(f typeField) {
		ddl := ddlType(f.Type, p.Store)
		if ddl == "" {
			p.NoColumn = append(p.NoColumn, fmt.Sprintf("%s.%s (%s)", p.Type, f.Name, f.Type))
			return
		}
		p.Cols = append(p.Cols, sqlColumn{Field: f, Name: columnName(f), DDL: ddl})
	}
	add(p.Key)
	for _, f := range p.Fields {
		add(f)
	}
	for _, f := range p.System {
		add(f)
	}
}

// columnName is the field's name in the table. The json name is what the rest
// of the application already calls the field, so the schema and the wire read
// the same; a name that could not be a bare identifier falls back to the Go
// name in snake case rather than being quoted, because a quoted column is a
// column every hand-written query has to remember to quote.
func columnName(f typeField) string {
	if sqlIdent(f.JSON) {
		return f.JSON
	}
	return snake(f.Name)
}

func sqlIdent(s string) bool {
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

// snake turns CriadoEm into criado_em.
func snake(name string) string {
	var sb strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			sb.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// ddlType is the column type for a Go type, in the dialect asked for. A type
// that is not here has no column: the generator says so rather than guessing,
// because a column whose type is a guess is a migration somebody has to undo.
func ddlType(t, dialect string) string {
	pg := dialect == "postgres"
	t = strings.TrimPrefix(t, "*")
	switch {
	case t == "string":
		return "TEXT"
	case t == "bool":
		if pg {
			return "BOOLEAN"
		}
		// SQLite has no boolean: it stores 0 and 1 in an INTEGER, and the
		// driver scans both sides of that without anybody asking.
		return "INTEGER"
	case strings.HasPrefix(t, "int"), strings.HasPrefix(t, "uint"):
		if pg {
			return "BIGINT"
		}
		return "INTEGER"
	case strings.HasPrefix(t, "float"):
		if pg {
			return "DOUBLE PRECISION"
		}
		return "REAL"
	case t == "time.Time":
		return "TIMESTAMP"
	}
	return ""
}

// sqlFiles is the SQL half of the CRUD: the implementation, its test and the
// migration that makes the table exist.
func (p crudPlan) sqlFiles(root string) []crudFile {
	if !p.sql() {
		return nil
	}
	base := strings.ToLower(p.Type)
	return []crudFile{
		{rel: p.StoreDir + "/" + base + "_store_sql.go", body: p.storeSQL()},
		{rel: p.StoreDir + "/" + base + "_store_sql_test.go", body: p.storeSQLTest()},
		{rel: p.migrationRel(root), body: p.migration(), raw: true},
	}
}

func (p crudPlan) sql() bool { return p.Store == "sqlite" || p.Store == "postgres" }

// migrationRel is the next migration's name. The number is one past the
// highest on disk, because name order is apply order — a file numbered behind
// one that already ran would never be applied on a database that has seen it.
func (p crudPlan) migrationRel(root string) string {
	maior := 0
	entradas, _ := os.ReadDir(filepath.Join(root, "migrations"))
	for _, e := range entradas {
		nome := e.Name()
		if !strings.HasSuffix(nome, ".sql") {
			continue
		}
		n := 0
		for i := 0; i < len(nome) && nome[i] >= '0' && nome[i] <= '9'; i++ {
			n = n*10 + int(nome[i]-'0')
		}
		if n > maior {
			maior = n
		}
	}
	return fmt.Sprintf("migrations/%04d_%s.sql", maior+1, p.Table)
}

// migration is the table. It is a file and not a CREATE TABLE run at boot
// because that is the convention the store recipe owns: applied once, in name
// order, with a checksum that refuses an edit afterwards.
func (p crudPlan) migration() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `-- %s: the table behind %s.%sStore.
--
-- Written by `+"`"+`trilha generate crud %s --store %s`+"`"+`. From here it is yours:
-- an applied migration is never edited — the checksum is compared on every
-- boot and a changed file is refused by name — so a column you want tomorrow
-- is the next file, not this one.

CREATE TABLE IF NOT EXISTS %s (
`, p.Table, p.Pkg, p.Type, p.Ref, p.Store, p.Table)

	larg := 0
	for _, c := range p.Cols {
		if len(c.Name) > larg {
			larg = len(c.Name)
		}
	}
	for i, c := range p.Cols {
		virgula := ","
		if i == len(p.Cols)-1 {
			virgula = ""
		}
		tipo := c.DDL
		switch {
		case c.Field.Name == p.Key.Name:
			tipo += " PRIMARY KEY"
		case !strings.HasPrefix(c.Field.Type, "*"):
			// A field that is not a pointer has no way to say "no value", so
			// the column should not offer one either: a NULL nothing can
			// produce is a NULL every read has to handle anyway.
			tipo += " NOT NULL"
		}
		fmt.Fprintf(&sb, "\t%-*s %s%s\n", larg, c.Name, tipo, virgula)
	}
	sb.WriteString(");\n")
	// The listing orders by the first column it offers, on every page load.
	// An ordering with no index is the sequential scan that shows up the
	// month the table gets big, and never before.
	if s := p.sortFallback(); s != "" && s != p.keyColumn() {
		fmt.Fprintf(&sb, "\nCREATE INDEX IF NOT EXISTS %s_%s ON %s (%s);\n", p.Table, s, p.Table, s)
	}
	return sb.String()
}

// keyColumn and sortFallback are the two columns the generated SQL names on
// its own: the key it looks rows up by, and the ordering a listing falls back
// to when what came in the URL is not one it offers.
func (p crudPlan) keyColumn() string { return columnName(p.Key) }

func (p crudPlan) sortFallback() string {
	if s := p.sortable(); len(s) > 0 {
		return s[0].Name
	}
	return p.keyColumn()
}

// sortable is the columns the listing lets somebody order by — the table's
// own, plus the dates the store stamps, which the listing shows read-only.
// It is the same set the screen declares with Sort: true, and it has to be:
// a key the screen offers and the store does not know is an ordering that
// silently does nothing.
func (p crudPlan) sortable() []sqlColumn {
	offers := func(f typeField) bool {
		for _, col := range p.Columns {
			if col.Name == f.Name {
				return true
			}
		}
		for _, sys := range p.System {
			if sys.Name == f.Name && strings.TrimPrefix(sys.Type, "*") == "time.Time" {
				return true
			}
		}
		return false
	}
	var out []sqlColumn
	for _, c := range p.Cols {
		if offers(c.Field) {
			out = append(out, c)
		}
	}
	return out
}

// textColumns is what the search box looks in: the same choice the memory
// store makes, so the two answer the same question.
func (p crudPlan) textColumns() []sqlColumn {
	var out []sqlColumn
	for _, c := range p.Cols {
		if c.Field.Name != p.Key.Name && strings.TrimPrefix(c.Field.Type, "*") == "string" {
			out = append(out, c)
		}
	}
	return out
}

// columnList is "id, nome, sigla" — the one order every statement in the
// generated file reads and writes in, so the SELECT and the Scan cannot drift
// apart.
func (p crudPlan) columnList() string {
	var names []string
	for _, c := range p.Cols {
		names = append(names, c.Name)
	}
	return strings.Join(names, ", ")
}

// scanArgs is the Scan of a whole row, in the same order.
func (p crudPlan) scanArgs(v string) string {
	var out []string
	for _, c := range p.Cols {
		out = append(out, "&"+v+"."+c.Field.Name)
	}
	return strings.Join(out, ", ")
}

// writable is every column an UPDATE sets: not the key, which comes from the
// address, and not the created-at, which an update that rewrote it would
// erase.
func (p crudPlan) writable() []sqlColumn {
	var out []sqlColumn
	for _, c := range p.Cols {
		switch {
		case c.Field.Name == p.Key.Name:
		case c.Field.Name == "CriadoEm" || c.Field.Name == "CreatedAt":
		default:
			out = append(out, c)
		}
	}
	return out
}

// storeSQL is the same five methods over database/sql, written against the
// recipe: its dialect for the placeholders, its query kit for everything the
// URL gets to influence, its NotFound for the one database error that is not
// a failure.
//
// What it is careful about is one thing, and it is the thing a listing screen
// invites: no value and no identifier that came from outside is ever
// concatenated into a statement.
func (p crudPlan) storeSQL() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `// %sSQL is %sStore over database/sql.
//
// It is written against the store recipe: its dialect decides the
// placeholders, its query kit bounds everything the URL gets to influence,
// its migrations own the table. Nothing typed by a visitor is ever
// concatenated into a statement: values travel as arguments, the ordering
// selects a column from the table below, and the page has the recipe's cap.
type %sSQL struct {
	db *sql.DB
	d  store.Dialect
}

// New%sSQL takes the pool and the dialect the recipe opened. Both are
// arguments rather than globals read inside, so a test can hand this a
// database of its own.
func New%sSQL(db *sql.DB, d store.Dialect) *%sSQL { return &%sSQL{db: db, d: d} }

// The two implementations of %sStore have to stay in step, and what says so
// is the compiler and not a review.
var _ %sStore = (*%sSQL)(nil)

// The table, and the one column order every statement here reads and writes
// in: with a single list the SELECT and the Scan cannot drift apart.
const (
	%sTable   = %q
	%sColumns = %q
)

// %sSortable maps the sort keys a URL may use onto the columns they mean. It
// is the whole answer to the ORDER BY that comes from outside: an identifier
// cannot be a placeholder, so what arrives *selects* from a table the code
// wrote, and anything that does not select falls back — see store.OrderBy.
//
// It lives next to the queries on purpose: a column renamed in a migration
// should break the build here, not quietly start ordering by something else.
var %sSortable = store.Sortable{
`, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type,
		p.Var, p.Table, p.Var, p.columnList(), p.Var, p.Var)
	for _, c := range p.sortable() {
		fmt.Fprintf(&sb, "\t%q: %q,\n", c.Field.Form, c.Name)
	}
	sb.WriteString("}\n")

	p.listSQL(&sb)
	p.readSQL(&sb)
	p.writeSQL(&sb)
	return p.sqlHeader(sb.String()) + sb.String()
}

// listSQL is the listing: the filter, the ordering and the page, built in a
// function of its own so a test can read the statement without a database.
// What has to be true about this query is a property of the text that leaves,
// and asserting on the text costs nothing to run.
func (p crudPlan) listSQL(sb *strings.Builder) {
	texto := p.textColumns()
	fmt.Fprintf(sb, `
// query is the statement the listing runs, the one that counts the rows
// behind it, and the arguments both of them take. It is separate from List so
// that a test can read what leaves this process without a database.
func (s *%sSQL) query(q %sQuery) (rows, count string, args []any, page store.Page) {
	where := ""
`, p.Type, p.Type)
	switch len(texto) {
	case 0:
		fmt.Fprintf(sb, `	// No text column to search in: the search box filters nothing until
	// there is one, and q.Q never reaches a statement.
	_ = q.Q
`)
	default:
		fmt.Fprintf(sb, `	if q.Q != "" {
		// The box is escaped and travels as an argument — one per column,
		// because SQLite reads its placeholders in order. A %% somebody typed
		// is a %% they are looking for, not every row in the table.
		like := store.Like(q.Q)
		var ors []string
`)
		for _, c := range texto {
			fmt.Fprintf(sb, "\t\targs = append(args, like)\n\t\tors = append(ors, %q+s.d.Arg(len(args))+\" ESCAPE '\\\\'\")\n",
				c.Name+" LIKE ")
		}
		fmt.Fprintf(sb, `		where = " WHERE (" + strings.Join(ors, " OR ") + ")"
	}
`)
	}
	dir := "asc"
	_ = dir
	fmt.Fprintf(sb, `	dir := "asc"
	if !q.Asc {
		dir = "desc"
	}
	// Both of these are the recipe's, and both are about what the URL may
	// ask for: an ordering it did not declare, and a page big enough to read
	// the whole table.
	order := store.OrderBy(%sSortable, q.Sort, dir, %q)
	page = store.Paginate(q.Limit, q.Offset, 20)

	rows = "SELECT " + %sColumns + " FROM " + %sTable + where + order +
		" LIMIT " + s.d.Arg(len(args)+1) + " OFFSET " + s.d.Arg(len(args)+2)
	count = "SELECT count(*) FROM " + %sTable + where
	return rows, count, args, page
}

// List answers the page and how many rows passed the filter — the two numbers
// the pagination needs.
func (s *%sSQL) List(ctx context.Context, q %sQuery) ([]%s, int, error) {
	sel, count, args, page := s.query(q)
	var total int
	if err := s.db.QueryRowContext(ctx, count, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, sel, append(args, page.Limit, page.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []%s
	for rows.Next() {
		var v %s
		if err := rows.Scan(%s); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
`, p.Var, p.sortFallback(), p.Var, p.Var, p.Var,
		p.Type, p.Type, p.Type, p.Type, p.Type, p.scanArgs("v"))
}

// readSQL is Get: one row by its key, and the recipe's NotFound for the one
// database error that is not a server failure.
func (p crudPlan) readSQL(sb *strings.Builder) {
	fmt.Fprintf(sb, `
// Get answers one row. store.NotFound turns sql.ErrNoRows into the
// framework's not-found, which is the same error the memory store answers
// with — so a screen cannot tell the two implementations apart, and a
// database that is down is not written down as "it is not here".
func (s *%sSQL) Get(ctx context.Context, id string) (%s, error) {
	var v %s
	err := s.db.QueryRowContext(ctx,
		"SELECT "+%sColumns+" FROM "+%sTable+" WHERE %s = "+s.d.Arg(1), id,
	).Scan(%s)
	if err != nil {
		return %s{}, store.NotFound(err)
	}
	return v, nil
}
`, p.Type, p.Type, p.Type, p.Var, p.Var, p.keyColumn(), p.scanArgs("v"), p.Type)
}

// writeSQL is the three writes. Each one says how many rows it touched,
// because zero is the 404 and there is no other way to hear it.
func (p crudPlan) writeSQL(sb *strings.Builder) {
	var valores []string
	for _, c := range p.Cols {
		valores = append(valores, "v."+c.Field.Name)
	}
	var sets []string
	var setValores []string
	for i, c := range p.writable() {
		sep := " SET "
		if i > 0 {
			sep = ", "
		}
		sets = append(sets, fmt.Sprintf(`%q+s.d.Arg(%d)`, sep+c.Name+" = ", i+1))
		setValores = append(setValores, "v."+c.Field.Name)
	}
	setValores = append(setValores, "id")
	fmt.Fprintf(sb, `
// Create files a new row. The key is made here and never taken from the form:
// an id the visitor chose is an id the visitor can collide with somebody
// else's.
func (s *%sSQL) Create(ctx context.Context, v %s) (%s, error) {
	v.%s = new%sID()
%s	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO "+%sTable+" ("+%sColumns+") VALUES ("+s.d.Args(%d)+")",
		%s,
	); err != nil {
		return %s{}, err
	}
	return v, nil
}

// new%sID is the key of a new row: sixteen random bytes rather than a
// counter, because a number in an address tells a visitor how many rows there
// are and invites them to walk the ones that are not theirs.
func new%sID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// There is no useful way to continue: an id that is not random is an
		// id somebody else can guess.
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// Update writes the fields the form sends and leaves the rest alone — the key
// comes from the address, and the created-at is not rewritten, because that
// is the one date people go looking for. Zero rows touched is the 404.
func (s *%sSQL) Update(ctx context.Context, id string, v %s) (%s, error) {
	v.%s = id
%s	res, err := s.db.ExecContext(ctx,
		"UPDATE "+%sTable+%s+" WHERE %s = "+s.d.Arg(%d),
		%s,
	)
	if err != nil {
		return %s{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return %s{}, err
	}
	if n == 0 {
		return %s{}, trilha.ErrNotFound
	}
	// Read it back rather than answering with what the form brought: what the
	// caller gets is then the row as the database has it, columns this
	// statement did not write included.
	return s.Get(ctx, id)
}

// Delete removes the row, and says not-found when there was none — which is
// what tells a 404 from a 303.
func (s *%sSQL) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM "+%sTable+" WHERE %s = "+s.d.Arg(1), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return trilha.ErrNotFound
	}
	return nil
}
`, p.Type, p.Type, p.Type, p.Key.Name, p.Type, p.stampSQL("v", true),
		p.Var, p.Var, len(p.Cols), strings.Join(valores, ", "), p.Type,
		p.Type, p.Type,
		p.Type, p.Type, p.Type, p.Key.Name, p.stampSQL("v", false),
		p.Var, strings.Join(sets, "+"), p.keyColumn(), len(sets)+1,
		strings.Join(setValores, ", "), p.Type, p.Type, p.Type,
		p.Type, p.Var, p.keyColumn())
}

// stampSQL sets the dates the form never asks for. The clock is Go's and not
// the database's so that the value written and the value handed back are the
// same one — and it is UTC, because a timestamp whose zone depends on the
// server changes meaning when the server moves.
func (p crudPlan) stampSQL(v string, criando bool) string {
	var sb strings.Builder
	for _, f := range p.System {
		if strings.TrimPrefix(f.Type, "*") != "time.Time" {
			continue
		}
		switch f.Name {
		case "AtualizadoEm", "UpdatedAt":
			fmt.Fprintf(&sb, "\t%s.%s = time.Now().UTC()\n", v, f.Name)
		default:
			if criando {
				fmt.Fprintf(&sb, "\t%s.%s = time.Now().UTC()\n", v, f.Name)
			}
		}
	}
	return sb.String()
}

// sqlHeader is the package clause and the imports the body ended up needing.
func (p crudPlan) sqlHeader(body string) string {
	std := []string{"context", "crypto/rand", "database/sql", "encoding/hex"}
	if strings.Contains(body, "strings.Join") {
		std = append(std, "strings")
	}
	if strings.Contains(body, "time.Now()") {
		std = append(std, "time")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "package %s\n\nimport (\n", p.Pkg)
	for _, s := range sorted(std) {
		fmt.Fprintf(&sb, "\t%q\n", s)
	}
	fmt.Fprintf(&sb, "\n\t\"github.com/emersonjoe/trilha\"\n\n\t%q\n)\n\n", p.StoreImport)
	return sb.String()
}

// storeSQLTest is the test that goes with the SQL store, and it needs no
// database: what it asserts is a property of the statement that leaves this
// process, which is the same thing the recipe's own tests assert and the only
// way these claims stay checkable in somebody else's project.
func (p crudPlan) storeSQLTest() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, `package %s

import (
	"strings"
	"testing"

	%q
)

// The listing is where an application invites SQL injection: the column to
// order by and the direction both come from the URL, and neither can be a
// placeholder — an identifier is not a value. These tests read the statement
// this store builds without opening a database, because what has to be true
// about it is a property of the text.

// Ordering never comes from the URL: what arrives selects a column from
// %sSortable, and anything that does not select falls back.
func Test%sSQLOrderingComesFromTheTable(t *testing.T) {
	s := New%sSQL(nil, store.SQLite)
	for _, attack := range []string{
		"%s; DROP TABLE %s", "%s--", "' OR '1'='1", "(SELECT 1)", "%s ASC, 1", "", "id",
	} {
		sel, _, _, _ := s.query(%sQuery{Sort: attack, Asc: true})
		if !strings.Contains(sel, " ORDER BY %s ASC") {
			t.Errorf("sort %%q ordered by something else: %%s", attack, sel)
		}
		for _, leaked := range []string{"DROP", "--", "'", "SELECT 1"} {
			if strings.Contains(strings.TrimPrefix(sel, "SELECT "), leaked) {
				t.Errorf("sort %%q put %%q into the statement: %%s", attack, leaked, sel)
			}
		}
	}
}

// The page has a ceiling even when the address asks for the whole table: a
// limit nobody bounds is a denial of service written by the visitor.
func Test%sSQLPageHasACeiling(t *testing.T) {
	s := New%sSQL(nil, store.SQLite)
	_, _, _, page := s.query(%sQuery{Limit: 100000})
	if page.Limit > store.MaxLimit {
		t.Fatalf("limit = %%d, more than store.MaxLimit", page.Limit)
	}
}
`, p.Pkg, p.StoreImport, p.Var, p.Type, p.Type, p.sortFallback(), p.Table, p.sortFallback(),
		p.sortFallback(), p.Type, p.sortFallback(), p.Type, p.Type, p.Type)

	if texto := p.textColumns(); len(texto) > 0 {
		fmt.Fprintf(&sb, `
// What somebody types in the search box travels beside the statement and
// never inside it — and the wildcards are escaped, so a %% is a %% they are
// looking for and not every row in the table.
func Test%sSQLSearchTravelsAsAnArgument(t *testing.T) {
	s := New%sSQL(nil, store.SQLite)
	sel, count, args, _ := s.query(%sQuery{Q: "100%% ' OR 1=1"})
	for _, q := range []string{sel, count} {
		if strings.Contains(q, "100") || strings.Contains(q, "OR 1=1") {
			t.Fatalf("what was typed reached the statement: %%s", q)
		}
	}
	if len(args) != %d {
		t.Fatalf("args = %%v, expected one per text column", args)
	}
	if !strings.Contains(args[0].(string), "\\%%") {
		t.Fatalf("the wildcard was not escaped: %%q", args[0])
	}
}
`, p.Type, p.Type, p.Type, len(texto))
	}
	return sb.String()
}
