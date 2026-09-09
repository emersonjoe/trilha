package scaffold

import (
	"fmt"
	"strings"
)

// files is the whole CRUD, worked out. Every body is finished here so that a
// refusal — an existing file — happens before anything is written.
func (p crudPlan) files() []crudFile {
	return []crudFile{
		{rel: p.StoreDir + "/" + strings.ToLower(p.Type) + "_store.go", body: p.store()},
		{rel: p.At + "/page.go", body: p.list()},
		{rel: p.At + "/new/page.go", body: p.form(true)},
		{rel: p.At + "/id_/page.go", body: p.form(false)},
		{rel: strings.ToLower(p.Type) + "_crud_test.go", body: p.test()},
	}
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
	fmt.Fprintf(&sb, `package %s

import (
	"sort"
	"strings"
	"sync"
)

// %sStore is where the %s rows live. Five methods, which is what a screen
// needs: the listing with its filter, one row, and the three writes.
//
// It is an interface with an in-memory implementation because that is what
// makes this compile and run the moment it is generated. A database is the
// same five methods over your own SQL — see the database recipe.
type %sStore interface {
	List(q %sQuery) ([]%s, int)
	Get(id string) (%s, bool)
	Create(v %s) (%s, error)
	Update(id string, v %s) (%s, error)
	Delete(id string) bool
}

// %sQuery is what the listing screen asks for: the search box, the ordering
// and the page.
type %sQuery struct {
	Q      string
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
}

// New%sMemory is the empty store.
func New%sMemory() *%sMemory { return &%sMemory{} }

`, p.Pkg, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type,
		p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type)

	// List
	fmt.Fprintf(&sb, `// List filters, orders and pages, and answers how many passed the filter —
// the two numbers the pagination needs.
func (s *%sMemory) List(q %sQuery) ([]%s, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []%s
	for _, v := range s.rows {
		if q.Q != "" && !%sMatches(v, q.Q) {
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
		return nil, total
	}
	out = out[q.Offset:]
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, total
}

// %sMatches is the search box: it looks in the text fields, which is what
// somebody typing into it expects.
func %sMatches(v %s, q string) bool {
	q = strings.ToLower(q)
`, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type)
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

// Get answers one row.
func (s *%sMemory) Get(id string) (%s, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.rows {
		if v.ID == id {
			return v, true
		}
	}
	var zero %s
	return zero, false
}

// Create files a new row and gives it the key. The key is made here and never
// taken from the form: an id the visitor chose is an id the visitor can
// collide with somebody else's.
func (s *%sMemory) Create(v %s) (%s, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	v.ID = strconv.Itoa(s.seq)
%s	s.rows = append(s.rows, v)
	return v, nil
}

// Update replaces the fields of a row, keeping its key.
func (s *%sMemory) Update(id string, v %s) (%s, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, old := range s.rows {
		if old.ID != id {
			continue
		}
		v.ID = id
%s		s.rows[i] = v
		return v, nil
	}
	var zero %s
	return zero, errors.New("%s: not found")
}

// Delete answers whether there was anything to delete, which is what tells a
// 404 from a 303.
func (s *%sMemory) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.rows {
		if v.ID == id {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			return true
		}
	}
	return false
}
`, p.Type, p.Type, p.Type, p.Type, p.Type, p.Type, p.systemOnCreate(), p.Type, p.Type, p.Type,
		p.systemOnUpdate(), p.Type, strings.ToLower(p.Type), p.Type)

	body := sb.String()
	// The imports are decided by what the body ended up needing, which is the
	// only way to keep them honest across every shape of struct.
	imps := []string{"sort", "strconv", "strings", "sync"}
	if strings.Contains(body, "errors.New") {
		imps = append(imps, "errors")
	}
	if strings.Contains(body, "time.Now()") {
		imps = append(imps, "time")
	}
	return strings.Replace(body,
		"import (\n\t\"sort\"\n\t\"strings\"\n\t\"sync\"\n)",
		"import (\n\t\""+strings.Join(sorted(imps), "\"\n\t\"")+"\"\n)", 1)
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
