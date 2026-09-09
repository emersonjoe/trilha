package scaffold

import (
	"errors"
	"fmt"
	"go/format"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrCrudNoKey is a struct the generator cannot key on. It refuses rather than
// picking a field: the wrong key does not show up until the first Update, and
// by then there are five screens written on top of it.
var ErrCrudNoKey = errors.New("scaffold: the struct needs a string field named ID")

// CrudOptions is what `trilha generate crud` was asked for.
type CrudOptions struct {
	// Type is the struct: "Tipo" or "docs.Tipo". The qualified form settles
	// which package when the name is declared twice.
	Type string
	// At is where the screens go, under app/. Empty derives it from the type:
	// docs.Tipo becomes app/tipos.
	At string
	// Module is the project's module path, and Lang the language of the words
	// the generator writes on its own.
	Module string
	Lang   string
}

// CrudResult is what was written and what the routes came out as.
type CrudResult struct {
	Files    []string
	Patterns []string
	// Wired is app/setup.go when the generator had to touch it — the one file
	// it edits rather than refuses, and the one the command has to name,
	// because a generator that changes a file you did not ask for owes you
	// that sentence.
	Wired string
}

// Crud writes the screens a struct needs: the listing, the form that creates,
// the form that edits, the delete, and a store to keep them in.
//
// What comes out is code somebody reads and edits, in the shape
// templates/app already proved idiomatic — not a runtime that hides the
// screens behind a call. The tenth screen of a project should look like the
// first, and this is how.
func Crud(root string, o CrudOptions) (CrudResult, error) {
	var res CrudResult
	if o.Type == "" {
		return res, errors.New("scaffold: crud needs a type")
	}
	info, err := findType(root, o.Module, o.Type)
	if err != nil {
		return res, err
	}
	if info.Import == "" {
		return res, fmt.Errorf("scaffold: %s is in %s, and importing it needs the module path from go.mod",
			info.Name, info.Dir)
	}
	plan, err := planCrud(info, o)
	if err != nil {
		return res, err
	}
	// Every file is checked before any is written: a refusal that leaves half
	// a CRUD on disk is a refusal somebody has to clean up by hand.
	arquivos := plan.files()
	for _, f := range arquivos {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f.rel))); err == nil {
			return res, fmt.Errorf("%s: %w", f.rel, ErrGenExists)
		}
	}
	// Formatting before writing does two jobs: what comes out is gofmt'd, so
	// `trilha check` is green with nobody editing it — and a template that
	// produced broken Go fails here, naming the file, instead of at the first
	// compile with a line number into generated code.
	for i, f := range arquivos {
		src, err := format.Source([]byte(f.body))
		if err != nil {
			return res, fmt.Errorf("%s: %w", f.rel, err)
		}
		arquivos[i].body = string(src)
	}
	for _, f := range arquivos {
		if err := writeFile(root, f.rel, []byte(f.body), false); err != nil {
			return res, err
		}
		res.Files = append(res.Files, f.rel)
	}
	// The store has to be where the pages look, or the CRUD compiles and
	// answers 500 on the first request — which is the worst outcome a
	// generator can have, because it looks like it worked.
	wired, err := plan.wireSetup(root)
	if err != nil {
		return res, err
	}
	if wired != "" {
		res.Files = append(res.Files, wired)
		res.Wired = wired
	}
	res.Patterns = plan.patterns()
	return res, nil
}

// crudPlan is the whole answer worked out before a single byte is written.
type crudPlan struct {
	Type     string // Tipo
	Pkg      string // docs
	Ref      string // docs.Tipo
	Import   string // example.com/app/internal/docs
	StoreDir string // internal/docs
	Var      string // tipo — the receiver-ish name in generated code

	At      string // app/admin/tipos
	URL     string // /admin/tipos
	ListPkg string // tipos
	Title   string // Tipos
	One     string // Tipo

	Key     typeField   // the ID
	Fields  []typeField // what the form asks for
	Columns []typeField // what the table shows
	System  []typeField // what the generator fills in

	T map[string]string
}

type crudFile struct {
	rel  string
	body string
}

// planCrud decides everything: which field is the key, which are the form's,
// which the table shows, and where each file lands.
func planCrud(info typeInfo, o CrudOptions) (crudPlan, error) {
	p := crudPlan{
		Type: info.Name, Pkg: info.Pkg, Ref: info.Pkg + "." + info.Name,
		Import: info.Import, StoreDir: info.Dir, T: textsFor(o.Lang),
	}
	p.Var = strings.ToLower(info.Name[:1]) + info.Name[1:]
	for _, f := range info.Fields {
		switch {
		case f.Name == "ID":
			p.Key = f
		case f.JSON == "-" || f.Form == "-":
			// Explicitly out of the wire is explicitly out of the form.
		case systemField(f.Name):
			p.System = append(p.System, f)
		default:
			p.Fields = append(p.Fields, f)
		}
	}
	if p.Key.Name == "" || strings.TrimPrefix(p.Key.Type, "*") != "string" {
		return p, ErrCrudNoKey
	}
	if len(p.Fields) == 0 {
		return p, errors.New("scaffold: the struct has no field the form could ask for")
	}
	// Four columns is what a table shows before it starts scrolling sideways,
	// and the first fields of a struct are the ones somebody named first.
	p.Columns = p.Fields
	if len(p.Columns) > 4 {
		p.Columns = p.Columns[:4]
	}

	p.At = strings.Trim(o.At, "/")
	if p.At == "" {
		p.At = "app/" + plural(strings.ToLower(info.Name))
	}
	if !strings.HasPrefix(p.At, "app/") && p.At != "app" {
		return p, fmt.Errorf("scaffold: --at has to be inside app/, got %q", o.At)
	}
	p.URL = strings.TrimPrefix(p.At, "app")
	if p.URL == "" {
		p.URL = "/"
	}
	p.ListPkg = packageName(path.Base(p.At))
	p.Title = title(plural(info.Name))
	p.One = title(info.Name)
	return p, nil
}

// systemField is what the generator fills in and the form never asks for. A
// date field somebody types by hand is a bug waiting, and an "updated at" the
// person can edit is a lie in the audit trail.
func systemField(name string) bool {
	switch name {
	case "CriadoEm", "CreatedAt", "AtualizadoEm", "UpdatedAt", "ApagadoEm", "DeletedAt":
		return true
	}
	return false
}

// plural is the crude English/Portuguese plural the generated names use. It is
// crude on purpose: what it produces is a folder name and a title in a file
// somebody is about to edit, and a rule that is right nine times out of ten
// beats a flag nobody wants to pass.
func plural(s string) string {
	switch {
	case s == "":
		return s
	case strings.HasSuffix(s, "s"), strings.HasSuffix(s, "x"), strings.HasSuffix(s, "z"):
		return s + "es"
	case strings.HasSuffix(s, "ão"):
		return strings.TrimSuffix(s, "ão") + "ões"
	case strings.HasSuffix(s, "m"):
		return strings.TrimSuffix(s, "m") + "ns"
	case strings.HasSuffix(s, "l"):
		return strings.TrimSuffix(s, "l") + "is"
	case strings.HasSuffix(s, "y") && len(s) > 1 && !vowel(s[len(s)-2]):
		return strings.TrimSuffix(s, "y") + "ies"
	}
	return s + "s"
}

func vowel(b byte) bool { return strings.ContainsRune("aeiouAEIOU", rune(b)) }

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// textsFor is the language of the words the generator writes on its own. An
// unknown language is English, which is the same fallback the rest of the
// scaffold makes.
func textsFor(lang string) map[string]string {
	if t, ok := texts[lang]; ok {
		return t
	}
	return texts["en"]
}
