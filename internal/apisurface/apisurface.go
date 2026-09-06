// Package apisurface renders the exported surface of a set of packages as
// sorted text lines, one per symbol.
//
// The format follows Go's own api/go1.txt: a line names the package and the
// declaration, without parameter names and without documentation, so the diff
// of two renderings is exactly the set of symbols that came and went. The
// reading is syntactic — it sees what would break a compilation, not what
// would break a behaviour.
package apisurface

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Package is one package to render: Dir is relative to the root, Name is how
// the package appears at the start of each line.
type Package struct {
	Dir  string
	Name string
}

// Render returns the surface of every package, in the order given, with a
// trailing newline. Same source, same bytes.
func Render(root string, pkgs []Package) (string, error) {
	var out []string
	for _, p := range pkgs {
		lines, err := renderPackage(filepath.Join(root, filepath.FromSlash(p.Dir)), p.Name)
		if err != nil {
			return "", fmt.Errorf("%s: %w", p.Dir, err)
		}
		out = append(out, lines...)
	}
	return strings.Join(out, "\n") + "\n", nil
}

type collector struct {
	fset  *token.FileSet
	pkg   string
	lines []string
}

func renderPackage(dir, name string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	c := &collector{fset: token.NewFileSet(), pkg: name}
	var files []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		files = append(files, n)
	}
	sort.Strings(files)
	for _, n := range files {
		f, err := parser.ParseFile(c.fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		c.file(f)
	}
	sort.Strings(c.lines)
	return c.lines, nil
}

func (c *collector) file(f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			c.funcDecl(d)
		case *ast.GenDecl:
			c.genDecl(d)
		}
	}
}

func (c *collector) add(dep bool, format string, args ...any) {
	line := "pkg " + c.pkg + ", " + fmt.Sprintf(format, args...)
	if dep {
		line += " // deprecated"
	}
	c.lines = append(c.lines, line)
}

func (c *collector) funcDecl(d *ast.FuncDecl) {
	if !d.Name.IsExported() {
		return
	}
	dep := deprecated(d.Doc)
	if d.Recv == nil {
		c.add(dep, "func %s%s%s", d.Name.Name, c.typeParams(d.Type.TypeParams), c.sig(d.Type))
		return
	}
	recv, base := c.receiver(d.Recv)
	if base == "" || !ast.IsExported(base) {
		return
	}
	c.add(dep, "method %s %s%s", recv, d.Name.Name, c.sig(d.Type))
}

// receiver returns the printed receiver and the bare type name, so a method on
// an unexported type stays out of the surface.
func (c *collector) receiver(fl *ast.FieldList) (string, string) {
	if fl == nil || len(fl.List) == 0 {
		return "", ""
	}
	t := fl.List[0].Type
	star := ""
	if s, ok := t.(*ast.StarExpr); ok {
		star, t = "*", s.X
	}
	// A generic receiver comes as Type[T]; the surface names the type.
	switch v := t.(type) {
	case *ast.IndexExpr:
		t = v.X
	case *ast.IndexListExpr:
		t = v.X
	}
	id, ok := t.(*ast.Ident)
	if !ok {
		return "", ""
	}
	return "(" + star + id.Name + ")", id.Name
}

func (c *collector) genDecl(d *ast.GenDecl) {
	switch d.Tok {
	case token.CONST, token.VAR:
		kind := "const"
		if d.Tok == token.VAR {
			kind = "var"
		}
		var inherited string // an iota block repeats the type of the first spec
		for _, s := range d.Specs {
			vs, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			typ := ""
			switch {
			case vs.Type != nil:
				typ = " " + c.expr(vs.Type)
				inherited = typ
			case d.Tok == token.CONST && len(vs.Values) == 0:
				typ = inherited
			default:
				inherited = ""
			}
			dep := deprecated(vs.Doc) || deprecated(d.Doc)
			for _, n := range vs.Names {
				if n.IsExported() {
					c.add(dep, "%s %s%s", kind, n.Name, typ)
				}
			}
		}
	case token.TYPE:
		for _, s := range d.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				continue
			}
			c.typeSpec(ts, deprecated(ts.Doc) || deprecated(d.Doc))
		}
	}
}

func (c *collector) typeSpec(ts *ast.TypeSpec, dep bool) {
	head := ts.Name.Name + c.typeParams(ts.TypeParams)
	if ts.Assign.IsValid() {
		c.add(dep, "type %s = %s", head, c.expr(ts.Type))
		return
	}
	switch t := ts.Type.(type) {
	case *ast.StructType:
		c.add(dep, "type %s struct", head)
		c.structFields(ts.Name.Name, t)
	case *ast.InterfaceType:
		c.add(dep, "type %s interface", head)
		c.interfaceMethods(ts.Name.Name, t)
	default:
		c.add(dep, "type %s %s", head, c.expr(ts.Type))
	}
}

func (c *collector) structFields(name string, t *ast.StructType) {
	if t.Fields == nil {
		return
	}
	for _, f := range t.Fields.List {
		dep := deprecated(f.Doc)
		if len(f.Names) == 0 {
			// An embedded field promotes its methods: it is surface.
			c.add(dep, "type %s struct, embedded %s", name, c.expr(f.Type))
			continue
		}
		for _, n := range f.Names {
			if n.IsExported() {
				c.add(dep, "type %s struct, %s %s", name, n.Name, c.expr(f.Type))
			}
		}
	}
}

func (c *collector) interfaceMethods(name string, t *ast.InterfaceType) {
	if t.Methods == nil {
		return
	}
	for _, f := range t.Methods.List {
		dep := deprecated(f.Doc)
		if len(f.Names) == 0 {
			c.add(dep, "type %s interface, embedded %s", name, c.expr(f.Type))
			continue
		}
		ft, ok := f.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		for _, n := range f.Names {
			if n.IsExported() {
				c.add(dep, "type %s interface, %s%s", name, n.Name, c.sig(ft))
			}
		}
	}
}

// sig prints a signature without parameter names: renaming a parameter is not
// a change to the surface, and noise in the golden is what makes nobody read
// its diff.
func (c *collector) sig(ft *ast.FuncType) string {
	s := "(" + c.fields(ft.Params) + ")"
	res := c.fields(ft.Results)
	switch {
	case res == "":
	case len(ft.Results.List) == 1 && len(ft.Results.List[0].Names) == 0:
		s += " " + res
	default:
		s += " (" + res + ")"
	}
	return s
}

func (c *collector) fields(fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		t := c.expr(f.Type)
		for i := 0; i < n; i++ {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ", ")
}

// typeParams keeps the names, unlike parameters: a constraint is part of what
// the caller has to satisfy.
func (c *collector) typeParams(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		var names []string
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
		parts = append(parts, strings.TrimSpace(strings.Join(names, ", ")+" "+c.expr(f.Type)))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (c *collector) expr(e ast.Expr) string {
	// A function type printed inside another type would carry its parameter
	// names; the rest of the surface drops them, and half-dropping is worse
	// than either.
	ast.Inspect(e, func(n ast.Node) bool {
		if ft, ok := n.(*ast.FuncType); ok {
			stripNames(ft.Params)
			stripNames(ft.Results)
		}
		return true
	})
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, c.fset, e); err != nil {
		return "?"
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// stripNames drops parameter names, expanding "a, b int" into two fields so
// the arity survives.
func stripNames(fl *ast.FieldList) {
	if fl == nil {
		return
	}
	out := make([]*ast.Field, 0, len(fl.List))
	for _, f := range fl.List {
		if len(f.Names) <= 1 {
			f.Names = nil
			out = append(out, f)
			continue
		}
		for range f.Names {
			out = append(out, &ast.Field{Type: f.Type})
		}
	}
	fl.List = out
}

func deprecated(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, l := range strings.Split(doc.Text(), "\n") {
		if strings.HasPrefix(l, "Deprecated:") {
			return true
		}
	}
	return false
}
