package openapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha/internal/scan"
)

// schema is the piece of JSON Schema a Go type turns into. Field order here is
// the order in the document, and every map is written sorted by encoding/json:
// the same app always produces the same bytes.
type schema struct {
	Ref                  string             `json:"$ref,omitempty"`
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	MinItems             *int               `json:"minItems,omitempty"`
	MaxItems             *int               `json:"maxItems,omitempty"`
	Items                *schema            `json:"items,omitempty"`
	Properties           map[string]*schema `json:"properties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	AdditionalProperties *schema            `json:"additionalProperties,omitempty"`
}

// fileScope is where the identifiers of one file resolve: its own package and
// what it imports. A type name only means something inside one of these.
type fileScope struct {
	pkgPath string
	imports map[string]string // alias or last segment -> import path
	paths   []string          // every import path of the file, in source order
}

// typeRef is a type expression plus the scope its identifiers belong to. The
// expression may come from another package than the one being read, which is
// exactly why the scope travels with it.
type typeRef struct {
	expr  ast.Expr
	scope *fileScope
	name  string // the declared name, when this came from a type declaration
}

// funcSig is what a call to a function or a method gives back: the result
// types, and the type parameter names when the function is generic — the names
// the results are written in terms of.
type funcSig struct {
	results []typeRef
	tparams []string
}

type pkgIndex struct {
	name    string
	types   map[string]typeRef
	funcs   map[string]funcSig // exported function -> what it returns
	methods map[string]funcSig // "Type.Method" -> what it returns
}

// index is every package of the project, by import path. It is built by
// parsing the sources: no compiler, no module download, no dependency.
type index struct {
	module string
	pkgs   map[string]*pkgIndex
}

func newIndex(root, module string) (*index, error) {
	ix := &index{module: module, pkgs: map[string]*pkgIndex{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p == root {
				return nil
			}
			if name != scan.WellKnown && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" || name == "node_modules") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		ix.add(root, p)
		return nil
	})
	return ix, err
}

// add reads one file into the index. A file that does not parse is skipped:
// the compiler is the one who complains about it, not this command.
func (ix *index) add(root, file string) {
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return
	}
	rel, err := filepath.Rel(root, filepath.Dir(file))
	if err != nil {
		return
	}
	imp := ix.module
	if rel != "." {
		imp = path.Join(ix.module, filepath.ToSlash(rel))
	}
	pi := ix.pkgs[imp]
	if pi == nil {
		pi = &pkgIndex{types: map[string]typeRef{}, funcs: map[string]funcSig{}, methods: map[string]funcSig{}}
		ix.pkgs[imp] = pi
	}
	pi.name = f.Name.Name
	sc := scopeOf(f, imp)
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, sp := range d.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				pi.types[ts.Name.Name] = typeRef{expr: ts.Type, scope: sc, name: ts.Name.Name}
			}
		case *ast.FuncDecl:
			if !d.Name.IsExported() || d.Type.Results == nil {
				continue
			}
			sig := funcSig{results: resultTypes(d.Type.Results, sc), tparams: paramNames(d.Type.TypeParams)}
			if d.Recv == nil {
				pi.funcs[d.Name.Name] = sig
				continue
			}
			if recv := recvName(d.Recv); recv != "" {
				pi.methods[recv+"."+d.Name.Name] = sig
			}
		}
	}
}

// resultTypes reads a result list: one entry per value the call gives back,
// which is what a multiple assignment lines up against.
func resultTypes(results *ast.FieldList, sc *fileScope) []typeRef {
	var res []typeRef
	for _, r := range results.List {
		n := len(r.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			res = append(res, typeRef{expr: r.Type, scope: sc})
		}
	}
	return res
}

// paramNames is the type parameter names of a generic declaration, in the
// order the type arguments of a call fill them.
func paramNames(tp *ast.FieldList) []string {
	if tp == nil {
		return nil
	}
	var names []string
	for _, f := range tp.List {
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

// recvName is the type a method hangs on, with the pointer and the type
// parameters of a generic receiver taken off: func (s *Store[K]) All() is a
// method of Store.
func recvName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	e := recv.List[0].Type
	for {
		switch v := e.(type) {
		case *ast.StarExpr:
			e = v.X
		case *ast.ParenExpr:
			e = v.X
		case *ast.IndexExpr:
			e = v.X
		case *ast.IndexListExpr:
			e = v.X
		case *ast.Ident:
			return v.Name
		default:
			return ""
		}
	}
}

func scopeOf(f *ast.File, pkgPath string) *fileScope {
	sc := &fileScope{pkgPath: pkgPath, imports: map[string]string{}}
	for _, im := range f.Imports {
		p, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			continue
		}
		sc.paths = append(sc.paths, p)
		alias := path.Base(p)
		if im.Name != nil {
			alias = im.Name.Name
		}
		sc.imports[alias] = p
	}
	return sc
}

// resolveType finds "pkg.Type" (or "Type", in the file's own package) starting
// from one file. The alias comes first; when there is none, the package that
// declares that name wins — a directory can be called "relatorio.csv" and the
// package inside it "relatoriocsv".
func (ix *index) resolveType(sc *fileScope, name string) (typeRef, bool) {
	qual, sel, ok := strings.Cut(name, ".")
	if !ok {
		return ix.typeIn(sc.pkgPath, qual)
	}
	if p, ok := sc.imports[qual]; ok {
		if tr, ok := ix.typeIn(p, sel); ok {
			return tr, true
		}
	}
	for _, p := range sc.paths {
		if pi := ix.pkgs[p]; pi != nil && pi.name == qual {
			return ix.typeIn(p, sel)
		}
	}
	return typeRef{}, false
}

func (ix *index) typeIn(pkgPath, name string) (typeRef, bool) {
	pi := ix.pkgs[pkgPath]
	if pi == nil {
		return typeRef{}, false
	}
	tr, ok := pi.types[name]
	return tr, ok
}

// sigOf answers what a call to pkg.Fn returns, which is how a local variable
// gets a type without a type checker.
func (ix *index) sigOf(sc *fileScope, qual, name string) (funcSig, bool) {
	if qual == "" {
		pi := ix.pkgs[sc.pkgPath]
		if pi == nil {
			return funcSig{}, false
		}
		r, ok := pi.funcs[name]
		return r, ok
	}
	if p, ok := sc.imports[qual]; ok {
		if pi := ix.pkgs[p]; pi != nil {
			if r, ok := pi.funcs[name]; ok {
				return r, true
			}
		}
	}
	for _, p := range sc.paths {
		if pi := ix.pkgs[p]; pi != nil && pi.name == qual {
			r, ok := pi.funcs[name]
			return r, ok
		}
	}
	return funcSig{}, false
}

// named walks a type expression down to the declaration it points at, so a
// value of type *posts.Store finds the Store that package declares.
func (ix *index) named(t typeRef) (typeRef, bool) {
	if t.expr == nil || t.scope == nil {
		return typeRef{}, false
	}
	switch e := t.expr.(type) {
	case *ast.ParenExpr:
		return ix.named(typeRef{expr: e.X, scope: t.scope})
	case *ast.StarExpr:
		return ix.named(typeRef{expr: e.X, scope: t.scope})
	case *ast.Ident:
		return ix.resolveType(t.scope, e.Name)
	case *ast.SelectorExpr:
		if q, ok := e.X.(*ast.Ident); ok {
			return ix.resolveType(t.scope, q.Name+"."+e.Sel.Name)
		}
	}
	return typeRef{}, false
}

// methodSig answers what v.M() returns, when v holds a type this project
// declares. It is how a handler that reaches its store through a dependency
// still says what it answers.
func (ix *index) methodSig(t typeRef, name string) (funcSig, bool) {
	tr, ok := ix.named(t)
	if !ok || tr.name == "" || tr.scope == nil {
		return funcSig{}, false
	}
	pi := ix.pkgs[tr.scope.pkgPath]
	if pi == nil {
		return funcSig{}, false
	}
	r, ok := pi.methods[tr.name+"."+name]
	return r, ok
}

func (ix *index) pkgName(pkgPath string) string {
	if pi := ix.pkgs[pkgPath]; pi != nil {
		return pi.name
	}
	return path.Base(pkgPath)
}

// schemaFor turns a Go type into JSON Schema. A type it cannot read gives nil,
// and the caller writes the response without a schema: a wrong schema is worse
// than an absent one, because a client believes it.
func (g *generator) schemaFor(t typeRef) *schema {
	if t.expr == nil || t.scope == nil {
		return nil
	}
	switch e := t.expr.(type) {
	case *ast.ParenExpr:
		return g.schemaFor(typeRef{expr: e.X, scope: t.scope})
	case *ast.StarExpr:
		return g.schemaFor(typeRef{expr: e.X, scope: t.scope})
	case *ast.Ident:
		if s := basicSchema(e.Name); s != nil {
			return s
		}
		if tr, ok := g.ix.resolveType(t.scope, e.Name); ok {
			return g.declared(tr)
		}
		return nil
	case *ast.SelectorExpr:
		qual, ok := e.X.(*ast.Ident)
		if !ok {
			return nil
		}
		if qual.Name == "time" {
			switch e.Sel.Name {
			case "Time":
				return &schema{Type: "string", Format: "date-time"}
			case "Duration":
				return &schema{Type: "integer"}
			}
		}
		if tr, ok := g.ix.resolveType(t.scope, qual.Name+"."+e.Sel.Name); ok {
			return g.declared(tr)
		}
		return nil
	case *ast.ArrayType:
		if id, ok := e.Elt.(*ast.Ident); ok && id.Name == "byte" {
			return &schema{Type: "string", Format: "byte"}
		}
		return &schema{Type: "array", Items: g.schemaFor(typeRef{expr: e.Elt, scope: t.scope})}
	case *ast.MapType:
		return &schema{Type: "object", AdditionalProperties: g.schemaFor(typeRef{expr: e.Value, scope: t.scope})}
	case *ast.StructType:
		return g.structSchema(e, t.scope)
	case *ast.InterfaceType:
		return &schema{}
	}
	return nil
}

// declared handles a named type: a struct becomes a component and travels as a
// $ref, anything else (type Slug string) is inlined — there is nothing to
// reuse in it.
func (g *generator) declared(tr typeRef) *schema {
	st, ok := tr.expr.(*ast.StructType)
	if !ok {
		return g.schemaFor(typeRef{expr: tr.expr, scope: tr.scope})
	}
	key := g.key(tr)
	if _, done := g.schemas[key]; !done && !g.busy[key] {
		g.busy[key] = true
		s := g.structSchema(st, tr.scope)
		delete(g.busy, key)
		g.schemas[key] = s
	}
	return &schema{Ref: "#/components/schemas/" + key}
}

// key names a component "package.Type", which keeps the origin visible. Two
// packages with the same name (app/api/posts and internal/posts) get the
// parent directory in front of the second one.
func (g *generator) key(tr typeRef) string {
	full := tr.scope.pkgPath + "." + tr.name
	if k, ok := g.keys[full]; ok {
		return k
	}
	parts := strings.Split(tr.scope.pkgPath, "/")
	base := g.ix.pkgName(tr.scope.pkgPath) + "." + tr.name
	k := base
	for i := len(parts) - 1; g.taken[k] && i > 0; i-- {
		k = parts[i-1] + "." + k
	}
	g.keys[full] = k
	g.taken[k] = true
	return k
}

func (g *generator) structSchema(st *ast.StructType, sc *fileScope) *schema {
	s := &schema{Type: "object"}
	props := map[string]*schema{}
	g.fields(st, sc, props, s)
	if len(props) > 0 {
		s.Properties = props
	}
	sort.Strings(s.Required)
	return s
}

func (g *generator) fields(st *ast.StructType, sc *fileScope, props map[string]*schema, s *schema) {
	for _, f := range st.Fields.List {
		tag := reflect.StructTag(strings.Trim(tagText(f), "`"))
		name, _, _ := strings.Cut(tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if len(f.Names) == 0 {
			// Embedded field: encoding/json flattens it when there is no tag.
			if name == "" {
				if inner, ok := g.embedded(f.Type, sc); ok {
					g.fields(inner.st, inner.scope, props, s)
					continue
				}
			}
			if name == "" {
				name = embeddedName(f.Type)
			}
			if name == "" {
				continue
			}
			g.property(name, f, tag, sc, props, s)
			continue
		}
		for i, n := range f.Names {
			if !n.IsExported() {
				continue
			}
			key := name
			if key == "" || i > 0 {
				key = n.Name
			}
			g.property(key, f, tag, sc, props, s)
		}
	}
}

func (g *generator) property(name string, f *ast.Field, tag reflect.StructTag, sc *fileScope, props map[string]*schema, s *schema) {
	fs := g.schemaFor(typeRef{expr: f.Type, scope: sc})
	if fs == nil {
		fs = &schema{}
	}
	if d := docLine(f.Doc, f.Comment); d != "" {
		fs.Description = d
	}
	if applyValidate(fs, tag.Get("validate")) {
		s.Required = append(s.Required, name)
	}
	props[name] = fs
}

type embeddedStruct struct {
	st    *ast.StructType
	scope *fileScope
}

func (g *generator) embedded(e ast.Expr, sc *fileScope) (embeddedStruct, bool) {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	var name string
	switch t := e.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.SelectorExpr:
		if q, ok := t.X.(*ast.Ident); ok {
			name = q.Name + "." + t.Sel.Name
		}
	}
	if name == "" {
		return embeddedStruct{}, false
	}
	tr, ok := g.ix.resolveType(sc, name)
	if !ok {
		return embeddedStruct{}, false
	}
	st, ok := tr.expr.(*ast.StructType)
	if !ok {
		return embeddedStruct{}, false
	}
	return embeddedStruct{st: st, scope: tr.scope}, true
}

func embeddedName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return embeddedName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// applyValidate reads the same tag the runtime reads (spec 027), so the schema
// cannot drift from the validation: it is the same string. It reports whether
// the field is required.
func applyValidate(s *schema, tag string) bool {
	required := false
	for _, part := range strings.Split(tag, ",") {
		name, param, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch name {
		case "required":
			required = true
		case "email":
			s.Format = "email"
		case "url":
			s.Format = "uri"
		case "oneof":
			for _, opt := range strings.Fields(param) {
				if s.Type == "integer" || s.Type == "number" {
					if n, err := strconv.ParseFloat(opt, 64); err == nil {
						s.Enum = append(s.Enum, n)
						continue
					}
				}
				s.Enum = append(s.Enum, opt)
			}
		case "min", "max", "len":
			n, err := strconv.ParseFloat(param, 64)
			if err != nil {
				continue
			}
			i := int(n)
			switch s.Type {
			case "string":
				if name != "max" {
					s.MinLength = &i
				}
				if name != "min" {
					s.MaxLength = &i
				}
			case "array":
				if name != "max" {
					s.MinItems = &i
				}
				if name != "min" {
					s.MaxItems = &i
				}
			case "integer", "number":
				if name != "max" {
					s.Minimum = &n
				}
				if name != "min" {
					s.Maximum = &n
				}
			}
		}
	}
	return required
}

func basicSchema(name string) *schema {
	switch name {
	case "string":
		return &schema{Type: "string"}
	case "bool":
		return &schema{Type: "boolean"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "rune", "byte":
		return &schema{Type: "integer"}
	case "float32", "float64":
		return &schema{Type: "number"}
	case "any", "error":
		return &schema{}
	}
	return nil
}

func tagText(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}
	s, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return f.Tag.Value
	}
	return s
}
