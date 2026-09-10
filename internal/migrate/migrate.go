// Package migrate reads the app/ of a Next.js project and writes the app/ of a
// Trilha one: the folders the convention asks for, files that compile, and a
// report of what each screen does. It is the mechanical half of a migration —
// the half that costs a lot of tokens and no thought.
//
// What it does not do is translate JSX: the tree and the map are what is
// deterministic, and the screen is the work of whoever ports it.
package migrate

import (
	"errors"
	"fmt"
	"go/format"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

// The kinds of Next file that have somewhere to go here.
const (
	KindPage     = "page"
	KindLayout   = "layout"
	KindRoute    = "route"
	KindNotFound = "not-found"
	KindError    = "error"
)

// Page is one file of the Next app and the file that answers for it here.
type Page struct {
	Source  string   // path of the .tsx, relative to the project root
	Lines   int      // lines of the source, so a reader can size the job
	Kind    string   // KindPage, KindLayout, KindRoute...
	URL     string   // the URL it answers, spelled as Trilha spells it
	Dir     string   // folder under app/, "" for the root
	File    string   // page.go, layout.go, route.go...
	Pkg     string   // package clause of the generated file
	Params  []string // parameter names, in order
	Methods []string // handlers found in a route.ts
	Analysis
}

// Note is something the command will not translate. A note is not a failure:
// it is the line of the report that says a decision is somebody's to make.
type Note struct {
	Source string
	Code   string
	Detail string
}

// Codes of Note, so the report and the tests agree on the names.
const (
	NoteParallel    = "parallel-route"
	NoteIntercept   = "intercepting-route"
	NoteLoading     = "loading"
	NoteTemplate    = "template"
	NoteDefault     = "default"
	NoteMiddleware  = "middleware"
	NoteRewrite     = "rewrite"
	NoteRootLayout  = "root-layout"
	NoteNestedFile  = "nested-root-file"
	NoteRenamed     = "renamed-param"
	NoteDuplicate   = "duplicate-route"
	NoteOptionalAll = "optional-catch-all"
)

// Project is what Scan found: the pages in the order the tree gives them, and
// everything that has no equivalent here.
type Project struct {
	Root   string // the directory that was read
	AppDir string // the app/ inside it, relative to Root
	Pages  []Page
	Notes  []Note
	// Globals is the frame: what the layouts reach, with the signals that make
	// it work. It is ported once — as the layout, or as one island inside it —
	// and it is here instead of inside every screen that happens to sit under
	// it.
	Globals []Global

	global map[string]string // the same files, by path, while scanning
}

// Global is one file the layouts reach: the shell, the provider, the chat the
// shell carries.
type Global struct {
	Path    string   // relative to Root, slash-separated
	Lines   int      // the size of that piece of the job
	Signals []string // what it does that a form does not
	Class   string   // what it would be if it were a screen
}

// nextFiles maps the base name of a Next file to what it becomes here.
var nextFiles = map[string]struct{ kind, file string }{
	"page":      {KindPage, "page.go"},
	"layout":    {KindLayout, "layout.go"},
	"route":     {KindRoute, "route.go"},
	"not-found": {KindNotFound, "not_found.go"},
	"error":     {KindError, "error.go"},
}

// noEquivalent maps the base name of a Next file that has nowhere to go.
var noEquivalent = map[string]string{
	"loading":  NoteLoading,
	"template": NoteTemplate,
	"default":  NoteDefault,
}

var sourceExts = map[string]bool{".tsx": true, ".ts": true, ".jsx": true, ".js": true}

// Scan reads the Next project at root. It accepts the project root (with app/
// or src/app/ inside it) or the app directory itself, because both are what
// somebody has open when they think of running this.
func Scan(root string) (Project, error) {
	p := Project{Root: root}
	appDir, err := findApp(root)
	if err != nil {
		return p, err
	}
	p.AppDir = appDir
	abs := filepath.Join(root, filepath.FromSlash(appDir))
	// The frame is read first, because every screen after this is classified
	// against it: what the layout reaches is the application's work, not the
	// screen's.
	p.global = globalFiles(root, abs)
	seen := map[string]string{}
	err = filepath.WalkDir(abs, func(fp string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			return nil
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if !sourceExts[filepath.Ext(d.Name())] {
			return nil
		}
		rel := filepath.ToSlash(mustRel(root, fp))
		if code, ok := noEquivalent[base]; ok {
			p.Notes = append(p.Notes, Note{Source: rel, Code: code})
			return nil
		}
		target, ok := nextFiles[base]
		if !ok {
			return nil
		}
		dirRel := filepath.ToSlash(mustRel(abs, filepath.Dir(fp)))
		if dirRel == "." {
			dirRel = ""
		}
		pg, notes, skip := p.page(rel, dirRel, target.kind, target.file)
		p.Notes = append(p.Notes, notes...)
		if skip {
			return nil
		}
		key := path.Join(pg.Dir, pg.File)
		if first, dup := seen[key]; dup {
			p.Notes = append(p.Notes, Note{Source: rel, Code: NoteDuplicate, Detail: first})
			return nil
		}
		seen[key] = rel
		p.Pages = append(p.Pages, pg)
		return nil
	})
	if err != nil {
		return p, err
	}
	p.Notes = append(p.Notes, p.rootNotes()...)
	sort.SliceStable(p.Notes, func(i, j int) bool { return p.Notes[i].Source < p.Notes[j].Source })
	p.Globals = globalsOf(p.global)
	return p, nil
}

// globalSet is the membership test analyze needs.
func (p *Project) globalSet() map[string]bool {
	out := make(map[string]bool, len(p.global))
	for k := range p.global {
		out[k] = true
	}
	return out
}

// globalFiles reads every layout and template of the app and returns what they
// reach, by path. Those two are what the App Router defines as the frame; a
// providers.tsx that only the layout imports arrives through the reach, with no
// rule that has to guess from a file name.
func globalFiles(root, appAbs string) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(appAbs, func(fp string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if !sourceExts[filepath.Ext(d.Name())] || (base != "layout" && base != "template") {
			return nil
		}
		for k, v := range deps(root, filepath.ToSlash(mustRel(root, fp))) {
			out[k] = v
		}
		return nil
	})
	return out
}

// globalsOf keeps the global files that are work: a frame that does nothing a
// form cannot do needs no line in the report.
func globalsOf(files map[string]string) []Global {
	var out []Global
	for _, path := range sortedKeys(files) {
		a := analyze(files[path], nil, nil)
		if len(a.Signals) == 0 {
			continue
		}
		out = append(out, Global{
			Path:    path,
			Lines:   strings.Count(files[path], "\n") + 1,
			Signals: a.Signals,
			Class:   a.Class,
		})
	}
	return out
}

// findApp locates the app directory: the one given, or app/ or src/app/ under
// it.
func findApp(root string) (string, error) {
	for _, c := range []string{"app", "src/app", "."} {
		dir := filepath.Join(root, filepath.FromSlash(c))
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() && hasRoute(dir) {
			if c == "." {
				return ".", nil
			}
			return c, nil
		}
	}
	return "", errors.New("no Next app directory here: expected app/ or src/app/ with a page or route file in it")
}

// hasRoute says whether the directory tree holds at least one file the App
// Router recognises — the cheapest way to tell app/ from a folder called app.
func hasRoute(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(fp string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || found {
			return nil
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if sourceExts[filepath.Ext(d.Name())] {
			if _, ok := nextFiles[base]; ok {
				found = true
			}
		}
		return nil
	})
	return found
}

// page turns one Next file into the file that answers for it, or into the
// notes that say why it has no answer.
func (p *Project) page(source, dirRel, kind, file string) (Page, []Note, bool) {
	var notes []Note
	var dirs, params []string
	var url strings.Builder
	if dirRel != "" {
		for _, raw := range strings.Split(dirRel, "/") {
			seg, err := segment(raw)
			if err != nil {
				notes = append(notes, Note{Source: source, Code: err.code, Detail: raw})
				return Page{}, notes, true
			}
			if seg.note != "" {
				notes = append(notes, Note{Source: source, Code: seg.note, Detail: raw})
			}
			dirs = append(dirs, seg.dir)
			if seg.param != "" {
				params = append(params, seg.param)
			}
			if seg.url != "" {
				url.WriteString("/")
				url.WriteString(seg.url)
			}
		}
	}
	// not-found.go and error.go answer for the whole app: nested ones have
	// nowhere to go, and saying so is more useful than writing a file that
	// never runs.
	if (kind == KindNotFound || kind == KindError) && len(dirs) > 0 {
		notes = append(notes, Note{Source: source, Code: NoteNestedFile})
		return Page{}, notes, true
	}
	// The root layout of a Trilha project is a whole <html> document, written
	// by trilha new. Overwriting it with a skeleton would trade something that
	// works for something that compiles.
	if kind == KindLayout && len(dirs) == 0 {
		notes = append(notes, Note{Source: source, Code: NoteRootLayout})
		return Page{}, notes, true
	}
	dir := path.Join(dirs...)
	u := url.String()
	if u == "" {
		u = "/"
	}
	pg := Page{
		Source: source,
		Kind:   kind,
		URL:    u,
		Dir:    dir,
		File:   file,
		Pkg:    packageOf(dirs),
		Params: params,
	}
	body, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(source)))
	if err == nil {
		pg.Lines = strings.Count(string(body), "\n") + 1
		pg.Analysis = analyze(string(body), deps(p.Root, source), p.globalSet())
		if kind == KindRoute {
			pg.Methods = handlers(string(body))
		}
	}
	return pg, notes, false
}

// rootNotes reads what sits beside app/: the middleware and the config.
func (p *Project) rootNotes() []Note {
	var out []Note
	for _, name := range []string{"middleware.ts", "middleware.js", "src/middleware.ts", "src/middleware.js"} {
		if _, err := os.Stat(filepath.Join(p.Root, filepath.FromSlash(name))); err == nil {
			out = append(out, Note{Source: name, Code: NoteMiddleware})
		}
	}
	for _, name := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		body, err := os.ReadFile(filepath.Join(p.Root, name))
		if err != nil {
			continue
		}
		for _, r := range rewrites(string(body)) {
			out = append(out, Note{Source: name, Code: NoteRewrite, Detail: r})
		}
	}
	return out
}

type segResult struct {
	dir   string // folder name here
	url   string // URL segment, empty for a group
	param string
	note  string
}

type segError struct{ code string }

func (e segError) Error() string { return e.code }

// segment translates one Next folder name. The rules are the inverse of the
// ones internal/scan reads, so what comes out is a tree the scanner accepts.
func segment(raw string) (segResult, *segError) {
	switch {
	case strings.HasPrefix(raw, "@"):
		return segResult{}, &segError{NoteParallel}
	case strings.HasPrefix(raw, "(.)") || strings.HasPrefix(raw, "(..)") || strings.HasPrefix(raw, "(...)"):
		return segResult{}, &segError{NoteIntercept}
	case strings.HasPrefix(raw, "(") && strings.HasSuffix(raw, ")"):
		// A route group: it wraps the subtree and adds no URL segment.
		return segResult{dir: identifier(raw[1:len(raw)-1]) + "-"}, nil
	case strings.HasPrefix(raw, "[[...") && strings.HasSuffix(raw, "]]"):
		name := identifier(raw[5 : len(raw)-2])
		return segResult{dir: name + "__", url: "{" + name + "...}", param: name, note: NoteOptionalAll}, nil
	case strings.HasPrefix(raw, "[...") && strings.HasSuffix(raw, "]"):
		name := identifier(raw[4 : len(raw)-1])
		return segResult{dir: name + "__", url: "{" + name + "...}", param: name}, nil
	case strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]"):
		inner := raw[1 : len(raw)-1]
		name := identifier(inner)
		r := segResult{dir: name + "_", url: "{" + name + "}", param: name}
		if name != inner {
			r.note = NoteRenamed
		}
		return r, nil
	default:
		return segResult{dir: raw, url: raw}, nil
	}
}

// identifier turns whatever Next allowed in a folder name into a Go
// identifier: user-id becomes user_id, and so does the c.Param that reads it.
func identifier(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteRune('_')
		}
	}
	out := sb.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		out = "p" + out
	}
	return out
}

// packageOf is the package clause of the generated file: the same rule the
// generator of skeletons uses, so two files in the same folder agree.
func packageOf(dirs []string) string {
	if len(dirs) == 0 {
		return "app"
	}
	return scaffold.PackageName(dirs[len(dirs)-1])
}

// File is one file the migration writes.
type File struct {
	Path string // relative to the output root
	Body string
}

// Files renders every page as the Go file that answers for it, in the order
// they were found: same tree, same bytes.
func (p Project) Files() []File {
	out := make([]File, 0, len(p.Pages))
	for _, pg := range p.Pages {
		out = append(out, File{Path: path.Join(pg.Dir, pg.File), Body: render(pg)})
	}
	return out
}

// Result is what happened to one file.
type Result struct {
	Path   string
	Action string // "created" or "kept"
}

// Written is the action of a file this run wrote.
const (
	Created = "created"
	Kept    = "kept"
)

// Write puts the files under root. A file that is already there is kept
// unless force: running the command a second time, after three screens have
// been ported, must not cost the three screens.
func Write(root string, p Project, force bool) ([]Result, error) {
	var out []Result
	for _, f := range p.Files() {
		dst := filepath.Join(root, filepath.FromSlash(f.Path))
		if _, err := os.Stat(dst); err == nil && !force {
			out = append(out, Result{Path: f.Path, Action: Kept})
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return out, err
		}
		if err := os.WriteFile(dst, []byte(f.Body), 0o644); err != nil {
			return out, err
		}
		out = append(out, Result{Path: f.Path, Action: Created})
	}
	return out, nil
}

// render writes the Go file of one page: the skeleton, and above it the ten
// lines of context somebody would otherwise go and read three hundred lines
// to find.
func render(pg Page) string {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", pg.Pkg)
	imports := []string{`"github.com/emersonjoe/trilha"`}
	switch pg.Kind {
	case KindRoute:
		imports = append([]string{`"net/http"`, ""}, imports...)
	case KindLayout:
		imports = append(imports, `"github.com/emersonjoe/trilha/h"`)
	default:
		imports = append(imports, `"github.com/emersonjoe/trilha/h"`, `"github.com/emersonjoe/trilha/ui"`)
	}
	b.WriteString("import (\n")
	for _, im := range imports {
		if im == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString("\t" + im + "\n")
	}
	b.WriteString(")\n\n")
	b.WriteString(comment(pg))
	switch pg.Kind {
	case KindRoute:
		for i, m := range pg.Methods {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "func %s(c *trilha.Ctx) error {\n", m)
			fmt.Fprintf(&b, "\treturn trilha.Errorf(http.StatusNotImplemented, %q)\n}\n", m+" "+pg.URL+" has not been ported yet")
		}
	case KindLayout:
		b.WriteString("func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {\n\treturn children, nil\n}\n")
	case KindNotFound:
		b.WriteString("func NotFound(c *trilha.Ctx) (h.Node, error) {\n")
		b.WriteString("\treturn ui.Container(ui.H1(h.Text(\"Not found\"))), nil\n}\n")
	case KindError:
		b.WriteString("func Error(c *trilha.Ctx, err error) (h.Node, error) {\n")
		b.WriteString("\treturn ui.Container(ui.H1(h.Text(\"Something went wrong\"))), nil\n}\n")
	default:
		title := titleOf(pg.URL)
		b.WriteString("func Page(c *trilha.Ctx) (h.Node, error) {\n")
		fmt.Fprintf(&b, "\tc.SetTitle(%q)\n", title)
		b.WriteString("\treturn ui.Container(\n")
		fmt.Fprintf(&b, "\t\tui.H1(h.Text(%q)),\n", title)
		for _, p := range pg.Params {
			fmt.Fprintf(&b, "\t\th.P(h.Text(c.Param(%q))),\n", p)
		}
		b.WriteString("\t), nil\n}\n")
	}
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return b.String()
	}
	return string(src)
}

// comment is the doc comment of the generated function: what the source file
// says about itself, in the place where it will be read.
func comment(pg Page) string {
	var b strings.Builder
	switch pg.Kind {
	case KindRoute:
		fmt.Fprintf(&b, "// %s, ported from %s.\n", pg.URL, pg.Source)
	case KindLayout:
		fmt.Fprintf(&b, "// Layout wraps %s, ported from %s.\n", pg.URL, pg.Source)
	case KindNotFound, KindError:
		fmt.Fprintf(&b, "// Ported from %s.\n", pg.Source)
	default:
		fmt.Fprintf(&b, "// Page renders GET %s, ported from %s.\n", pg.URL, pg.Source)
	}
	b.WriteString("//\n")
	fmt.Fprintf(&b, "// Source: %d lines, %s.\n", pg.Lines, describeHooks(pg.Analysis))
	if len(pg.Endpoints) > 0 {
		fmt.Fprintf(&b, "// Calls: %s.\n", strings.Join(endpointStrings(pg.Endpoints), ", "))
	}
	// The suggestion is about a screen. A route.ts has no screen, and a
	// layout is a frame: classifying either would be noise in the one place
	// somebody is going to read.
	if pg.Kind == KindPage && pg.Class != "" {
		fmt.Fprintf(&b, "// Suggested: %s — %s.\n", pg.Class, pg.Why)
	}
	return b.String()
}

// titleOf is the name of the screen: the last literal segment, or Home.
func titleOf(u string) string {
	segs := strings.Split(strings.Trim(u, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		s := segs[i]
		if s == "" || strings.HasPrefix(s, "{") {
			continue
		}
		s = strings.ReplaceAll(strings.ReplaceAll(s, "-", " "), "_", " ")
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return "Home"
}

func mustRel(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}
