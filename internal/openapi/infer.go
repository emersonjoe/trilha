package openapi

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// statusByName maps http.StatusCreated to 201 without a table to maintain: the
// name is the text with the spaces taken out, except for the two the standard
// library spells differently.
var statusByName = func() map[string]int {
	m := map[string]int{}
	for code := 100; code < 600; code++ {
		t := http.StatusText(code)
		if t == "" {
			continue
		}
		var sb strings.Builder
		for _, r := range t {
			if r != ' ' && r != '-' {
				sb.WriteRune(r)
			}
		}
		m["Status"+sb.String()] = code
	}
	m["StatusTeapot"] = http.StatusTeapot
	m["StatusNonAuthoritativeInfo"] = http.StatusNonAuthoritativeInfo
	return m
}()

// handler is what one exported method function says about itself.
type handler struct {
	summary     string
	description string
	tag         string
	queries     []*parameter
	body        *schema
	bodyMedia   string
	media       string          // media type of the successful responses
	ok          map[int]*schema // status -> schema (nil: no body)
	fail        map[int]bool    // status answered as problem+json
}

func newHandler() *handler {
	return &handler{ok: map[int]*schema{}, fail: map[int]bool{}}
}

// routeFuncs parses the route directory and returns its exported functions
// with the scope their identifiers resolve in.
func routeFuncs(dir, pkgPath string) (map[string]*ast.FuncDecl, *fileScope, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	funcs := map[string]*ast.FuncDecl{}
	var sc *fileScope
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.SkipObjectResolution|parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
		fsc := scopeOf(f, pkgPath)
		if sc == nil {
			sc = fsc
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !fd.Name.IsExported() {
				continue
			}
			funcs[fd.Name.Name] = fd
			// Each function reads its own file's imports; the merged scope is
			// enough because a route directory is one package.
			for alias, p := range fsc.imports {
				sc.imports[alias] = p
			}
			sc.paths = append(sc.paths, fsc.paths...)
		}
	}
	if sc == nil {
		sc = &fileScope{pkgPath: pkgPath, imports: map[string]string{}}
	}
	return funcs, sc, nil
}

// read walks the handler body and its doc comment. The body says what the code
// actually answers; the comment says what the code cannot show.
func (g *generator) read(fn *ast.FuncDecl, sc *fileScope, file string) (*handler, error) {
	h := newHandler()
	h.summary, h.description = docText(fn)
	if err := g.directives(h, fn, sc, file); err != nil {
		return nil, err
	}
	locals := map[string]typeRef{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.DeclStmt:
			gd, ok := n.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok || vs.Type == nil {
					continue
				}
				for _, id := range vs.Names {
					locals[id.Name] = typeRef{expr: vs.Type, scope: sc}
				}
			}
		case *ast.AssignStmt:
			g.assign(n, sc, locals)
		case *ast.CallExpr:
			g.call(h, n, sc, locals)
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok && x.Name == "trilha" && n.Sel.Name == "ErrNotFound" {
				h.fail[http.StatusNotFound] = true
			}
		case *ast.CompositeLit:
			if sel, ok := n.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Problem" {
				for _, el := range n.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if k, ok := kv.Key.(*ast.Ident); !ok || k.Name != "Status" {
						continue
					}
					if code, ok := statusOf(kv.Value); ok {
						h.fail[code] = true
					}
				}
			}
		}
		return true
	})
	return h, nil
}

// assign gives a local variable a type, in the three shapes that carry one:
// v := pkg.Fn(), v, ok := pkg.Fn() and v := pkg.Type{}.
func (g *generator) assign(n *ast.AssignStmt, sc *fileScope, locals map[string]typeRef) {
	if len(n.Rhs) == 1 {
		if call, ok := n.Rhs[0].(*ast.CallExpr); ok {
			res, ok := g.callResults(call, sc)
			if !ok {
				return
			}
			for i, lhs := range n.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(res) && id.Name != "_" {
					locals[id.Name] = res[i]
				}
			}
			return
		}
	}
	if len(n.Lhs) != len(n.Rhs) {
		return
	}
	for i, lhs := range n.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		if t, ok := literalType(n.Rhs[i], sc); ok {
			locals[id.Name] = t
		}
	}
}

func literalType(e ast.Expr, sc *fileScope) (typeRef, bool) {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	if cl, ok := e.(*ast.CompositeLit); ok && cl.Type != nil {
		return typeRef{expr: cl.Type, scope: sc}, true
	}
	return typeRef{}, false
}

func (g *generator) callResults(call *ast.CallExpr, sc *fileScope) ([]typeRef, bool) {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return g.ix.resultsOf(sc, "", fn.Name)
	case *ast.SelectorExpr:
		if q, ok := fn.X.(*ast.Ident); ok {
			return g.ix.resultsOf(sc, q.Name, fn.Sel.Name)
		}
	}
	return nil, false
}

// call reads the four things a handler does that the document cares about:
// binding a body, writing JSON, writing a bare status and setting the type.
func (g *generator) call(h *handler, call *ast.CallExpr, sc *fileScope, locals map[string]typeRef) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	switch sel.Sel.Name {
	case "Bind", "BindJSON":
		if len(call.Args) != 1 || h.body != nil {
			return
		}
		if t, ok := boundType(call.Args[0], locals); ok {
			h.body = g.schemaFor(t)
			h.fail[http.StatusUnprocessableEntity] = true
		}
	case "JSON":
		if len(call.Args) != 2 {
			return
		}
		code, ok := statusOf(call.Args[0])
		if !ok {
			return
		}
		h.ok[code] = g.valueSchema(call.Args[1], sc, locals)
	case "WriteHeader":
		if len(call.Args) != 1 {
			return
		}
		if code, ok := statusOf(call.Args[0]); ok {
			if _, seen := h.ok[code]; !seen {
				h.ok[code] = nil
			}
		}
	case "Header":
		if len(call.Args) != 2 || h.media != "" {
			return
		}
		if k, ok := stringOf(call.Args[0]); !ok || !strings.EqualFold(k, "Content-Type") {
			return
		}
		if v, ok := stringOf(call.Args[1]); ok {
			h.media = strings.TrimSpace(strings.SplitN(v, ";", 2)[0])
		}
	case "Errorf":
		q, ok := sel.X.(*ast.Ident)
		if !ok || q.Name != "trilha" || len(call.Args) == 0 {
			return
		}
		if code, ok := statusOf(call.Args[0]); ok {
			h.fail[code] = true
		}
	}
}

// valueSchema answers the type of what went into c.JSON: a local variable, a
// call, or a literal. Anything else has no schema, and the response says so by
// leaving it out.
func (g *generator) valueSchema(e ast.Expr, sc *fileScope, locals map[string]typeRef) *schema {
	switch v := e.(type) {
	case *ast.Ident:
		if t, ok := locals[v.Name]; ok {
			return g.schemaFor(t)
		}
	case *ast.CallExpr:
		if res, ok := g.callResults(v, sc); ok && len(res) > 0 {
			return g.schemaFor(res[0])
		}
	case *ast.UnaryExpr, *ast.CompositeLit:
		if t, ok := literalType(e, sc); ok {
			return g.schemaFor(t)
		}
	}
	return nil
}

func boundType(arg ast.Expr, locals map[string]typeRef) (typeRef, bool) {
	u, ok := arg.(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return typeRef{}, false
	}
	id, ok := u.X.(*ast.Ident)
	if !ok {
		return typeRef{}, false
	}
	t, ok := locals[id.Name]
	return t, ok
}

func statusOf(e ast.Expr) (int, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.INT {
			n, err := strconv.Atoi(v.Value)
			return n, err == nil && n >= 100 && n < 600
		}
	case *ast.SelectorExpr:
		if q, ok := v.X.(*ast.Ident); ok && q.Name == "http" {
			c, ok := statusByName[v.Sel.Name]
			return c, ok
		}
	}
	return 0, false
}

func stringOf(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// directives reads the openapi: lines of the doc comment — what the code
// cannot say by itself.
func (g *generator) directives(h *handler, fn *ast.FuncDecl, sc *fileScope, file string) error {
	if fn.Doc == nil {
		return nil
	}
	for _, c := range fn.Doc.List {
		line := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		if !strings.HasPrefix(line, "openapi:") {
			continue
		}
		verb, rest, _ := strings.Cut(strings.TrimPrefix(line, "openapi:"), " ")
		fields := strings.Fields(rest)
		where := fmt.Sprintf("%s: %s: openapi:%s", file, fn.Name.Name, verb)
		switch verb {
		case "tag":
			if len(fields) == 0 {
				return fmt.Errorf("%s: needs a name", where)
			}
			h.tag = fields[0]
		case "response":
			if len(fields) == 0 {
				return fmt.Errorf("%s: needs a status", where)
			}
			code, err := strconv.Atoi(fields[0])
			if err != nil || code < 100 || code >= 600 {
				return fmt.Errorf("%s: %q is not a status", where, fields[0])
			}
			var s *schema
			if len(fields) > 1 {
				if s, err = g.namedSchema(sc, fields[1], where); err != nil {
					return err
				}
			}
			if code >= 400 && s == nil {
				h.fail[code] = true
				continue
			}
			delete(h.fail, code)
			h.ok[code] = s
		case "body":
			if len(fields) == 0 {
				return fmt.Errorf("%s: needs a type", where)
			}
			s, err := g.namedSchema(sc, fields[0], where)
			if err != nil {
				return err
			}
			h.body = s
		case "query":
			if len(fields) < 2 {
				return fmt.Errorf("%s: needs a name and a type", where)
			}
			s := basicSchema(fields[1])
			if s == nil {
				return fmt.Errorf("%s: %q is not a basic type", where, fields[1])
			}
			h.queries = append(h.queries, &parameter{
				Name:        fields[0],
				In:          "query",
				Description: strings.Join(fields[2:], " "),
				Schema:      s,
			})
		default:
			return fmt.Errorf("%s: unknown directive", where)
		}
	}
	return nil
}

// namedSchema resolves "posts.Post" or "[]posts.Post" written by hand. A name
// nobody can find is an error: an empty schema here would publish a contract
// that looks right and is not.
func (g *generator) namedSchema(sc *fileScope, name, where string) (*schema, error) {
	list := strings.HasPrefix(name, "[]")
	name = strings.TrimPrefix(name, "[]")
	s := basicSchema(name)
	if s == nil {
		tr, ok := g.ix.resolveType(sc, name)
		if !ok {
			return nil, fmt.Errorf("%s: unknown type %q", where, name)
		}
		s = g.declared(tr)
	}
	if list {
		return &schema{Type: "array", Items: s}, nil
	}
	return s, nil
}

// docText splits the doc comment into the first sentence and the rest, with
// the openapi: lines taken out.
func docText(fn *ast.FuncDecl) (string, string) {
	if fn.Doc == nil {
		return "", ""
	}
	var lines []string
	for _, c := range fn.Doc.List {
		l := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(c.Text, "//"), "/*"))
		l = strings.TrimSuffix(l, "*/")
		if strings.HasPrefix(l, "openapi:") {
			continue
		}
		lines = append(lines, strings.TrimSpace(l))
	}
	text := strings.TrimSpace(strings.Join(lines, "\n"))
	if text == "" {
		return "", ""
	}
	first, rest := text, ""
	if i := strings.Index(text, ". "); i >= 0 {
		first, rest = text[:i+1], strings.TrimSpace(text[i+1:])
	} else if i := strings.Index(text, ".\n"); i >= 0 {
		first, rest = text[:i+1], strings.TrimSpace(text[i+1:])
	}
	return strings.TrimSpace(strings.ReplaceAll(first, "\n", " ")), rest
}

// docLine is the one-line description of a struct field.
func docLine(groups ...*ast.CommentGroup) string {
	for _, g := range groups {
		if g == nil {
			continue
		}
		if t := strings.TrimSpace(g.Text()); t != "" {
			return strings.TrimSpace(strings.SplitN(t, "\n", 2)[0])
		}
	}
	return ""
}
