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
	"path"
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
		imports: map[string]map[string]string{},
		pkgName: map[string]string{},
		noted:   map[string]bool{},
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
		s.pkgName[filepath.Dir(path)] = f.Name.Name
	}
	app := len(s.files)
	if err := s.modulePackages(root); err != nil {
		return nil, err
	}
	for _, fi := range s.files {
		s.index(fi)
	}
	for _, fi := range s.files[:app] { // islands are mounted by the app, not by what it imports
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
	structs map[string]*structDecl       // dir + "." + name
	byName  map[string][]*structDecl     // name, for a type reached through another package
	named   map[string]ast.Expr          // dir + "." + name, for a defined non-struct type
	seen    map[string]bool              // island sources already recorded
	stack   []string                     // structs being written, to break a cycle
	imports map[string]map[string]string // dir → package name in its files → dir of the package, or "ext:" + import path
	noted   map[string]bool              // notes already written
	pkgName map[string]string            // dir → the package clause of its files
	results map[string]ast.Expr          // dir + "." + func name → its first result type
	res     *Result
}

// modulePackages reads, besides the app, every package of the same module
// the app reaches through its imports — the client trilha client wrote in
// internal/api, a domain package — so a props field typed with them gets its
// interface instead of unknown (#299). Packages outside the module are only
// recorded, for the note that says why their types stay unknown.
func (s *scanner) modulePackages(root string) error {
	modRoot, modPath := findModule(root)
	read := map[string]bool{}
	for _, fi := range s.files {
		read[fi.dir] = true
	}
	for i := 0; i < len(s.files); i++ {
		fi := s.files[i]
		for _, im := range fi.file.Imports {
			ip, err := strconv.Unquote(im.Path.Value)
			if err != nil {
				continue
			}
			target := "ext:" + ip
			if modPath != "" && strings.HasPrefix(ip, modPath+"/") {
				target = filepath.Join(modRoot, filepath.FromSlash(strings.TrimPrefix(ip, modPath+"/")))
			}
			if !strings.HasPrefix(target, "ext:") && !read[target] {
				read[target] = true
				if err := s.readPackage(target); err != nil {
					return err
				}
			}
			// The name the file uses: its alias, or the package clause of the
			// package — a folder slug_ holds package slug.
			name := path.Base(ip)
			if im.Name != nil {
				name = im.Name.Name
			} else if n, ok := s.pkgName[target]; ok {
				name = n
			}
			if s.imports[fi.dir] == nil {
				s.imports[fi.dir] = map[string]string{}
			}
			s.imports[fi.dir][name] = target
		}
	}
	return nil
}

// readPackage parses the non-test files of one package directory.
func (s *scanner) readPackage(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // an import that does not resolve is the compiler's to report
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		p := filepath.Join(dir, n)
		f, err := parser.ParseFile(s.fset, p, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		s.pkgName[dir] = f.Name.Name
		s.files = append(s.files, fileInfo{path: p, dir: dir, file: f})
	}
	return nil
}

// findModule walks up from dir to the go.mod and returns its directory and
// module path, or two empty strings.
func findModule(dir string) (string, string) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", ""
	}
	for {
		if b, err := os.ReadFile(filepath.Join(abs, "go.mod")); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if f := strings.Fields(line); len(f) == 2 && f[0] == "module" {
					return abs, strings.Trim(f[1], `"`)
				}
			}
			return "", ""
		}
		up := filepath.Dir(abs)
		if up == abs {
			return "", ""
		}
		abs = up
	}
}

// index records every struct and every defined type, so a props type declared
// next to the page can be found by name.
func (s *scanner) index(fi fileInfo) {
	for _, d := range fi.file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Type.Results != nil && len(fd.Type.Results.List) > 0 {
			if s.results == nil {
				s.results = map[string]ast.Expr{}
			}
			s.results[fi.dir+"."+fd.Name.Name] = fd.Type.Results.List[0].Type
		}
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

// calls walks a file for c.Island("/x.js", Props{...}), knowing the function
// each call is in: a variable or a parameter is read from there (#303).
func (s *scanner) calls(fi fileInfo) {
	for _, d := range fi.file.Decls {
		fd, _ := d.(*ast.FuncDecl)
		s.callsIn(fi, d, fd)
	}
}

func (s *scanner) callsIn(fi fileInfo, root ast.Node, fn *ast.FuncDecl) {
	ast.Inspect(root, func(n ast.Node) bool {
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
		s.island(fi, src, call.Args[1], fn)
		return true
	})
}

func (s *scanner) island(fi fileInfo, src string, props ast.Expr, fn *ast.FuncDecl) {
	sd := s.propsType(fi, props, fn, 0)
	if sd == nil {
		if !isNil(props) {
			s.note("%s: the type of the props of %s cannot be read, so it is unknown; pass a props struct (a literal, a variable, a parameter or a function that returns one)", s.where(fi, props), src)
		}
		s.res.Islands = append(s.res.Islands, Island{Src: src})
		return
	}
	name := s.object(sd)
	s.res.Islands = append(s.res.Islands, Island{Src: src, Type: name, From: sd.name})
}

// propsType finds the struct behind the second argument, from the type of the
// expression (#303): Props{...} and &Props{...}; a variable or a parameter of
// the function the call is in; a conversion Props(x); a call to a function of
// the app that returns the struct. Anything else has no name to hang a type on.
func (s *scanner) propsType(fi fileInfo, e ast.Expr, fn *ast.FuncDecl, depth int) *structDecl {
	if depth > 8 { // a := b; b := a never ends otherwise
		return nil
	}
	switch x := e.(type) {
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return s.propsType(fi, x.X, fn, depth+1)
		}
	case *ast.ParenExpr:
		return s.propsType(fi, x.X, fn, depth+1)
	case *ast.CompositeLit:
		return s.namedStruct(fi, x.Type)
	case *ast.CallExpr:
		if sd := s.namedStruct(fi, x.Fun); sd != nil {
			return sd // a conversion
		}
		if id, ok := x.Fun.(*ast.Ident); ok {
			return s.namedStruct(fi, s.results[fi.dir+"."+id.Name])
		}
	case *ast.Ident:
		if fn != nil {
			return s.localType(fi, x, fn, depth)
		}
	}
	return nil
}

// namedStruct is the struct a type expression names: Props, *Props, pkg.Props.
func (s *scanner) namedStruct(fi fileInfo, t ast.Expr) *structDecl {
	switch x := t.(type) {
	case *ast.StarExpr:
		return s.namedStruct(fi, x.X)
	case *ast.Ident:
		return s.lookup(fi.dir, x.Name)
	case *ast.SelectorExpr:
		return s.lookup("", x.Sel.Name)
	}
	return nil
}

// localType reads the type of a name used inside fn: a parameter, or the last
// declaration of it before the use (x := ..., var x T, var x = ...).
func (s *scanner) localType(fi fileInfo, id *ast.Ident, fn *ast.FuncDecl, depth int) *structDecl {
	for _, f := range fn.Type.Params.List {
		for _, n := range f.Names {
			if n.Name == id.Name {
				return s.namedStruct(fi, f.Type)
			}
		}
	}
	var found *structDecl
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil || n.Pos() >= id.Pos() {
			return n == nil || n.Pos() < id.Pos()
		}
		switch d := n.(type) {
		case *ast.AssignStmt:
			if d.Tok == token.DEFINE && len(d.Lhs) == len(d.Rhs) {
				for i, l := range d.Lhs {
					if l, ok := l.(*ast.Ident); ok && l.Name == id.Name {
						found = s.propsType(fi, d.Rhs[i], fn, depth+1)
					}
				}
			}
		case *ast.ValueSpec:
			for i, n := range d.Names {
				if n.Name != id.Name {
					continue
				}
				if d.Type != nil {
					found = s.namedStruct(fi, d.Type)
				} else if i < len(d.Values) {
					found = s.propsType(fi, d.Values[i], fn, depth+1)
				}
			}
		}
		return true
	})
	return found
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
	if pkg != nil {
		target := s.imports[dir][pkg.Name]
		if strings.HasPrefix(target, "ext:") {
			s.noteOnce("%s.%s comes from %s, outside this module, so its islands.d.ts type is unknown; "+
				"declare the field with a type of this module to get one", pkg.Name, t.Sel.Name, strings.TrimPrefix(target, "ext:"))
			return "unknown"
		}
		if target != "" {
			if sd, ok := s.structs[target+"."+t.Sel.Name]; ok {
				return s.object(sd)
			}
			if under, ok := s.named[target+"."+t.Sel.Name]; ok {
				ts, _ := s.tsType(target, under)
				return ts
			}
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

// noteOnce is note for what every use of a type would repeat.
func (s *scanner) noteOnce(format string, a ...any) {
	n := fmt.Sprintf(format, a...)
	if !s.noted[n] {
		s.noted[n] = true
		s.res.Notes = append(s.res.Notes, n)
	}
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
