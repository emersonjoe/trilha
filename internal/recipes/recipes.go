// Package recipes writes the patterns an application repeats into the
// application itself.
//
// Half of what a management app needs is a pattern and not a primitive: the
// trail of who did what, the screen that issues API keys, the settings section
// somebody edits instead of redeploying. In the framework they would be rigid;
// as documentation they are work to copy. Here they are files the project
// receives and then owns.
//
// The difference from the generator is the direction. `trilha generate crud`
// writes from the user's code — a struct becomes a screen. `trilha add` writes
// from a recipe of the framework's.
//
// There is no "requires" gate here, and that is on purpose: none of these
// recipes needs one, and a field with no consumer is a promise nobody asked
// for. It arrives with the first recipe that genuinely depends on something —
// a share-link needs a session — and it will be written and tested there.
package recipes

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// ErrUnknown is a name no recipe answers to.
var ErrUnknown = errors.New("recipes: no recipe by that name")

// ErrMissing is a recipe that needs another one first.
var ErrMissing = errors.New("recipes: this recipe needs another one first")

// Recipe is one pattern: what it is called and what it writes.
type Recipe struct {
	Name string
	// Summary is the one line `trilha add` prints, per language.
	Summary map[string]string
	// Doc is the page that explains it. Recipes point at documentation that
	// already exists rather than each carrying a page of its own.
	Doc string
	// Files are written when they are not already there.
	Files []File
	// Setup are the lines that go into app/setup.go, each behind its own
	// marker so a second run recognises them instead of duplicating them.
	Setup []Insert
	// Imports are what those lines need.
	Imports []string
	// Next is what the person does after this, printed at the end.
	Next map[string]string
	// Needs are the recipes that have to be there first, each recognised by a
	// file it writes. Refusing costs one line; writing five files that do not
	// compile costs somebody an afternoon of cleaning up after a tool.
	Needs []Need
}

// Need is one recipe this one is written on top of.
type Need struct {
	// Recipe is the name to run first.
	Recipe string
	// File is what proves it ran, relative to the project root. It is a file
	// that recipe writes and that this one depends on — not a marker in
	// setup.go, because the dependency here is on the code, and somebody who
	// deleted the code has the problem this check is about.
	File string
}

// File is one file the recipe writes.
type File struct {
	// Rel is where it goes, slash-separated from the project root. It is a
	// template too: a recipe can put a screen where the app's language wants
	// it.
	Rel string
	// Body is the file, as a template.
	Body string
	// Go marks a file that gets gofmt'd before it is written — which is also
	// how a broken template fails here, naming the file, instead of at the
	// project's first compile.
	Go bool
}

// Insert is one line of app/setup.go.
type Insert struct {
	// Marker is what makes the insertion recognisable on a second run.
	Marker string
	// Line is the code, as a template.
	Line string
}

// Options is what `trilha add` was asked for.
type Options struct {
	Module string
	Lang   string
	// At is the folder the screens land under, slash-terminated. Empty is
	// "app/", which is where somebody adding one recipe to their own project
	// wants it; the app template asks for "app/admin/", because screens that
	// name people and issue credentials do not belong behind the same door as
	// a listing of items.
	//
	// It moves the screens and nothing else: what a recipe writes under
	// internal/ is not a screen and has no business moving with one.
	At string
	// DryRun prints what would happen and writes nothing. It is what somebody
	// runs before letting a command touch a project that already has code.
	DryRun bool
}

// Result is what happened, so the command can print it and a test can assert
// on it.
type Result struct {
	Written []string
	Skipped []string
	Setup   []string
	Doc     string
	Next    string
}

// All is every recipe, by name.
func All() []Recipe {
	out := []Recipe{apiKeysRecipe(), approvalsRecipe(), auditRecipe(), blobRecipe(), connectionsRecipe(), loginRecipe(), mailRecipe(), permissionsRecipe(), profileRecipe(), searchRecipe(), shareLinkRecipe(), settingsRecipe(), tasksRecipe(), tenantRecipe(), usersRecipe(), webhooksRecipe()}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get finds one.
func Get(name string) (Recipe, error) {
	for _, r := range All() {
		if r.Name == name {
			return r, nil
		}
	}
	return Recipe{}, fmt.Errorf("%w: %q", ErrUnknown, name)
}

// Add writes the recipe into the project.
func Add(root string, r Recipe, o Options) (Result, error) {
	var res Result
	res.Doc = r.Doc

	// What it needs comes before what it writes: half a recipe in a project is
	// worse than none, because the person now has files to delete before they
	// can try again.
	for _, need := range r.Needs {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(need.File))); err != nil {
			return res, fmt.Errorf("%w: run `trilha add %s` first (%s is not there)",
				ErrMissing, need.Recipe, need.File)
		}
	}

	at := o.At
	if at == "" {
		at = "app/"
	}
	if !strings.HasSuffix(at, "/") {
		at += "/"
	}
	// URL is the address side of At: the screens have to link and redirect to
	// where they actually landed. Writing "/chaves" in a recipe that a
	// template puts under /admin/ is a form that posts to a 404 — and the
	// kind of mistake nobody sees until they press the button.
	url := "/" + strings.TrimPrefix(at, "app/")
	dados := map[string]any{"Module": o.Module, "Lang": o.Lang, "At": at, "URL": url, "T": words(o.Lang)}

	// Everything is rendered before anything is written: a template that
	// produces broken Go fails here, naming the file, rather than at the
	// project's first compile with a line number into code nobody wrote.
	type pronto struct{ rel, body string }
	var arquivos []pronto
	for _, f := range r.Files {
		rel, err := render(f.Rel, dados)
		if err != nil {
			return res, err
		}
		body, err := render(f.Body, dados)
		if err != nil {
			return res, fmt.Errorf("%s: %w", rel, err)
		}
		if f.Go {
			src, err := format.Source([]byte(body))
			if err != nil {
				return res, fmt.Errorf("%s: %w", rel, err)
			}
			body = string(src)
		}
		arquivos = append(arquivos, pronto{rel, body})
	}

	var err error
	for _, f := range arquivos {
		abs := filepath.Join(root, filepath.FromSlash(f.rel))
		if _, err := os.Stat(abs); err == nil {
			// Skipped and not refused: somebody running add a second time is
			// adding, not starting over.
			res.Skipped = append(res.Skipped, f.rel)
			continue
		}
		res.Written = append(res.Written, f.rel)
		if o.DryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(abs, []byte(f.body), 0o644); err != nil {
			return res, err
		}
	}

	// The next step names the address the screens actually landed on, so it
	// is a template like everything else.
	if res.Next, err = render(pick(r.Next, o.Lang), dados); err != nil {
		return res, err
	}

	linhas, err := r.setupLines(dados)
	if err != nil {
		return res, err
	}
	feitas, err := writeSetup(root, r, linhas, dados, o.DryRun)
	if err != nil {
		return res, err
	}
	res.Setup = feitas
	return res, nil
}

func (r Recipe) setupLines(dados map[string]any) ([]Insert, error) {
	out := make([]Insert, 0, len(r.Setup))
	for _, in := range r.Setup {
		linha, err := render(in.Line, dados)
		if err != nil {
			return nil, err
		}
		out = append(out, Insert{Marker: in.Marker, Line: linha})
	}
	return out, nil
}

func render(tmpl string, dados map[string]any) (string, error) {
	t, err := template.New("r").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, dados); err != nil {
		return "", err
	}
	return b.String(), nil
}

func pick(m map[string]string, lang string) string {
	if v, ok := m[lang]; ok {
		return v
	}
	return m["en"]
}
