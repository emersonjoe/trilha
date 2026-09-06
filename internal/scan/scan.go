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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Error codes reported by the scanner.
const (
	ErrPageAndRoute     = "E_PAGE_AND_ROUTE"
	ErrNoPageFunc       = "E_NO_PAGE_FUNC"
	ErrNoMethod         = "E_NO_METHOD"
	ErrNoLayoutFunc     = "E_NO_LAYOUT_FUNC"
	ErrNoMiddlewareFunc = "E_NO_MIDDLEWARE_FUNC"
	ErrUnusedMethodMW   = "E_UNUSED_METHOD_MIDDLEWARE"
	ErrNoNotFoundFunc   = "E_NO_NOT_FOUND_FUNC"
	ErrNoErrorFunc      = "E_NO_ERROR_FUNC"
	ErrNoSetupFunc      = "E_NO_SETUP_FUNC"
	ErrAmbiguousSegment = "E_AMBIGUOUS_SEGMENT"
	ErrCatchAllNotLeaf  = "E_CATCHALL_NOT_LEAF"
	ErrBadSegment       = "E_BAD_SEGMENT"
	ErrDuplicateRoute   = "E_DUPLICATE_ROUTE"
	ErrParse            = "E_PARSE"
	ErrNoApp            = "E_NO_APP"
	ErrDuplicateParam   = "E_DUPLICATE_PARAM"
	ErrHiddenRoute      = "E_HIDDEN_ROUTE"
	ErrUnroutableMethod = "E_UNROUTABLE_METHOD"
	ErrCORSOnPage       = "E_CORS_ON_PAGE"
)

// WellKnown is the single dot-prefixed directory that is not skipped: RFC 8414,
// RFC 9728, RFC 8555, RFC 9116 and OpenID Discovery all publish under
// /.well-known/, so a folder named this way is a route like any other. Every
// other scan of the project (the type index of internal/openapi, the watcher of
// internal/dev) reads this constant instead of repeating the exception.
const WellKnown = ".well-known"

// Methods is every HTTP method a route.go may export a handler for, in the
// order the generated file lists them. Anything outside this list is not a
// handler for the scanner, so the generator of skeletons reads it from here
// instead of writing the list down a second time.
//
// OPTIONS is here because a preflight has to be answered by the route that owns
// the policy: Config.CORS is the whole app, and a document published under
// /.well-known/ is fetched from another origin while the other routes are
// same-origin cookies.
var Methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

// unroutableMethods are HTTP methods the router does not take from a file, so a
// function named after one of them would be dropped without a word. HEAD is not
// missing: since Go 1.22 net/http's ServeMux answers HEAD with the GET handler,
// and a second one would be two answers for the same request.
var unroutableMethods = map[string]string{
	"HEAD":    "GET already answers HEAD; write the response in func GET",
	"TRACE":   "TRACE echoes the request back and is not routable",
	"CONNECT": "CONNECT is for proxies and is not routable",
}

// GeneratedFileName is the file trilha gen writes at the project root. The
// scanner skips it when reading what the author wrote by hand.
const GeneratedFileName = "trilha_gen.go"

// Error is one convention violation. Line is 0 when the violation is about a
// directory, or about a file as a whole; Fix is the sentence that resolves it,
// and it is never empty — an error without a fix costs the reader a round trip
// to find out what to do (spec 047).
type Error struct {
	File string
	Code string
	Msg  string
	Line int
	Fix  string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", e.File, e.Line, e.Code, e.Msg)
	}
	return fmt.Sprintf("%s: %s: %s", e.File, e.Code, e.Msg)
}

// fixes is the conserto of each code. The scanner says what is wrong; this
// says what to do about it, in one line, the way trilha audit already does.
var fixes = map[string]string{
	ErrPageAndRoute:     "keep the page here and move the API to a subdirectory, or the other way around",
	ErrNoPageFunc:       "rename the function to Page, or delete page.go if this directory is not a page",
	ErrNoMethod:         "name the handler after the HTTP method it answers: func GET(c *trilha.Ctx) error",
	ErrNoLayoutFunc:     "rename the function to Layout, or delete layout.go",
	ErrNoMiddlewareFunc: "rename the function to Middleware (or MiddlewareGET, MiddlewarePOST, ...), or delete middleware.go",
	ErrUnusedMethodMW:   "remove it, or add below this directory the route it was meant to guard",
	ErrNoNotFoundFunc:   "rename the function to NotFound, or delete not_found.go",
	ErrNoErrorFunc:      "rename the function to Error, or delete error.go",
	ErrNoSetupFunc:      "rename the function to Setup, or delete setup.go",
	ErrAmbiguousSegment: "keep one dynamic directory per level: merge them, or make one a literal segment",
	ErrCatchAllNotLeaf:  "move what is below it to a sibling directory; a catch-all takes the rest of the path",
	ErrBadSegment:       "rename the directory: name_ is a parameter, name__ a catch-all, name- a group, and name must be a Go identifier",
	ErrDuplicateRoute:   "rename one of the directories, or drop the route group so the two URLs differ",
	ErrDuplicateParam:   "rename one of them: a pattern cannot bind the same name twice",
	ErrHiddenRoute:      "rename the directory without the leading dot, or start it with \"_\" if you meant to park it out of the routing",
	ErrParse:            "fix the syntax error the compiler reports; go build ./... shows the same line",
	ErrUnroutableMethod: "delete the function, or answer that request from the method the router knows",
	ErrCORSOnPage:       "move the route to a route.go, or drop the var: a page is navigation on your own site",
	ErrNoApp:            "run trilha from the project root, the directory that has app/",
}

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

// Ref points at an exported function (or, for Kind, an exported variable) in a
// route package.
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
	// KindRef points at the `var Kind = trilha.KindPage|KindAPI` that decides
	// this route: the one in its own package, or the nearest one above it.
	// Kind is inherited down the subtree like Layout and Middleware are, and
	// the deepest declaration wins. Nil for a route that no Kind reaches, and
	// always nil for a page route: a page.go is a page whatever a branch of
	// APIs above it says.
	KindRef *Ref
	// HasCORS is true when route.go exports `var CORS = trilha.CORS{...}`: the
	// cross-origin policy of this route alone, preflight included.
	HasCORS     bool
	Layouts     []Ref // innermost first
	Middlewares []Ref // outermost first
	// MiddlewaresByMethod holds the chains declared per method
	// (MiddlewareGET, MiddlewarePOST, ...), inherited down the subtree the same
	// way Middleware is. Only methods this route actually serves appear here;
	// each chain runs after Middlewares, outermost first.
	MiddlewaresByMethod map[string][]Ref
}

// Methods reports every method this route answers, GET included when a page
// serves it. It is what decides whether a MiddlewareX above it guards anything.
func (r Route) servedMethods() []string {
	out := append([]string{}, r.Methods...)
	if r.HasPage {
		out = append(out, "GET")
	}
	return out
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
	// ConfigReturnsError is true when Config returns an error, the form that
	// lets the app fail where it reads its own configuration.
	ConfigReturnsError bool
	// ShutdownFunc is the optional func Shutdown(a *trilha.App) error in setup.go.
	ShutdownFunc *Ref
	// HasMain is true when a non-generated file of the root package already
	// declares func main(); the generator then omits its own.
	HasMain bool
	// Package is the package clause the generated file must carry: the one the
	// directory already declares, so an app embedded in an existing binary is a
	// normal, importable package instead of a package main nobody can import.
	Package   string
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
	s.walk(appDir, "app", nil, nil, nil, nil, nil)
	s.res.Module = module
	s.res.AppDir = "app"
	if st, err := os.Stat(filepath.Join(root, "public")); err == nil && st.IsDir() {
		s.res.HasPublic = dirHasFiles(filepath.Join(root, "public"))
	}
	s.checkMethodMiddlewares()
	s.res.HasMain = rootHasMain(root)
	s.res.Package = RootPackage(root)
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
	// methodMW records every MiddlewareX seen, to check afterwards that each
	// one guards a route.
	methodMW []methodMW
}

func (s *scanner) errf(file, code, format string, a ...any) *Error {
	e := &Error{File: filepath.ToSlash(file), Code: code, Msg: fmt.Sprintf(format, a...), Fix: fixes[code]}
	if e.Line == 0 && code == ErrParse {
		e.Line = lineOf(e.Msg)
	}
	s.errs = append(s.errs, e)
	return e
}

// at records the line of the offending declaration; 0 leaves the error about
// the file as a whole.
func (e *Error) at(line int) *Error { e.Line = line; return e }

// withFix replaces the fix of the code with one that knows the case at hand.
func (e *Error) withFix(fix string) *Error { e.Fix = fix; return e }

// lineOf reads the line out of a "file.go:12:3: ..." message, which is how the
// parser reports itself.
func lineOf(msg string) int {
	parts := strings.SplitN(msg, ":", 3)
	if len(parts) < 3 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
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

// hiddenRoutes reports the pages and APIs buried in a directory the scanner is
// about to skip. Only the dot is loud: a leading underscore and testdata are the
// documented way to keep a folder out of the routing, and the message points at
// them. Without this, the single symptom of a misnamed folder was a 404.
func (s *scanner) hiddenRoutes(abs, rel, name string) {
	if !strings.HasPrefix(name, ".") {
		return
	}
	dir := filepath.Join(abs, name)
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && (d.Name() == ".git" || d.Name() == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "page.go" && d.Name() != "route.go" {
			return nil
		}
		r, err := filepath.Rel(abs, p)
		if err != nil {
			return nil
		}
		s.errf(path.Join(rel, filepath.ToSlash(r)), ErrHiddenRoute,
			"%s is inside %q, a directory the scanner skips: a name that starts with a dot is not a route (%s is the only exception)",
			d.Name(), name, WellKnown)
		return nil
	})
}

// walk visits one directory. rel is slash-separated relative to root.
func (s *scanner) walk(abs, rel string, segs []segment, layouts, mws []Ref, byMethod map[string][]Ref, kind *Ref) {
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
			if name != WellKnown && (strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") || name == "testdata") {
				s.hiddenRoutes(abs, rel, name)
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
		for _, fn := range append([]string{"Page", "Layout", "Middleware", "NotFound", "Error", "Setup"}, Methods...) {
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
			other, line := pkg.instead("Layout")
			s.errf(rel+"/layout.go", ErrNoLayoutFunc, "layout.go must export func Layout(c *trilha.Ctx, children h.Node) (h.Node, error)%s", other).at(line)
		}
	}
	// Kind for this subtree (own dir included). Unlike Layout and Middleware it
	// is a variable, so any file of the package declares it; kind.go is where a
	// directory with no route.go of its own says it.
	if pkg.vars["Kind"] {
		kind = &Ref{Alias: alias, ImportPath: importPath, Func: "Kind"}
	}
	if present["middleware.go"] {
		found := false
		if pkg.funcs["Middleware"] {
			mws = append(append([]Ref{}, mws...), Ref{Alias: alias, ImportPath: importPath, Func: "Middleware"})
			found = true
		}
		if !pkg.broken {
			for _, m := range Methods {
				fn := "Middleware" + m
				if !pkg.funcs[fn] {
					continue
				}
				byMethod = withMethodRef(byMethod, m, Ref{Alias: alias, ImportPath: importPath, Func: fn})
				s.methodMW = append(s.methodMW, methodMW{dir: rel, method: m})
				found = true
			}
		}
		if found {
			s.use(alias, importPath)
		} else {
			other, line := pkg.instead(middlewareFuncs()...)
			s.errf(rel+"/middleware.go", ErrNoMiddlewareFunc, "middleware.go must export func Middleware(c *trilha.Ctx, next trilha.Next) error, or MiddlewareGET|POST|PUT|PATCH|DELETE|OPTIONS with the same signature for a single method%s", other).at(line)
		}
	}
	if isRoot {
		s.rootFile(present, pkg, rel, alias, importPath, "not_found.go", "NotFound", ErrNoNotFoundFunc, "func NotFound(c *trilha.Ctx) (h.Node, error)", &s.res.NotFound)
		s.rootFile(present, pkg, rel, alias, importPath, "error.go", "Error", ErrNoErrorFunc, "func Error(c *trilha.Ctx, err error) (h.Node, error)", &s.res.ErrorPage)
		s.rootFile(present, pkg, rel, alias, importPath, "setup.go", "Setup", ErrNoSetupFunc, "func Setup(a *trilha.App) error", &s.res.Setup)
		if present["setup.go"] && pkg.funcs["Config"] {
			s.res.ConfigFunc = &Ref{Alias: alias, ImportPath: importPath, Func: "Config"}
			s.res.ConfigReturnsError = pkg.results["Config"] > 0
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
		if name, first, second := repeatedParam(rel); name != "" {
			s.errf(rel, ErrDuplicateParam, "parameter {%s} is bound twice in this path: %s and %s", name, first, second)
		}
		r := Route{Dir: rel, ImportPath: importPath, Alias: alias, Layouts: layouts, Middlewares: mws, Pattern: patternOf(segs)}
		file := "route.go"
		if present["page.go"] {
			file = "page.go"
		}
		s.unroutable(pkg, rel+"/"+file)
		if present["page.go"] {
			r.Kind = "page"
			if !pkg.funcs["Page"] {
				other, line := pkg.instead("Page")
				s.errf(rel+"/page.go", ErrNoPageFunc, "page.go must export func Page(c *trilha.Ctx) (h.Node, error)%s", other).at(line)
			}
			if pkg.vars["CORS"] {
				// A page is navigation on the site itself; a var nobody reads is
				// the silent discard this convention exists to avoid.
				s.errf(rel+"/page.go", ErrCORSOnPage, "var CORS is a route.go declaration: a page cannot carry a cross-origin policy")
			}
			r.HasPage = pkg.funcs["Page"]
			for _, m := range Methods[1:] {
				if pkg.funcs[m] {
					r.Methods = append(r.Methods, m)
				}
			}
		} else {
			r.Kind = "api"
			r.KindRef = kind
			if kind != nil {
				s.use(kind.Alias, kind.ImportPath)
			}
			r.HasCORS = pkg.vars["CORS"]
			r.Layouts = nil
			for _, m := range Methods {
				if pkg.funcs[m] {
					r.Methods = append(r.Methods, m)
				}
			}
			if len(r.Methods) == 0 {
				other, line := pkg.instead(Methods...)
				e := s.errf(rel+"/route.go", ErrNoMethod, "route.go must export at least one of GET, POST, PUT, PATCH, DELETE, OPTIONS with signature func(c *trilha.Ctx) error%s", other).at(line)
				if m, ml := pkg.lowercaseMethod(); m != "" {
					e.at(ml).withFix("handlers are named by HTTP method in upper case: rename func " + m + " to func " + strings.ToUpper(m))
				}
			}
		}
		sort.Strings(r.Methods)
		for _, m := range r.servedMethods() {
			chain := byMethod[m]
			if len(chain) == 0 {
				continue
			}
			if r.MiddlewaresByMethod == nil {
				r.MiddlewaresByMethod = map[string][]Ref{}
			}
			r.MiddlewaresByMethod[m] = chain
		}
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
		s.walk(filepath.Join(abs, d), rel+"/"+d, childSegs, layouts, mws, byMethod, kind)
	}
	if dynamic > 1 {
		s.errf(rel, ErrAmbiguousSegment, "more than one dynamic directory (name_ or name__) at the same level is ambiguous")
	}
}

// unroutable reports a handler named after a method the router does not take
// from a file. Without this the function compiles, the generator says nothing,
// and the request answers 405 in production — the same silent discard that
// E_HIDDEN_ROUTE closed for directories.
func (s *scanner) unroutable(pkg pkgInfo, file string) {
	if pkg.broken {
		return
	}
	var names []string
	for n := range unroutableMethods {
		if pkg.funcs[n] {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		s.errf(file, ErrUnroutableMethod, "func %s is not a route: %s", n, unroutableMethods[n]).at(pkg.lines[n])
	}
}

// methodMW records where a per-method middleware was declared, so the scanner
// can tell afterwards whether it ended up guarding any route at all.
type methodMW struct {
	dir    string
	method string
}

// withMethodRef appends ref to the chain of method m without touching the map
// the caller (an ancestor directory) still holds.
func withMethodRef(byMethod map[string][]Ref, m string, ref Ref) map[string][]Ref {
	out := make(map[string][]Ref, len(byMethod)+1)
	for k, v := range byMethod {
		out[k] = v
	}
	out[m] = append(append([]Ref{}, out[m]...), ref)
	return out
}

// checkMethodMiddlewares reports a MiddlewareX that reaches no route serving
// that method. A permission that guards nothing is the failure these
// conventions exist to prevent, so it is an error and not a warning.
func (s *scanner) checkMethodMiddlewares() {
	for _, mw := range s.methodMW {
		used := false
		for _, r := range s.res.Routes {
			if r.Dir != mw.dir && !strings.HasPrefix(r.Dir, mw.dir+"/") {
				continue
			}
			for _, m := range r.servedMethods() {
				if m == mw.method {
					used = true
				}
			}
		}
		if !used {
			s.errf(mw.dir+"/middleware.go", ErrUnusedMethodMW, "Middleware%s guards no %s route in %s or below it", mw.method, mw.method, mw.dir)
		}
	}
}

func (s *scanner) rootFile(present map[string]bool, pkg pkgInfo, rel, alias, importPath, file, fn, code, sig string, dst **Ref) {
	if !present[file] {
		return
	}
	if !pkg.funcs[fn] {
		other, line := pkg.instead(fn)
		s.errf(rel+"/"+file, code, "%s must export %s%s", file, sig, other).at(line)
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

// middlewareFuncs lists every name middleware.go may export.
func middlewareFuncs() []string {
	out := []string{"Middleware"}
	for _, m := range Methods {
		out = append(out, "Middleware"+m)
	}
	return out
}

// repeatedParam finds a parameter bound twice along one path
// (app/a/id_/b/id_). net/http panics on a pattern like that, so the scanner
// stops it here, naming the two directories.
func repeatedParam(rel string) (name, first, second string) {
	seen := map[string]string{}
	var dir string
	for _, part := range strings.Split(rel, "/") {
		if dir == "" {
			dir = part
		} else {
			dir += "/" + part
		}
		seg, err := parseSegment(part)
		if err != nil || seg.kind == 0 || seg.kind == kindGroup {
			continue
		}
		if prev, ok := seen[seg.name]; ok {
			return seg.name, prev, dir
		}
		seen[seg.name] = dir
	}
	return "", "", ""
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
	name  string
	funcs map[string]bool
	// results counts the return values of each exported function: Config
	// exists with and without an error, and both stay valid.
	results map[string]int
	vars    map[string]bool // exported package-level var/const names
	// lines is the line of every top-level function, exported or not: it is
	// what lets an error point at the function that was written instead of
	// the one the convention asks for.
	lines  map[string]int
	broken bool // a file failed to parse: skip "missing func" checks
}

// instead names the functions the directory declares in place of the one the
// convention asks for, and the line of the first of them: "page.go must export
// func Page ...; found func Render" saves the reader a look at the file.
func (info pkgInfo) instead(want ...string) (string, int) {
	skip := map[string]bool{}
	for _, w := range want {
		skip[w] = true
	}
	var names []string
	for n := range info.lines {
		if !skip[n] {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return "", 0
	}
	sort.Strings(names)
	return "; found func " + strings.Join(names, ", func "), info.lines[names[0]]
}

// lowercaseMethod finds a handler written in lower case (func get), the
// mistake that deserves its own sentence instead of the generic one.
func (info pkgInfo) lowercaseMethod() (string, int) {
	var found string
	for n := range info.lines {
		up := strings.ToUpper(n)
		if up == n {
			continue
		}
		for _, m := range Methods {
			if up == m && (found == "" || n < found) {
				found = n
			}
		}
	}
	if found == "" {
		return "", 0
	}
	return found, info.lines[found]
}

// parsePackage collects exported top-level functions across the dir's files.
func (s *scanner) parsePackage(abs, rel string, files []string) pkgInfo {
	info := pkgInfo{funcs: map[string]bool{}, results: map[string]int{}, vars: map[string]bool{}, lines: map[string]int{}}
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
				if d.Recv == nil {
					info.lines[d.Name.Name] = fset.Position(d.Name.Pos()).Line
				}
				if d.Recv == nil && d.Name.IsExported() {
					info.funcs[d.Name.Name] = true
					if d.Type.Results != nil {
						info.results[d.Name.Name] = len(d.Type.Results.List)
					}
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
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == GeneratedFileName {
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

// RootPackage reports the package clause the generated file must carry. A
// hand-written file in the root decides it; failing that, an existing
// trilha_gen.go does, so `--package` is passed once and the CI's `gen --check`
// keeps agreeing without repeating it. Default: main.
func RootPackage(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "main"
	}
	fset := token.NewFileSet()
	generated := ""
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(root, name), nil, parser.PackageClauseOnly|parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		if name == GeneratedFileName {
			generated = file.Name.Name
			continue
		}
		return file.Name.Name
	}
	if generated != "" {
		return generated
	}
	return "main"
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
