// Package islands reads the c.Island calls of an app and models the props each
// island module receives, so trilha gen can write the types an editor needs to
// check the JavaScript on the other side of the boundary.
//
// It is a reader, not a compiler: what it cannot resolve becomes unknown plus a
// note, and a note never stops the generator. An island whose props are a map
// literal or a variable still works at runtime; it just gets no type.
package islands

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Prop is one field of a props struct, named the way encoding/json names it.
type Prop struct {
	Name     string
	Type     string
	Optional bool
	Doc      string
}

// Island is one c.Island call site, keyed by the module it mounts.
type Island struct {
	Src  string // the module path, as written: "/editor.js"
	Type string // the TypeScript interface name, empty when the props are unknown
	From string // the Go type the props came from, for the doc comment
}

// Object is a TypeScript interface written from a Go struct.
type Object struct {
	Name  string
	Doc   string
	Props []Prop
}

// Result is what the app declares: which module gets which props, and what the
// reader could not answer.
type Result struct {
	Islands []Island
	Objects []Object
	Notes   []string
}

// Scan reads every .go file under root (the app directory) and returns the
// islands it mounts. A missing root is not an error: an app without islands has
// nothing to declare.
func Scan(root string) (*Result, error) {
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return &Result{}, nil
		}
		return nil, err
	}
	s := &scanner{
		fset:    token.NewFileSet(),
		structs: map[string]*structDecl{},
		byName:  map[string][]*structDecl{},
		named:   map[string]ast.Expr{},
		seen:    map[string]bool{},
		res:     &Result{},
	}
	files, err := goFiles(root)
	if err != nil {
		return nil, err
	}
	for _, path := range files {
		f, err := parser.ParseFile(s.fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		s.files = append(s.files, fileInfo{path: path, dir: filepath.Dir(path), file: f})
	}
	for _, fi := range s.files {
		s.index(fi)
	}
	for _, fi := range s.files {
		s.calls(fi)
	}
	sort.Slice(s.res.Islands, func(i, j int) bool { return s.res.Islands[i].Src < s.res.Islands[j].Src })
	sort.Strings(s.res.Notes)
	return s.res, nil
}

type fileInfo struct {
	path string
	dir  string
	file *ast.File
}

type structDecl struct {
	dir  string
	name string
	doc  string
	spec *ast.StructType
}

type scanner struct {
	fset    *token.FileSet
	files   []fileInfo
	structs map[string]*structDecl   // dir + "." + name
	byName  map[string][]*structDecl // name, for a type reached through another package
	named   map[string]ast.Expr      // dir + "." + name, for a defined non-struct type
	seen    map[string]bool          // island sources already recorded
	stack   []string                 // structs being written, to break a cycle
	res     *Result
}

// index records every struct and every defined type, so a props type declared
// next to the page can be found by name.
func (s *scanner) index(fi fileInfo) {
	for _, d := range fi.file.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, sp := range gd.Specs {
			ts, ok := sp.(*ast.TypeSpec)
			if !ok {
				continue
			}
			doc := docText(ts.Doc)
			if doc == "" {
				doc = docText(gd.Doc)
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				sd := &structDecl{dir: fi.dir, name: ts.Name.Name, doc: doc, spec: st}
				s.structs[fi.dir+"."+ts.Name.Name] = sd
				s.byName[ts.Name.Name] = append(s.byName[ts.Name.Name], sd)
				continue
			}
			s.named[fi.dir+"."+ts.Name.Name] = ts.Type
		}
	}
}

// calls walks a file for c.Island("/x.js", Props{...}).
func (s *scanner) calls(fi fileInfo) {
	ast.Inspect(fi.file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Island" || len(call.Args) < 2 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		src, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.HasPrefix(src, "/") {
			return true
		}
		if s.seen[src] {
			return true
		}
		s.seen[src] = true
		s.island(fi, src, call.Args[1])
		return true
	})
}

func (s *scanner) island(fi fileInfo, src string, props ast.Expr) {
	sd := s.propsType(props)
	if sd == nil {
		if !isNil(props) {
			s.note("%s: the props of %s are not a struct literal, so its type is unknown; declare a props struct to get one", s.where(fi, props), src)
		}
		s.res.Islands = append(s.res.Islands, Island{Src: src})
		return
	}
	name := s.object(sd)
	s.res.Islands = append(s.res.Islands, Island{Src: src, Type: name, From: sd.name})
}

// propsType finds the struct behind the second argument: Props{...}, &Props{...}
// or pkg.Props{...}. Anything else has no name to hang a type on.
func (s *scanner) propsType(e ast.Expr) *structDecl {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	cl, ok := e.(*ast.CompositeLit)
	if !ok || cl.Type == nil {
		return nil
	}
	switch t := cl.Type.(type) {
	case *ast.Ident:
		return s.lookup("", t.Name)
	case *ast.SelectorExpr:
		return s.lookup("", t.Sel.Name)
	}
	return nil
}

// lookup finds a struct by name: in dir first, then anywhere in the app when
// the name is unambiguous.
func (s *scanner) lookup(dir, name string) *structDecl {
	if dir != "" {
		if sd, ok := s.structs[dir+"."+name]; ok {
			return sd
		}
	}
	all := s.byName[name]
	if len(all) == 1 {
		return all[0]
	}
	if dir == "" && len(all) > 1 {
		return all[0]
	}
	return nil
}

// object writes one interface for a struct and returns its name.
func (s *scanner) object(sd *structDecl) string {
	name := sd.name
	for _, o := range s.res.Objects {
		if o.Name == name {
			return name
		}
	}
	for _, up := range s.stack {
		if up == name {
			return name // a struct that points at itself: the name is enough
		}
	}
	s.stack = append(s.stack, name)
	obj := Object{Name: name, Doc: sd.doc}
	obj.Props = s.fields(sd)
	s.stack = s.stack[:len(s.stack)-1]
	s.res.Objects = append(s.res.Objects, obj)
	sort.Slice(s.res.Objects, func(i, j int) bool { return s.res.Objects[i].Name < s.res.Objects[j].Name })
	return name
}

func (s *scanner) fields(sd *structDecl) []Prop {
	var out []Prop
	for _, f := range sd.spec.Fields.List {
		name, opt, skip := jsonName(f)
		if skip {
			continue
		}
		if len(f.Names) == 0 { // embedded: encoding/json flattens it
			if inner := s.embedded(sd.dir, f.Type); inner != nil && name == "" {
				out = append(out, s.fields(inner)...)
				continue
			}
		}
		if name == "" {
			continue
		}
		ts, optional := s.tsType(sd.dir, f.Type)
		out = append(out, Prop{Name: name, Type: ts, Optional: opt || optional, Doc: docText(f.Doc)})
	}
	return out
}

func (s *scanner) embedded(dir string, e ast.Expr) *structDecl {
	if st, ok := e.(*ast.StarExpr); ok {
		e = st.X
	}
	switch t := e.(type) {
	case *ast.Ident:
		return s.lookup(dir, t.Name)
	case *ast.SelectorExpr:
		return s.lookup(dir, t.Sel.Name)
	}
	return nil
}

// tsType maps a Go type to what JSON.parse gives the island. The second result
// says the field may be missing.
func (s *scanner) tsType(dir string, e ast.Expr) (string, bool) {
	switch t := e.(type) {
	case *ast.StarExpr:
		inner, _ := s.tsType(dir, t.X)
		return inner, true
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && id.Name == "byte" {
			return "string", false // encoding/json writes bytes as base64
		}
		inner, _ := s.tsType(dir, t.Elt)
		return inner + "[]", false
	case *ast.MapType:
		val, _ := s.tsType(dir, t.Value)
		return "Record<string, " + val + ">", false
	case *ast.InterfaceType:
		return "unknown", false
	case *ast.SelectorExpr:
		return s.qualified(dir, t), false
	case *ast.Ident:
		return s.ident(dir, t.Name), false
	case *ast.StructType:
		return s.inline(dir, t), false
	}
	return "unknown", false
}

// qualified maps the few standard types an app really puts in props.
func (s *scanner) qualified(dir string, t *ast.SelectorExpr) string {
	pkg, _ := t.X.(*ast.Ident)
	if pkg != nil {
		switch pkg.Name + "." + t.Sel.Name {
		case "time.Time":
			return "string" // RFC 3339, the way encoding/json writes it
		case "time.Duration":
			return "number"
		case "json.RawMessage":
			return "unknown"
		case "json.Number":
			return "number"
		}
	}
	if sd := s.lookup("", t.Sel.Name); sd != nil {
		return s.object(sd)
	}
	return "unknown"
}

func (s *scanner) ident(dir, name string) string {
	switch name {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16",
		"uint32", "uint64", "uintptr", "float32", "float64", "byte", "rune":
		return "number"
	case "any", "error":
		return "unknown"
	case "complex64", "complex128":
		return "unknown" // encoding/json refuses these anyway
	}
	if sd := s.lookup(dir, name); sd != nil {
		return s.object(sd)
	}
	if under, ok := s.named[dir+"."+name]; ok {
		ts, _ := s.tsType(dir, under)
		return ts
	}
	for d := range s.structs {
		if strings.HasSuffix(d, "."+name) {
			return s.object(s.structs[d])
		}
	}
	for k, under := range s.named {
		if strings.HasSuffix(k, "."+name) {
			ts, _ := s.tsType(dir, under)
			return ts
		}
	}
	return "unknown"
}

// inline writes an anonymous struct as an object type on one line.
func (s *scanner) inline(dir string, t *ast.StructType) string {
	sd := &structDecl{dir: dir, spec: t}
	props := s.fields(sd)
	if len(props) == 0 {
		return "Record<string, unknown>"
	}
	parts := make([]string, 0, len(props))
	for _, p := range props {
		q := p.Name
		if p.Optional {
			q += "?"
		}
		parts = append(parts, q+": "+p.Type)
	}
	return "{ " + strings.Join(parts, "; ") + " }"
}

func (s *scanner) note(format string, a ...any) {
	s.res.Notes = append(s.res.Notes, fmt.Sprintf(format, a...))
}

func (s *scanner) where(fi fileInfo, n ast.Node) string {
	pos := s.fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", filepath.Base(fi.path), pos.Line)
}

// jsonName reads the json tag: the name it gives, whether omitempty makes the
// field optional, and whether the field is out of the document altogether.
func jsonName(f *ast.Field) (name string, optional, skip bool) {
	if len(f.Names) == 1 {
		name = f.Names[0].Name
		if !ast.IsExported(name) {
			return "", false, true
		}
	} else if len(f.Names) > 1 {
		return "", false, true // trilha reads one field per line
	}
	if f.Tag == nil {
		return name, false, false
	}
	tag, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return name, false, false
	}
	j, ok := lookupTag(tag, "json")
	if !ok {
		return name, false, false
	}
	parts := strings.Split(j, ",")
	if parts[0] == "-" && len(parts) == 1 {
		return "", false, true
	}
	if parts[0] != "" {
		name = parts[0]
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			optional = true
		}
	}
	return name, optional, false
}

// lookupTag is reflect.StructTag.Get without importing reflect for one line.
func lookupTag(tag, key string) (string, bool) {
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := tag[:i]
		tag = tag[i+1:]
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		val := tag[:i+1]
		tag = tag[i+1:]
		if name == key {
			v, err := strconv.Unquote(val)
			if err != nil {
				return "", false
			}
			return v, true
		}
	}
	return "", false
}

func isNil(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "nil"
}

func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return strings.TrimSpace(g.Text())
}

func goFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (name == "testdata" || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}
