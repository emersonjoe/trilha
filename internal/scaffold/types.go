package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha/internal/scan"
)

// errTypeNotFound is the answer for a name the project does not declare: the
// caller writes the struct instead of importing it.
var errTypeNotFound = errors.New("scaffold: type not found")

// typeInfo is a struct the project declares, seen from the outside: where it
// lives and what it carries. It is read by parsing, like internal/openapi does
// since 031 — no compiler, no module download.
type typeInfo struct {
	Name   string      // Comment
	Pkg    string      // package name that declares it
	Dir    string      // slash folder, relative to the project root
	Import string      // import path, empty when the module is unknown
	Fields []typeField // exported fields, in declaration order
}

// typeField is one field of that struct, with the tags the framework reads.
type typeField struct {
	Name     string // Go name
	Type     string // type expression as written
	JSON     string // json tag name, or the field name lowered
	Form     string // form tag name, or the json one
	Validate string // validate tag, as written
}

// Required reports whether the field has to be sent for the value to pass.
func (f typeField) Required() bool {
	for _, r := range strings.Split(f.Validate, ",") {
		if strings.TrimSpace(r) == "required" {
			return true
		}
	}
	return false
}

// rule returns the parameter of a validate rule ("min=3" gives "3") and
// whether the rule is there at all.
func (f typeField) rule(name string) (string, bool) {
	for _, r := range strings.Split(f.Validate, ",") {
		r = strings.TrimSpace(r)
		if r == name {
			return "", true
		}
		if v, ok := strings.CutPrefix(r, name+"="); ok {
			return v, true
		}
	}
	return "", false
}

// findType looks for a struct named name in the project. A qualified name
// ("posts.Comment") settles which package; a bare name declared twice is a
// refusal, because choosing for the caller is guessing, and a wrong guess only
// shows up at compile time.
func findType(root, module, name string) (typeInfo, error) {
	qual := ""
	if i := strings.LastIndex(name, "."); i >= 0 {
		qual, name = name[:i], name[i+1:]
	}
	if !token.IsIdentifier(name) {
		return typeInfo{}, fmt.Errorf("%q: a type name has to be a Go identifier", name)
	}
	var found []typeInfo
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil // the compiler complains about it, not this command
		}
		st := structDecl(f, name)
		if st == nil {
			return nil
		}
		if qual != "" && f.Name.Name != qual {
			return nil
		}
		dir := filepath.Dir(p)
		rel, rerr := filepath.Rel(root, dir)
		if rerr != nil {
			return nil
		}
		info := typeInfo{Name: name, Pkg: f.Name.Name, Dir: filepath.ToSlash(rel), Fields: fields(fset, st)}
		if info.Dir == "." {
			info.Dir = ""
		}
		if module != "" {
			info.Import = path.Join(module, info.Dir)
		}
		found = append(found, info)
		return nil
	})
	if err != nil {
		return typeInfo{}, err
	}
	switch len(found) {
	case 0:
		return typeInfo{}, fmt.Errorf("%s: %w", name, errTypeNotFound)
	case 1:
		return found[0], nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Dir < found[j].Dir })
	var where []string
	for _, f := range found {
		where = append(where, f.Pkg+"."+name+" ("+f.Dir+")")
	}
	return typeInfo{}, fmt.Errorf("%s is declared in more than one package: %s — write the qualified name",
		name, strings.Join(where, " and "))
}

// skipDir is the same rule the rest of the project scans by: a folder starting
// with a dot or an underscore is not source of the app, and .well-known is the
// documented exception.
func skipDir(name string) bool {
	if name == scan.WellKnown {
		return false
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") ||
		name == "testdata" || name == "vendor" || name == "node_modules"
}

func structDecl(f *ast.File, name string) *ast.StructType {
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, s := range gd.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				return st
			}
		}
	}
	return nil
}

func fields(fset *token.FileSet, st *ast.StructType) []typeField {
	var out []typeField
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			continue // embedded: it has no name to bind by
		}
		tag := ""
		if f.Tag != nil {
			if v, err := strconv.Unquote(f.Tag.Value); err == nil {
				tag = v
			}
		}
		st := reflect.StructTag(tag)
		for _, n := range f.Names {
			if !n.IsExported() {
				continue
			}
			tf := typeField{Name: n.Name, Type: exprString(fset, f.Type), Validate: st.Get("validate")}
			tf.JSON = tagName(st.Get("json"), strings.ToLower(n.Name))
			tf.Form = tagName(st.Get("form"), tf.JSON)
			if tf.JSON == "-" || tf.Form == "-" {
				continue
			}
			out = append(out, tf)
		}
	}
	return out
}

func tagName(tag, fallback string) string {
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return fallback
	}
	return name
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, e); err != nil {
		return ""
	}
	return buf.String()
}

// exampleValue is a value the field accepts, built from the tags: it is what
// makes a generated test go through the validation instead of bumping into it.
// A field it cannot answer for is left out, and the validation says so, which
// is better than a body that passes by accident.
func exampleValue(f typeField) (any, bool) {
	t := strings.TrimPrefix(f.Type, "*")
	switch {
	case t == "string":
		return exampleString(f), true
	case t == "bool":
		return true, true
	case strings.HasPrefix(t, "int") || strings.HasPrefix(t, "uint") || strings.HasPrefix(t, "float"):
		return exampleNumber(f), true
	}
	return nil, false
}

func exampleString(f typeField) string {
	if v, ok := f.rule("oneof"); ok {
		if first, _, _ := strings.Cut(strings.TrimSpace(v), " "); first != "" {
			return first
		}
	}
	if _, ok := f.rule("email"); ok {
		return "someone@example.com"
	}
	if _, ok := f.rule("url"); ok {
		return "https://example.com"
	}
	s := "example"
	if v, ok := f.rule("len"); ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return fit(s, n, n)
		}
	}
	min, max := 0, 0
	if v, ok := f.rule("min"); ok {
		min, _ = strconv.Atoi(v)
	}
	if v, ok := f.rule("max"); ok {
		max, _ = strconv.Atoi(v)
	}
	return fit(s, min, max)
}

// fit grows or cuts s to sit between min and max characters.
func fit(s string, min, max int) string {
	for len(s) < min {
		s += "x"
	}
	if max > 0 && len(s) > max {
		s = s[:max]
	}
	return s
}

func exampleNumber(f typeField) int {
	n := 1
	if v, ok := f.rule("min"); ok {
		if m, err := strconv.Atoi(v); err == nil && m > n {
			n = m
		}
	}
	if v, ok := f.rule("max"); ok {
		if m, err := strconv.Atoi(v); err == nil && m < n {
			n = m
		}
	}
	return n
}

// writeFile is the one write of this package that is not Go source: the caller
// already formatted whatever needs formatting.
func writeFile(root, rel string, body []byte, force bool) error {
	dst := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(dst); err == nil && !force {
		return fmt.Errorf("%s: %w", rel, ErrGenExists)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, body, 0o644)
}
