// Package scan walks an app/ directory and turns its file conventions into a
// list of routes, validating them along the way. It only parses declarations
// (go/parser with SkipObjectResolution) — the compiler checks signatures when
// the generated file is built.
package scan

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Error codes reported by the scanner.
const (
	ErrPageAndRoute     = "E_PAGE_AND_ROUTE"
	ErrNoPageFunc       = "E_NO_PAGE_FUNC"
	ErrNoMethod         = "E_NO_METHOD"
	ErrNoLayoutFunc     = "E_NO_LAYOUT_FUNC"
	ErrNoMiddlewareFunc = "E_NO_MIDDLEWARE_FUNC"
	ErrNoNotFoundFunc   = "E_NO_NOT_FOUND_FUNC"
	ErrNoErrorFunc      = "E_NO_ERROR_FUNC"
	ErrNoSetupFunc      = "E_NO_SETUP_FUNC"
	ErrAmbiguousSegment = "E_AMBIGUOUS_SEGMENT"
	ErrCatchAllNotLeaf  = "E_CATCHALL_NOT_LEAF"
	ErrBadSegment       = "E_BAD_SEGMENT"
	ErrDuplicateRoute   = "E_DUPLICATE_ROUTE"
	ErrParse            = "E_PARSE"
	ErrNoApp            = "E_NO_APP"
)

var methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// Error is one convention violation.
type Error struct {
	File string
	Code string
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s: %s", e.File, e.Code, e.Msg) }

// Errors is a list of violations; the scanner reports all it finds.
type Errors []*Error

func (es Errors) Error() string {
	var sb strings.Builder
	for i, e := range es {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(e.Error())
	}
	return sb.String()
}

// Ref points at an exported function in a route package.
type Ref struct {
	Alias      string
	ImportPath string
	Func       string
}

// Route is one page or API route.
type Route struct {
	Pattern    string
	Kind       string // "page" | "api"
	Dir        string // relative to the project root, slash-separated (app/blog/slug_)
	ImportPath string
	Alias      string
	Methods    []string // sorted
	HasPage    bool
	// HasKind is true when route.go exports `var Kind = trilha.KindPage|KindAPI`.
	HasKind     bool
	Layouts     []Ref // innermost first
	Middlewares []Ref // outermost first
}

// Result is the scanned application.
type Result struct {
	Module     string
	AppDir     string
	Routes     []Route // sorted by Pattern
	RootLayout *Ref
	NotFound   *Ref
	ErrorPage  *Ref
	Setup      *Ref
	// ConfigFunc is the optional func Config(cfg *trilha.Config) in setup.go,
	// called before trilha.New.
	ConfigFunc *Ref
	// ShutdownFunc is the optional func Shutdown(a *trilha.App) error in setup.go.
	ShutdownFunc *Ref
	// HasMain is true when a non-generated file of the root package already
	// declares func main(); the generator then omits its own.
	HasMain   bool
	HasPublic bool
	Imports   []Import // sorted by Alias
}

// Import is one package the generated file must import.
type Import struct {
	Alias string
	Path  string
}

// Scan reads <root>/app and returns the routes. module is the Go module path
// of the project (from go.mod).
func Scan(root, module string) (*Result, error) {
	appDir := filepath.Join(root, "app")
	if st, err := os.Stat(appDir); err != nil || !st.IsDir() {
		return nil, Errors{{File: "app", Code: ErrNoApp, Msg: "app/ directory not found"}}
	}
	s := &scanner{root: root, module: module, aliases: map[string]bool{}, imports: map[string]string{}}
	s.walk(appDir, "app", nil, nil, nil)
	s.res.Module = module
	s.res.AppDir = "app"
	if st, err := os.Stat(filepath.Join(root, "public")); err == nil && st.IsDir() {
		s.res.HasPublic = dirHasFiles(filepath.Join(root, "public"))
	}
	s.res.HasMain = rootHasMain(root)
	sort.SliceStable(s.res.Routes, func(i, j int) bool { return s.res.Routes[i].Pattern < s.res.Routes[j].Pattern })
	for i := 1; i < len(s.res.Routes); i++ {
		a, b := s.res.Routes[i-1], s.res.Routes[i]
		if a.Pattern == b.Pattern {
			s.errf(b.Dir, ErrDuplicateRoute, "pattern %s is already served by %s (route groups cannot repeat the same URL)", b.Pattern, a.Dir)
		}
	}
	for alias, p := range s.imports {
		s.res.Imports = append(s.res.Imports, Import{Alias: alias, Path: p})
	}
	sort.Slice(s.res.Imports, func(i, j int) bool { return s.res.Imports[i].Alias < s.res.Imports[j].Alias })
	if len(s.errs) > 0 {
		sort.Slice(s.errs, func(i, j int) bool { return s.errs[i].File+s.errs[i].Code < s.errs[j].File+s.errs[j].Code })
		return &s.res, s.errs
	}
	return &s.res, nil
}

type scanner struct {
	root    string
	module  string
	res     Result
	errs    Errors
	aliases map[string]bool
	imports map[string]string
}

func (s *scanner) errf(file, code, format string, a ...any) {
	s.errs = append(s.errs, &Error{File: filepath.ToSlash(file), Code: code, Msg: fmt.Sprintf(format, a...)})
}

type segment struct {
	literal string
	name    string
	kind    int // 0 literal, 1 param, 2 catch-all, 3 group (no URL segment)
}

const kindGroup = 3

func parseSegment(dir string) (segment, error) {
	switch {
	case strings.HasSuffix(dir, "-"):
		n := strings.TrimSuffix(dir, "-")
		if strings.HasSuffix(n, "_") {
			return segment{}, fmt.Errorf("route group (%q) cannot be dynamic", dir)
		}
		if n == "" {
			return segment{}, fmt.Errorf("route group needs a name before the \"-\"")
		}
		return segment{name: n, kind: kindGroup}, nil
	case strings.HasSuffix(dir, "__"):
		n := strings.TrimSuffix(dir, "__")
		if !token.IsIdentifier(n) {
			return segment{}, fmt.Errorf("%q is not a valid identifier", n)
		}
		return segment{name: n, kind: 2}, nil
	case strings.HasSuffix(dir, "_"):
		n := strings.TrimSuffix(dir, "_")
		if !token.IsIdentifier(n) {
			return segment{}, fmt.Errorf("%q is not a valid identifier", n)
		}
		return segment{name: n, kind: 1}, nil
	default:
		return segment{literal: dir}, nil
	}
}

func (seg segment) pattern() string {
	switch seg.kind {
	case 1:
		return "{" + seg.name + "}"
	case 2:
		return "{" + seg.name + "...}"
	}
	return seg.literal
}

// walk visits one directory. rel is slash-separated relative to root.
func (s *scanner) walk(abs, rel string, segs []segment, layouts, mws []Ref) {
	files, err := os.ReadDir(abs)
	if err != nil {
		s.errf(rel, ErrParse, "%v", err)
		return
	}
	var goFiles, subdirs []string
	present := map[string]bool{}
	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || name == "testdata" {
				continue
			}
			subdirs = append(subdirs, name)
			continue
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			goFiles = append(goFiles, name)
			present[name] = true
		}
	}
	sort.Strings(subdirs)

	pkg := s.parsePackage(abs, rel, goFiles)
	if pkg.broken {
		for _, fn := range append([]string{"Page", "Layout", "Middleware", "NotFound", "Error", "Setup"}, methods...) {
			pkg.funcs[fn] = true
		}
	}
	importPath := path.Join(s.module, rel)
	alias := s.alias(rel)
	isRoot := rel == "app"

	// Layout / middleware for this subtree (own dir included).
	if present["layout.go"] {
		if pkg.funcs["Layout"] {
			ref := Ref{Alias: alias, ImportPath: importPath, Func: "Layout"}
			layouts = append([]Ref{ref}, layouts...)
			if isRoot {
				s.res.RootLayout = &ref
			}
			s.use(alias, importPath)
		} else {
			s.errf(rel+"/layout.go", ErrNoLayoutFunc, "layout.go must export func Layout(c *trilha.Ctx, children h.Node) (h.Node, error)")
		}
	}
	if present["middleware.go"] {
		if pkg.funcs["Middleware"] {
			mws = append(append([]Ref{}, mws...), Ref{Alias: alias, ImportPath: importPath, Func: "Middleware"})
			s.use(alias, importPath)
		} else {
			s.errf(rel+"/middleware.go", ErrNoMiddlewareFunc, "middleware.go must export func Middleware(c *trilha.Ctx, next trilha.Next) error")
		}
	}
	if isRoot {
		s.rootFile(present, pkg, rel, alias, importPath, "not_found.go", "NotFound", ErrNoNotFoundFunc, "func NotFound(c *trilha.Ctx) (h.Node, error)", &s.res.NotFound)
		s.rootFile(present, pkg, rel, alias, importPath, "error.go", "Error", ErrNoErrorFunc, "func Error(c *trilha.Ctx, err error) (h.Node, error)", &s.res.ErrorPage)
		s.rootFile(present, pkg, rel, alias, importPath, "setup.go", "Setup", ErrNoSetupFunc, "func Setup(a *trilha.App) error", &s.res.Setup)
		if present["setup.go"] && pkg.funcs["Config"] {
			s.res.ConfigFunc = &Ref{Alias: alias, ImportPath: importPath, Func: "Config"}
			s.use(alias, importPath)
		}
		if present["setup.go"] && pkg.funcs["Shutdown"] {
			s.res.ShutdownFunc = &Ref{Alias: alias, ImportPath: importPath, Func: "Shutdown"}
			s.use(alias, importPath)
		}
	}

	// Route for this directory.
	if present["page.go"] && present["route.go"] {
		s.errf(rel, ErrPageAndRoute, "a directory serves either a page (page.go) or an API (route.go), never both")
	} else if present["page.go"] || present["route.go"] {
		r := Route{Dir: rel, ImportPath: importPath, Alias: alias, Layouts: layouts, Middlewares: mws, Pattern: patternOf(segs)}
		if present["page.go"] {
			r.Kind = "page"
			if !pkg.funcs["Page"] {
				s.errf(rel+"/page.go", ErrNoPageFunc, "page.go must export func Page(c *trilha.Ctx) (h.Node, error)")
			}
			r.HasPage = pkg.funcs["Page"]
			for _, m := range methods[1:] {
				if pkg.funcs[m] {
					r.Methods = append(r.Methods, m)
				}
			}
		} else {
			r.Kind = "api"
			r.HasKind = pkg.vars["Kind"]
			r.Layouts = nil
			for _, m := range methods {
				if pkg.funcs[m] {
					r.Methods = append(r.Methods, m)
				}
			}
			if len(r.Methods) == 0 {
				s.errf(rel+"/route.go", ErrNoMethod, "route.go must export at least one of GET, POST, PUT, PATCH, DELETE with signature func(c *trilha.Ctx) error")
			}
		}
		sort.Strings(r.Methods)
		if len(r.Methods) > 0 || r.HasPage {
			s.use(alias, importPath)
			s.res.Routes = append(s.res.Routes, r)
		}
	}

	// Children.
	dynamic := 0
	for _, d := range subdirs {
		seg, err := parseSegment(d)
		if err != nil {
			s.errf(rel+"/"+d, ErrBadSegment, "%v", err)
			continue
		}
		if seg.kind == 1 || seg.kind == 2 {
			dynamic++
		}
		childSegs := append(append([]segment{}, segs...), seg)
		if len(segs) > 0 && segs[len(segs)-1].kind == 2 {
			if s.hasRoutes(filepath.Join(abs, d)) {
				s.errf(rel+"/"+d, ErrCatchAllNotLeaf, "catch-all (%s) must be a leaf; it cannot have routes below it", segs[len(segs)-1].name+"__")
				continue
			}
		}
		s.walk(filepath.Join(abs, d), rel+"/"+d, childSegs, layouts, mws)
	}
	if dynamic > 1 {
		s.errf(rel, ErrAmbiguousSegment, "more than one dynamic directory (name_ or name__) at the same level is ambiguous")
	}
}

func (s *scanner) rootFile(present map[string]bool, pkg pkgInfo, rel, alias, importPath, file, fn, code, sig string, dst **Ref) {
	if !present[file] {
		return
	}
	if !pkg.funcs[fn] {
		s.errf(rel+"/"+file, code, "%s must export %s", file, sig)
		return
	}
	*dst = &Ref{Alias: alias, ImportPath: importPath, Func: fn}
	s.use(alias, importPath)
}

func (s *scanner) hasRoutes(abs string) bool {
	found := false
	_ = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if !d.IsDir() && (d.Name() == "page.go" || d.Name() == "route.go") {
			found = true
		}
		return nil
	})
	return found
}

func patternOf(segs []segment) string {
	if len(segs) == 0 {
		return "/"
	}
	parts := make([]string, 0, len(segs))
	for _, seg := range segs {
		if seg.kind == kindGroup {
			continue
		}
		parts = append(parts, seg.pattern())
	}
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}

func (s *scanner) use(alias, importPath string) { s.imports[alias] = importPath }

// alias derives a unique, valid Go identifier for an import.
func (s *scanner) alias(rel string) string {
	var b strings.Builder
	for _, r := range rel {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	base := b.String()
	if base == "" || (base[0] >= '0' && base[0] <= '9') {
		base = "p" + base
	}
	if p, ok := s.imports[base]; ok && p == path.Join(s.module, rel) {
		return base
	}
	a := base
	for i := 2; s.aliases[a]; i++ {
		a = fmt.Sprintf("%s%d", base, i)
	}
	s.aliases[a] = true
	return a
}

type pkgInfo struct {
	name   string
	funcs  map[string]bool
	vars   map[string]bool // exported package-level var/const names
	broken bool            // a file failed to parse: skip "missing func" checks
}

// parsePackage collects exported top-level functions across the dir's files.
func (s *scanner) parsePackage(abs, rel string, files []string) pkgInfo {
	info := pkgInfo{funcs: map[string]bool{}, vars: map[string]bool{}}
	fset := token.NewFileSet()
	for _, f := range files {
		file, err := parser.ParseFile(fset, filepath.Join(abs, f), nil, parser.SkipObjectResolution)
		if err != nil {
			s.errf(rel+"/"+f, ErrParse, "%v", stripPath(err, abs))
			info.broken = true
			continue
		}
		info.name = file.Name.Name
		for _, d := range file.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					info.funcs[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, sp := range d.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.IsExported() {
								info.vars[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return info
}

// rootHasMain reports whether a hand-written .go file in the project root
// (package main, not trilha_gen.go, not a test) declares func main().
func rootHasMain(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "trilha_gen.go" {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, name), nil, parser.SkipObjectResolution)
		if err != nil || file.Name.Name != "main" {
			continue
		}
		for _, d := range file.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "main" {
				return true
			}
		}
	}
	return false
}

func stripPath(err error, abs string) error {
	return errors.New(strings.ReplaceAll(err.Error(), abs+string(filepath.Separator), ""))
}

func dirHasFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
