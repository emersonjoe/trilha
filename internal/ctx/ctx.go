// Package ctx answers "what does this project already have?" in one read: the
// routes in the order the CLI lists them, the contract of every API handler,
// the types that contract names, and what setup.go registers. It is what an
// agent would otherwise spend a dozen file reads to learn, and it is the model
// behind trilha ctx (spec 047).
//
// Nothing here reads more of the project than the scanner and the openapi
// generator already read, and nothing in the output depends on the clock, the
// working directory or the order of a map: same project, same bytes.
package ctx

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/scan"
)

// Context is the whole map of a project.
type Context struct {
	Trilha    string    `json:"trilha"`
	Module    string    `json:"module"`
	Generated Generated `json:"generated"`
	Routes    []Route   `json:"routes"`
	Types     []Type    `json:"types,omitempty"`
	Setup     *Setup    `json:"setup,omitempty"`
}

// Generated says whether trilha_gen.go matches what app/ asks for. A route
// answering 404 is almost always this.
type Generated struct {
	File   string `json:"file"`
	Status string `json:"status"` // up to date | stale | missing
}

// Route is one URL: what serves it, what runs around it, and what it answers.
type Route struct {
	Pattern     string   `json:"pattern"`
	Kind        string   `json:"kind"` // page | api
	File        string   `json:"file"`
	Methods     []string `json:"methods"`
	Params      []string `json:"params,omitempty"`
	Layouts     []string `json:"layouts,omitempty"`
	Middlewares []string `json:"middlewares,omitempty"`
	// MiddlewaresByMethod holds the chains that guard a single method; the
	// compact output elides them and --all prints them.
	MiddlewaresByMethod map[string][]string `json:"middlewaresByMethod,omitempty"`
	API                 []Operation         `json:"api,omitempty"`
}

// Operation is one method of an API route, as the openapi inference reads it.
type Operation struct {
	Method    string     `json:"method"`
	Summary   string     `json:"summary,omitempty"`
	Query     []string   `json:"query,omitempty"`
	Request   string     `json:"request,omitempty"`
	Responses []Response `json:"responses,omitempty"`
}

// Response is one status the handler writes.
type Response struct {
	Status int    `json:"status"`
	Type   string `json:"type,omitempty"`
	Media  string `json:"media,omitempty"` // only when it is not application/json
}

// Type is a project type the contract names.
type Type struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields,omitempty"`
}

// Field is one field of it, with the rules the validate tag declares.
type Field struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required,omitempty"`
	Rules    string `json:"rules,omitempty"`
}

// Setup is what app/setup.go declares.
type Setup struct {
	File   string   `json:"file"`
	Funcs  []string `json:"funcs"`
	Values []Value  `json:"values,omitempty"`
}

// Value is one thing registered with trilha.Provide, read from the source.
// Type is empty when it takes a compiler to know it; From always says where
// it came from.
type Value struct {
	Type string `json:"type,omitempty"`
	From string `json:"from"`
}

// Build reads the project at root and returns its map. version is the CLI's
// own version, which the package has no way to know.
func Build(root, module, version string) (*Context, error) {
	res, err := scan.Scan(root, module)
	if err != nil {
		return nil, err
	}
	c := &Context{
		Trilha:    version,
		Module:    module,
		Generated: generated(root, res),
		Routes:    make([]Route, 0, len(res.Routes)),
	}
	doc, err := document(root, res)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, r := range res.Routes {
		c.Routes = append(c.Routes, route(r, doc, used))
	}
	c.Types = types(doc, used)
	c.Setup = setup(root, res)
	return c, nil
}

// JSON is the same map, indented, with a trailing newline.
func (c *Context) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// generated compares trilha_gen.go with what app/ asks for now.
func generated(root string, res *scan.Result) Generated {
	g := Generated{File: gen.FileName, Status: "missing"}
	cur, err := os.ReadFile(filepath.Join(root, gen.FileName))
	if err != nil {
		return g
	}
	src, err := gen.Generate(res)
	if err != nil {
		return g
	}
	g.Status = "stale"
	if bytes.Equal(cur, src) {
		g.Status = "up to date"
	}
	return g
}

func route(r scan.Route, doc *doc, used map[string]bool) Route {
	out := Route{
		Pattern: r.Pattern,
		Kind:    r.Kind,
		File:    r.Dir + "/" + routeFile(r),
		Methods: methods(r),
		Params:  params(r.Pattern),
	}
	for _, l := range r.Layouts {
		out.Layouts = append(out.Layouts, refFile(l, "layout.go"))
	}
	for _, m := range r.Middlewares {
		out.Middlewares = append(out.Middlewares, refFile(m, "middleware.go"))
	}
	for m, chain := range r.MiddlewaresByMethod {
		if out.MiddlewaresByMethod == nil {
			out.MiddlewaresByMethod = map[string][]string{}
		}
		for _, ref := range chain {
			out.MiddlewaresByMethod[m] = append(out.MiddlewaresByMethod[m], refFile(ref, "middleware.go"))
		}
	}
	if r.Kind == "api" {
		out.API = operations(r, doc, used)
	}
	return out
}

func routeFile(r scan.Route) string {
	if r.Kind == "page" {
		return "page.go"
	}
	return "route.go"
}

// refFile turns an import path back into the file it came from: the map is
// about files, because a file is what the agent opens.
func refFile(r scan.Ref, file string) string {
	p := r.ImportPath
	if i := strings.Index(p, "/app/"); i >= 0 {
		p = p[i+1:]
	} else if strings.HasSuffix(p, "/app") {
		p = "app"
	}
	return p + "/" + file
}

func methods(r scan.Route) []string {
	out := append([]string{}, r.Methods...)
	if r.HasPage {
		out = append([]string{"GET"}, out...)
	}
	return out
}

// params reads the parameter names out of the pattern, which is where the
// scanner already put them.
func params(pattern string) []string {
	var out []string
	for _, part := range strings.Split(pattern, "/") {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			out = append(out, strings.TrimSuffix(strings.Trim(part, "{}"), "..."))
		}
	}
	return out
}

// setup reads app/setup.go: which of the three functions it declares, and
// what it registers with trilha.Provide.
func setup(root string, res *scan.Result) *Setup {
	if res.Setup == nil && res.ConfigFunc == nil && res.ShutdownFunc == nil {
		return nil
	}
	s := &Setup{File: res.AppDir + "/setup.go"}
	if res.Setup != nil {
		s.Funcs = append(s.Funcs, "Setup")
	}
	if res.ConfigFunc != nil {
		s.Funcs = append(s.Funcs, "Config")
	}
	if res.ShutdownFunc != nil {
		s.Funcs = append(s.Funcs, "Shutdown")
	}
	s.Values = provided(filepath.Join(root, filepath.FromSlash(s.File)))
	return s
}

// provided finds the trilha.Provide calls. The type comes from an explicit
// type argument or from a composite literal; anything else would take a
// compiler, so the expression itself is what gets recorded.
func provided(file string) []Value {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	var out []Value
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		fun := call.Fun
		var typ string
		if ix, ok := fun.(*ast.IndexExpr); ok {
			fun, typ = ix.X, expr(fset, ix.Index)
		}
		sel, ok := fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Provide" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "trilha" {
			return true
		}
		arg := call.Args[1]
		if typ == "" {
			typ = literalType(fset, arg)
		}
		out = append(out, Value{Type: typ, From: expr(fset, arg)})
		return true
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].From < out[j].From
	})
	return out
}

// literalType reads the type off a composite literal, the one form that says
// it outright.
func literalType(fset *token.FileSet, e ast.Expr) string {
	switch v := e.(type) {
	case *ast.UnaryExpr:
		if v.Op == token.AND {
			if t := literalType(fset, v.X); t != "" {
				return "*" + t
			}
		}
	case *ast.CompositeLit:
		if v.Type != nil {
			return expr(fset, v.Type)
		}
	}
	return ""
}

// expr prints a node the way it was written, shortened when it is long: the
// map is a summary, not a copy of the file.
func expr(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	if err := printer.Fprint(&sb, fset, e); err != nil {
		return ""
	}
	s := strings.Join(strings.Fields(sb.String()), " ")
	if len(s) > 60 {
		s = s[:59] + "…"
	}
	return s
}
