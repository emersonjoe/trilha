package scaffold

import (
	"fmt"
	"strings"
)

// files is the whole CRUD, worked out. Every body is finished here so that a
// refusal — an existing file — happens before anything is written.
func (p crudPlan) files() []crudFile {
	files := []crudFile{
		{rel: p.StoreDir + "/" + strings.ToLower(p.Type) + "_store.go", body: p.store()},
		{rel: p.At + "/page.go", body: p.list()},
		{rel: p.At + "/new/page.go", body: p.form(true)},
		{rel: p.At + "/id_/page.go", body: p.form(false)},
		{rel: strings.ToLower(p.Type) + "_crud_test.go", body: p.test()},
	}
	if p.Tenant || p.Policy != "" {
		files = append(files, crudFile{rel: p.At + "/middleware.go", body: p.middleware()})
	}
	return files
}

func (p crudPlan) middleware() string {
	var declarations strings.Builder
	if p.Tenant {
		declarations.WriteString("var exigeTenant = sessao.Flow.RequireTenant()\n")
	}
	if p.Policy != "" {
		fmt.Fprintf(&declarations, "var exigePolicy = sessao.Flow.RequirePolicy(acesso.Policy, %q, \"administrar\")\n", p.Policy)
	}
	next := "return next(c)"
	if p.Policy != "" {
		next = "return exigePolicy(c, next)"
	}
	if p.Tenant {
		next = "return exigeTenant(c, func() error {\n\t\t" + next + "\n\t})"
	}
	accessImport := ""
	if p.Policy != "" {
		accessImport = fmt.Sprintf("\n\t%q", p.Module+"/internal/acesso")
	}
	return fmt.Sprintf(`package %s

import (
	"github.com/emersonjoe/trilha"

	%q%s
)

%s
// Middleware requires the tenant and permission selected by the generator.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	%s
}
`, p.ListPkg, p.Module+"/internal/sessao", accessImport, declarations.String(), next)
}

func (p crudPlan) patterns() []string {
	return []string{p.URL, p.URL + "/new", p.URL + "/{id}"}
}

// store is the interface and the memory behind it.
//
// Interface plus memory is what every store in this framework is, and here it
// pays twice: what comes out of the generator runs — the screen opens, the
// form saves, the test passes — and swapping memory for a database is
// implementing five methods whose signatures are already written.
func (p crudPlan) store() string {
	var sb strings.Builder
	getArgs := "ctx context.Context, id string"
	createArgs := fmt.Sprintf("ctx context.Context, v %s", p.Type)
	updateArgs := fmt.Sprintf("ctx context.Context, id string, v %s", p.Type)
	deleteArgs := "ctx context.Context, id string"
	queryTenant, memoryTenant, memoryInit := "", "", "&"+p.Type+"Memory{}"
	if p.Tenant {
		getArgs = "ctx context.Context, tenant, id string"
		createArgs = fmt.Sprintf("ctx context.Context, tenant string, v %s", p.Type)
		updateArgs = fmt.Sprintf("ctx context.Context, tenant, id string, v %s", p.Type)
		deleteArgs = "ctx context.Context, tenant, id string"
		queryTenant = "\tTenant string\n"
		memoryTenant = "\ttenant map[string]string\n"
		memoryInit = "&" + p.Type + "Memory{tenant: map[string]string{}}"
	}
	fmt.Fprintf(&sb, `// %sStore is where the %s rows live. Five methods, which is what a screen
// needs: the listing with its filter, one row, and the three writes.
//
// Every one of them takes the request's context and answers an error, and
// neither is ceremony. The context is what stops a query when the person who
// asked for it closed the tab, or when the route's deadline ran out —
// without it a listing goes on scanning for somebody who left. The error is
// what tells a row that is not there from a database that is not answering:
// a store that can only say "no" turns an outage into a 404, which is the one
// lie a screen must not tell.
//
// It is an interface with an in-memory implementation because that is what
// makes this compile and run the moment it is generated. A database is the
// same five methods over SQL, and --store sqlite writes that one too.
type %sStore interface {
	List(ctx context.Context, q %sQuery) ([]%s, int, error)
	Get(%s) (%s, error)
	Create(%s) (%s, error)
	Update(%s) (%s, error)
	Delete(%s) error
}

// %sQuery is what the listing screen asks for: the search box, the ordering
// and the page.
type %sQuery struct {
	%sQ      string
	Sort   string
	Asc    bool
	Offset int
	Limit  int
}

// %sMemory keeps the rows in memory. It is the starting point, and the place
// a test gets a store of its own.
type %sMemory struct {
	mu   sync.RWMutex
	rows []%s
	seq  int
	%s}

// New%sMemory is the empty store.
func New%sMemory() *%sMemory { return %s }

`, p.Type, p.Type, p.Type, p.Type, p.Type, getArgs, p.Type, createArgs, p.Type,
		updateArgs, p.Type, deleteArgs, p.Type, p.Type, queryTenant, p.Type, p.Type, p.Type,
		memoryTenant, p.Type, p.Type, p.Type, memoryInit)

	// List
	fmt.Fprintf(&sb, `// List filters, orders and pages, and answers how many passed the filter —
// the two numbers the pagination needs.
//
// The context is checked even here, where the data is in the process: a
// memory store that ignores it would let a screen pass a cancelled request
// and still look right, and then the same screen over SQL would be the one
// that behaves differently.
func (s *%sMemory) List(ctx context.Context, q %sQuery) ([]%s, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []%s
	for _, v := range s.rows {
%s		if q.Q != "" && !%sMatches(v, q.Q) {
			continue
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		less := %sLess(out[i], out[j], q.Sort)
		if q.Asc {
			return less
		}
		return !less
	})
	total := len(out)
	if q.Offset >= total {
		return nil, total, nil
	}
	out = out[q.Offset:]
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, total, nil
}

// %sMatches is the search box: it looks in the text fields, which is what
// somebody typing into it expects.
func %sMatches(v %s, q string) bool {
	q = strings.ToLower(q)
`, p.Type, p.Type, p.Type, p.Type, p.memoryTenantFilter(), p.Type, p.Type, p.Type, p.Type, p.Type)
	achou := false
	for _, f := range p.Fields {
		if strings.TrimPrefix(f.Type, "*") == "string" {
			fmt.Fprintf(&sb, "\tif strings.Contains(strings.ToLower(v.%s), q) {\n\t\treturn true\n\t}\n", f.Name)
			achou = true
		}
	}
	if !achou {
		sb.WriteString("\t_ = q // no text field to search in; add one here when there is\n")
	}
	fmt.Fprintf(&sb, `	return false
}

// %sLess is the ordering. A column the screen does not offer falls back to the
// key, so a crooked address answers in a stable order instead of panicking.
func %sLess(a, b %s, sort string) bool {
	switch sort {
`, p.Type, p.Type, p.Type)
	for _, f := range p.Columns {
		fmt.Fprintf(&sb, "\tcase %q:\n\t\treturn %s\n", f.Form, lessExpr(f))
	}
	fmt.Fprintf(&sb, `	}
	return a.ID < b.ID
}

// Get answers one row, or trilha.ErrNotFound: "it is not here" is an error
// and not a second return value, so a handler that forwards it gets the 404 without
// deciding anything, and a store that cannot answer at all is not mistaken
// for one that answered "no".
func (s *%sMemory) Get(%s) (%s, error) {
	var zero %s
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.rows {
		%s
		if v.ID == id {
			return v, nil
		}
	}
	return zero, trilha.ErrNotFound
}

// Create files a new row and gives it the key. The key is made here and never
// taken from the form: an id the visitor chose is an id the visitor can
// collide with somebody else's.
func (s *%sMemory) Create(%s) (%s, error) {
	if err := ctx.Err(); err != nil {
		return v, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	v.ID = strconv.Itoa(s.seq)
%s	s.rows = append(s.rows, v)
	%s
	return v, nil
}

// Update replaces the fields of a row, keeping its key.
func (s *%sMemory) Update(%s) (%s, error) {
	if err := ctx.Err(); err != nil {
		return v, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.rows {
		%s
		if old.ID != id {
			continue
		}
		v.ID = id
%s		s.rows[i] = v
		return v, nil
	}
	var zero %s
	return zero, trilha.ErrNotFound
}

// Delete removes the row, and says trilha.ErrNotFound when there was none —
// which is what tells a 404 from a 303.
func (s *%sMemory) Delete(%s) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.rows {
		%s
		if v.ID == id {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			%s
			return nil
		}
	}
	return trilha.ErrNotFound
}
`, p.Type, getArgs, p.Type, p.Type, p.memoryTenantCheck(),
		p.Type, createArgs, p.Type, p.systemOnCreate(), p.memoryTenantSet(),
		p.Type, updateArgs, p.Type, p.memoryTenantCheck(), p.systemOnUpdate(), p.Type,
		p.Type, deleteArgs, p.memoryTenantCheck(), p.memoryTenantDelete())

	return p.storeHeader(sb.String()) + sb.String()
}

func (p crudPlan) memoryTenantFilter() string {
	if !p.Tenant {
		return ""
	}
	return "\t\tif s.tenant[v.ID] != q.Tenant {\n\t\t\tcontinue\n\t\t}\n"
}

func (p crudPlan) memoryTenantCheck() string {
	if !p.Tenant {
		return ""
	}
	return "if s.tenant[v.ID] != tenant { continue }"
}

func (p crudPlan) memoryTenantSet() string {
	if !p.Tenant {
		return ""
	}
	return "s.tenant[v.ID] = tenant"
}

func (p crudPlan) memoryTenantDelete() string {
	if !p.Tenant {
		return ""
	}
	return "delete(s.tenant, id)"
}

// storeHeader is the package clause and the imports the body ended up
// needing. Deciding them from the body is the only way they stay honest
// across every shape of struct.
func (p crudPlan) storeHeader(body string) string {
	std := []string{"context", "sort", "strconv", "strings", "sync"}
	if strings.Contains(body, "time.Now()") || strings.Contains(body, "time.Time") {
		std = append(std, "time")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "package %s\n\nimport (\n", p.Pkg)
	for _, s := range sorted(std) {
		fmt.Fprintf(&sb, "\t%q\n", s)
	}
	// The framework's not-found is the store's way of saying it: the handler
	// returns the error and the app answers its own 404 page. It is the same
	// error the store recipe hands back for sql.ErrNoRows, so the two
	// implementations of this interface are indistinguishable from a screen.
	sb.WriteString("\n\t\"github.com/emersonjoe/trilha\"\n)\n\n")
	return sb.String()
}

// systemOnCreate and systemOnUpdate stamp the fields the form never asks for.
func (p crudPlan) systemOnCreate() string {
	var sb strings.Builder
	for _, f := range p.System {
		if strings.Contains(f.Type, "time.Time") {
			fmt.Fprintf(&sb, "\tv.%s = time.Now()\n", f.Name)
		}
	}
	return sb.String()
}

func (p crudPlan) systemOnUpdate() string {
	var sb strings.Builder
	for _, f := range p.System {
		if !strings.Contains(f.Type, "time.Time") {
			continue
		}
		switch f.Name {
		case "AtualizadoEm", "UpdatedAt":
			fmt.Fprintf(&sb, "\t\tv.%s = time.Now()\n", f.Name)
		default:
			// Created stays created: an update that resets it loses the one
			// date somebody actually goes looking for.
			fmt.Fprintf(&sb, "\t\tv.%s = old.%s\n", f.Name, f.Name)
		}
	}
	return sb.String()
}

// lessExpr is how one column compares. Only what the table shows gets one.
func lessExpr(f typeField) string {
	t := strings.TrimPrefix(f.Type, "*")
	switch {
	case t == "string":
		return fmt.Sprintf("strings.ToLower(a.%s) < strings.ToLower(b.%s)", f.Name, f.Name)
	case t == "bool":
		return fmt.Sprintf("!a.%s && b.%s", f.Name, f.Name)
	case strings.HasPrefix(t, "int"), strings.HasPrefix(t, "uint"), strings.HasPrefix(t, "float"):
		return fmt.Sprintf("a.%s < b.%s", f.Name, f.Name)
	case t == "time.Time":
		return fmt.Sprintf("a.%s.Before(b.%s)", f.Name, f.Name)
	}
	return fmt.Sprintf("a.ID < b.ID // %s: no ordering for this type", f.Name)
}

func sorted(v []string) []string {
	out := append([]string{}, v...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
