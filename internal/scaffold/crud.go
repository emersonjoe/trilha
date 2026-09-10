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
	// Skipped names the fields the screens could not draw. The file already
	// carries a comment where each one would go; this is so the person sees it
	// without opening the file, because a field that disappears in silence is
	// a field somebody looks for later.
	Skipped []string
	// Guard is the middleware.go that closes the folder the screens landed in,
	// and Helper the package the generated test uses to open a session — empty
	// when there is none, and then the test carries a Skip that explains.
	Guard  string
	Helper string
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
	// What is above the destination decides whether the generated test can
	// reach the screens at all.
	plan.findGuard(root, o.Module)
	res.Guard, res.Helper = plan.Guard, plan.Helper
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
	res.Skipped = plan.skipped()
	return res, nil
}

// skipped is the fields with no control: a type the CRUD does not know how to
// draw yet.
func (p crudPlan) skipped() []string {
	var out []string
	for _, f := range p.Fields {
		if _, kind := formControl(f); kind == "" {
			out = append(out, fmt.Sprintf("%s.%s (%s)", p.Type, f.Name, f.Type))
		}
	}
	return out
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

	// Guard is the middleware.go above the destination, relative to the root,
	// and Helper is the package that opens a session in a test — the recipe's
	// or the template's, whichever this project has. A CRUD generated under a
	// closed folder whose test does not sign in is a test that answers 401 and
	// blames the generator.
	Guard  string
	Helper string // import path of the test helper, "" when there is none
	Call   string // the call that opens the session
	Env    string // what the helper needs in the environment, as t.Setenv lines

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

// CrudChange is one field the struct has and a screen does not, with the
// place to put it.
type CrudChange struct {
	Field string // the Go name, as the struct writes it
	Form  string // the name it has on the wire
	File  string // the screen it is missing from, relative to the root
	Line  int    // the line the next one goes after
	Kind  string // "list" or "form"
}

// CrudMissing answers what changed in the struct since the screens were
// written: the fields the screens do not mention, and where each one goes.
//
// It is the other half of refusing to overwrite. A generator that refuses and
// stops tells somebody what they already knew — the file is there — while the
// reason they ran it again is that the struct grew a field. This says which,
// and the line to paste it on.
func CrudMissing(root string, o CrudOptions) ([]CrudChange, error) {
	info, err := findType(root, o.Module, o.Type)
	if err != nil {
		return nil, err
	}
	plan, err := planCrud(info, o)
	if err != nil {
		return nil, err
	}
	// The three screens that name fields, in the order somebody edits them.
	screens := []struct {
		rel, kind, anchor string
	}{
		{plan.At + "/page.go", "list", "{Key: "},
		{plan.At + "/new/page.go", "form", "ui.Field("},
		{plan.At + "/id_/page.go", "form", "ui.Field("},
	}
	var out []CrudChange
	for _, sc := range screens {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(sc.rel)))
		if err != nil {
			// Half a CRUD on disk is a different problem, and the answer to it
			// is the refusal that names the file — not a list of fields
			// missing from screens that were never written.
			return nil, err
		}
		src := string(body)
		fields := plan.Fields
		if sc.kind == "list" {
			fields = plan.Columns
		}
		for _, f := range fields {
			name := f.Form
			if sc.kind == "list" {
				name = f.JSON
			}
			if strings.Contains(src, sc.anchor+`"`+name+`"`) {
				continue
			}
			out = append(out, CrudChange{
				Field: f.Name, Form: name, File: sc.rel, Kind: sc.kind,
				Line: lastLineWith(src, sc.anchor),
			})
		}
	}
	return out, nil
}

// lastLineWith is the line of the last anchor in the file — where the next
// field goes. Zero when the anchor is not there at all, and then the person is
// looking at a screen this generator did not write.
func lastLineWith(src, anchor string) int {
	line, found := 0, 0
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, anchor) {
			found, line = i+1, i+1
		}
	}
	_ = found
	return line
}

// findGuard looks up from the destination for a middleware.go, and then for a
// package that can open a session in a test.
//
// The rule is "there is a middleware above", and not "there is authentication
// above": what a middleware does is somebody else's code, and a generator that
// read it would be guessing. A folder with a middleware is a folder the test
// has to assume is closed.
func (p *crudPlan) findGuard(root, module string) {
	dir := p.At
	for dir != "" && dir != "." {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir), "middleware.go")); err == nil {
			p.Guard = dir + "/middleware.go"
			break
		}
		if dir == "app" {
			break
		}
		dir = path.Dir(dir)
	}
	if p.Guard == "" {
		return
	}
	// The two helpers this repository writes: the login recipe's and the app
	// template's. A project with neither gets the Skip that names the file.
	for _, h := range []struct{ rel, imp, call, env string }{
		// The login recipe seeds its first administrator from the environment,
		// so a test that signs in has to set the same two variables before the
		// app is built — that is the recipe's rule, not this generator's.
		{"internal/sessao/sessaotest", "/internal/sessao/sessaotest", "sessaotest.Entrar(t, c)",
			"\tt.Setenv(\"ADMIN_EMAIL\", \"admin@example.com\")\n\tt.Setenv(\"ADMIN_PASSWORD\", \"a-password-nobody-guesses\")\n"},
		{"internal/session/sessiontest", "/internal/session/sessiontest", "sessiontest.Login(t, c)", ""},
	} {
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(h.rel))); err == nil && fi.IsDir() {
			p.Helper, p.Call, p.Env = module+h.imp, h.call, h.env
			return
		}
	}
}
