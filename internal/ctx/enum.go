package ctx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Enum is a domain list the project registered: the statuses a document can
// be in, the types a contract can have.
//
// It is on the map because somebody who does not know a list exists invents a
// second one — and then two screens spell "in review" two ways, and a report
// counts neither.
type Enum struct {
	// Name is what RegisterEnum called it, which is also what a validate tag
	// names: `validate:"enum=documento.status"`.
	Name string `json:"name"`
	// From is the file the values were read from.
	From   string      `json:"from"`
	Values []EnumValue `json:"values"`
}

// EnumValue is one entry: what goes in the column, what a person reads, and
// the tone a screen paints it with.
type EnumValue struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

// enums reads the project's registered lists.
//
// The registration is a runtime call and the map is read from source, so this
// does the joining: it finds `trilha.RegisterEnum("name", X)` and then the
// declaration of X. A list declared but never registered does not appear —
// nothing looks it up by name either — and a name registered from something
// this cannot read does not appear, because a guess on the map is worse than
// a gap.
func enums(root string) []Enum {
	decls := map[string][]EnumValue{} // nome da var → valores
	files := map[string]string{}      // nome da var → arquivo
	registros := map[string]string{}  // nome registrado → nome da var

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case d.IsDir():
			if pulaDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		texto := string(src)
		if !strings.Contains(texto, "Enum") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nome := range vs.Names {
					if i >= len(vs.Values) {
						break
					}
					if vals, ok := valoresDe(vs.Values[i]); ok {
						decls[nome.Name] = vals
						files[nome.Name] = rel
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "RegisterEnum" {
				return true
			}
			nome, ok := literal(call.Args[0])
			if !ok {
				return true
			}
			switch v := call.Args[1].(type) {
			case *ast.Ident:
				registros[nome] = v.Name
			case *ast.SelectorExpr:
				registros[nome] = v.Sel.Name
			}
			return true
		})
		return nil
	})

	out := make([]Enum, 0, len(registros))
	for nome, varName := range registros {
		vals, ok := decls[varName]
		if !ok {
			continue
		}
		out = append(out, Enum{Name: nome, From: files[varName], Values: vals})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// valoresDe reads a `trilha.Enum{...}` literal. Anything else — a slice built
// by a function, a list appended to at startup — is not read: it would take a
// compiler, and the map does not guess.
func valoresDe(e ast.Expr) ([]EnumValue, bool) {
	lit, ok := e.(*ast.CompositeLit)
	if !ok || lit.Type == nil {
		return nil, false
	}
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Enum" {
		return nil, false
	}
	out := make([]EnumValue, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		item, ok := elt.(*ast.CompositeLit)
		if !ok {
			return nil, false
		}
		v, ok := valorDe(item)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// valorDe reads one entry, written either with field names or positionally.
func valorDe(item *ast.CompositeLit) (EnumValue, bool) {
	var v EnumValue
	posicional := 0
	for _, elt := range item.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			// {"rascunho", "Rascunho"} — value, label, tone, in that order.
			s, ok := literal(elt)
			if !ok {
				return v, false
			}
			switch posicional {
			case 0:
				v.Value = s
			case 1:
				v.Label = s
			case 2:
				v.Tone = s
			}
			posicional++
			continue
		}
		campo, ok := kv.Key.(*ast.Ident)
		if !ok {
			return v, false
		}
		s, ok := literal(kv.Value)
		if !ok {
			return v, false
		}
		switch campo.Name {
		case "Value":
			v.Value = s
		case "Label":
			v.Label = s
		case "Tone":
			v.Tone = s
		}
	}
	return v, v.Value != ""
}
