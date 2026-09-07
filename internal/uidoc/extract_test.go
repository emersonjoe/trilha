package uidoc

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// uiDir is the package this catalogue describes, from the test's directory.
const uiDir = "../../ui"

var (
	bannerRe = regexp.MustCompile(`^-+ +(.+?) +-+$`)
	seeRe    = regexp.MustCompile(`\b(ui|trilha|h|ai)\.[A-Z][A-Za-z0-9]*\b`)
)

// extract reads the ui package and builds the catalogue. It lives in a test
// file on purpose: the CLI ships the JSON, not the parser.
func extract() ([]Component, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, uiDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	pkg := pkgs["ui"]
	files := make([]string, 0, len(pkg.Files))
	for name := range pkg.Files {
		files = append(files, name)
	}
	sort.Strings(files)

	// banners maps a file to its "// ---- name ----" markers, in order.
	type banner struct {
		pos  token.Pos
		name string
	}
	banners := map[string][]banner{}
	for _, name := range files {
		for _, cg := range pkg.Files[name].Comments {
			for _, c := range cg.List {
				text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				if m := bannerRe.FindStringSubmatch(text); m != nil {
					banners[name] = append(banners[name], banner{c.Pos(), m[1]})
				}
			}
		}
	}
	groupOf := func(file string, pos token.Pos) string {
		g := strings.TrimSuffix(filepath.Base(file), ".go")
		for _, b := range banners[file] {
			if b.pos < pos {
				g = b.name
			}
		}
		return g
	}

	// docOf falls back to the comment of the group a symbol belongs to: a run
	// of declarations with no blank line between them shares one comment, and
	// that is how Outline and Ghost are documented.
	var out []Component
	for _, name := range files {
		f := pkg.Files[name]
		lines := func(p token.Pos) int { return fset.Position(p).Line }
		lastDoc, lastEnd := "", -2
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil || !d.Name.IsExported() {
					lastDoc, lastEnd = "", -2
					continue
				}
				text := d.Doc.Text()
				if text == "" && lines(d.Pos()) == lastEnd+1 {
					text = lastDoc
				} else {
					lastDoc = text
				}
				lastEnd = lines(d.End())
				var b strings.Builder
				_ = printer.Fprint(&b, fset, d.Type)
				out = append(out, component(d.Name.Name, "func",
					groupOf(name, d.Pos()), d.Name.Name+strings.TrimPrefix(b.String(), "func"), text, nil))
			case *ast.GenDecl:
				lastDoc, lastEnd = "", -2
				if d.Tok != token.TYPE {
					continue
				}
				for _, s := range d.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					text := ts.Doc.Text()
					if text == "" {
						text = d.Doc.Text()
					}
					out = append(out, component(ts.Name.Name, "type",
						groupOf(name, ts.Pos()), "type "+ts.Name.Name+" "+typeName(fset, ts.Type), text, fieldsOf(fset, ts)))
				}
			}
		}
	}
	return out, nil
}

// component splits the doc comment into the text, the first sentence and the
// indented example, and collects the symbols the text mentions.
func component(name, kind, group, sig, text string, fields []Field) Component {
	var body, example []string
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if strings.HasPrefix(line, "\t") {
			example = append(example, strings.TrimPrefix(line, "\t"))
			continue
		}
		if len(example) > 0 && strings.TrimSpace(line) == "" {
			example = append(example, "")
			continue
		}
		body = append(body, line)
	}
	c := Component{
		Name: name, Kind: kind, Group: group, Signature: sig,
		Doc:     strings.TrimSpace(strings.Join(body, "\n")),
		Example: strings.TrimSpace(strings.Join(example, "\n")),
		Fields:  fields,
	}
	c.Summary = firstSentence(c.Doc)
	seen := map[string]bool{"ui." + name: true}
	for _, m := range seeRe.FindAllString(c.Doc+"\n"+c.Example, -1) {
		if !seen[m] {
			seen[m] = true
			c.See = append(c.See, m)
		}
	}
	sort.Strings(c.See)
	return c
}

// firstSentence is the summary line: up to the first period that ends a word.
func firstSentence(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	for i := 0; i < len(s); i++ {
		if s[i] != '.' {
			continue
		}
		if i+1 == len(s) || s[i+1] == ' ' {
			return strings.TrimSpace(s[:i+1])
		}
	}
	return strings.TrimSpace(s)
}

// fieldsOf lists the exported fields of a struct type with their own comment.
func fieldsOf(fset *token.FileSet, ts *ast.TypeSpec) []Field {
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return nil
	}
	var out []Field
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if !n.IsExported() {
				continue
			}
			var b strings.Builder
			_ = printer.Fprint(&b, fset, f.Type)
			out = append(out, Field{Name: n.Name, Type: b.String(), Doc: firstSentence(strings.TrimSpace(f.Doc.Text()))})
		}
	}
	return out
}

// typeName renders the underlying type in one word where it has one.
func typeName(fset *token.FileSet, e ast.Expr) string {
	switch e.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	}
	var b strings.Builder
	_ = printer.Fprint(&b, fset, e)
	return b.String()
}
