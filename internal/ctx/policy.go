package ctx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha/internal/scan"
)

// Policy is the module and level a route demands, and the file that says so.
// It is what somebody would otherwise learn by opening the folder's
// middleware.go and following a variable.
type Policy struct {
	Module string `json:"module"`
	Level  string `json:"level"`
	From   string `json:"from"`
}

// policies reads every middleware.go of the project and answers what each one
// demands, keyed by the file path the route map uses.
//
// The reading is static and resolves one indirection: the call inside
// Middleware, or the package-level var that Middleware returns. It does not
// resolve two, and it does not guess — a call it cannot read simply does not
// appear, because a map that invents an answer is worse than a map that is
// quiet about one folder.
func policies(root string) map[string]Policy {
	makers := policyMakers(root)
	out := map[string]Policy{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil
		case d.IsDir():
			if pulaDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		case d.Name() != "middleware.go":
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		for nome, p := range lePolitica(path, makers) {
			p.From = filepath.ToSlash(rel)
			out[filepath.ToSlash(rel)+"#"+nome] = p
		}
		return nil
	})
	return out
}

// lePolitica answers the policy of each exported Middleware function of one
// file, keyed by the function's name: Middleware, MiddlewarePOST, and so on.
// A folder readable at one level and writable at another is the case the
// matrix exists to express, so the answer is per function and not per file.
func lePolitica(path string, makers map[string]bool) map[string]Policy {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	// First the package-level vars: `var ver = ...RequirePolicy(P, "docs", "ver")`.
	vars := map[string]Policy{}
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
				if p, ok := politicaDe(vs.Values[i], makers); ok {
					vars[nome.Name] = p
				}
			}
		}
	}
	out := map[string]Policy{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Middleware") {
			continue
		}
		if p, ok := politicaDaFuncao(fn, vars, makers); ok {
			out[fn.Name.Name] = p
		}
	}
	return out
}

// politicaDaFuncao reads the body of a Middleware function. Two shapes reach
// here, and they are the two the documentation shows: returning the var, or
// calling the guard inline.
func politicaDaFuncao(fn *ast.FuncDecl, vars map[string]Policy, makers map[string]bool) (Policy, bool) {
	var achou Policy
	var ok bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ok {
			return false
		}
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		// `return ver(c, next)` — the var declared above.
		if id, isID := call.Fun.(*ast.Ident); isID {
			if p, found := vars[id.Name]; found {
				achou, ok = p, true
				return false
			}
			return true
		}
		// `return acesso.Auth.RequirePolicy(P, "docs", "ver")(c, next)`.
		if inner, isCall := call.Fun.(*ast.CallExpr); isCall {
			if p, found := politicaDe(inner, makers); found {
				achou, ok = p, true
				return false
			}
		}
		if p, found := politicaDe(call, makers); found {
			achou, ok = p, true
			return false
		}
		return true
	})
	return achou, ok
}

// politicaDe reads a call that declares a guard. It answers for
// `x.RequirePolicy(p, "module", "level")` and for a project function that
// forwards its two string parameters to one — the wrapper the auth
// documentation shows, and the shape examples/local-login uses.
func politicaDe(e ast.Expr, makers map[string]bool) (Policy, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return Policy{}, false
	}
	nome := ""
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		nome = fun.Sel.Name
	case *ast.Ident:
		nome = fun.Name
	default:
		return Policy{}, false
	}
	args := call.Args
	if nome == "RequirePolicy" {
		if len(args) != 3 {
			return Policy{}, false
		}
		args = args[1:] // o primeiro é a matriz
	} else if makers[nome] {
		if len(args) != 2 {
			return Policy{}, false
		}
	} else {
		return Policy{}, false
	}
	modulo, ok1 := literal(args[0])
	nivel, ok2 := literal(args[1])
	if !ok1 || !ok2 {
		// Um módulo que vem de variável precisaria de um compilador, e um mapa
		// que chuta é pior que um mapa calado.
		return Policy{}, false
	}
	return Policy{Module: modulo, Level: nivel}, true
}

// policyMakers finds the project's own wrappers: a function taking two strings
// and handing them to RequirePolicy.
//
//	func Exige(modulo, nivel string) trilha.MiddlewareFunc {
//		return Flow.RequirePolicy(Policy, modulo, nivel)
//	}
//
// It is one hop and not a general resolver: the wrapper is the idiom the
// documentation teaches, so recognising it is recognising what people write —
// and refusing it would ship a reading that does not work on this repository's
// own example.
func policyMakers(root string) map[string]bool {
	out := map[string]bool{}
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
		if err != nil || !strings.Contains(string(src), "RequirePolicy") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			if a, b, ok := doisParametrosString(fn); ok && encaminha(fn.Body, a, b) {
				out[fn.Name.Name] = true
			}
		}
		return nil
	})
	return out
}

// doisParametrosString answers the names of a function's two string
// parameters, when that is all it takes.
func doisParametrosString(fn *ast.FuncDecl) (string, string, bool) {
	var nomes []string
	for _, campo := range fn.Type.Params.List {
		id, ok := campo.Type.(*ast.Ident)
		if !ok || id.Name != "string" {
			return "", "", false
		}
		for _, n := range campo.Names {
			nomes = append(nomes, n.Name)
		}
	}
	if len(nomes) != 2 {
		return "", "", false
	}
	return nomes[0], nomes[1], true
}

// encaminha reports whether the body hands those two names to RequirePolicy,
// in that order.
func encaminha(body *ast.BlockStmt, a, b string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "RequirePolicy" || len(call.Args) != 3 {
			return true
		}
		x, ok1 := call.Args[1].(*ast.Ident)
		y, ok2 := call.Args[2].(*ast.Ident)
		found = ok1 && ok2 && x.Name == a && y.Name == b
		return !found
	})
	return found
}

// politicaDaRota picks the policy that actually guards a route: the deepest
// declaration wins, exactly the way the middleware chain itself is inherited —
// the chain arrives outermost first, so the last one to declare a policy is
// the one nearest the route.
func politicaDaRota(chain []scan.Ref, tabela map[string]Policy, fn string) (Policy, bool) {
	var achou Policy
	var ok bool
	for _, ref := range chain {
		if p, found := tabela[refFile(ref, "middleware.go")+"#"+fn]; found {
			achou, ok = p, true
		}
	}
	return achou, ok
}

func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

// pulaDir is what a project map has no business reading: the repository's own
// plumbing, and the synthetic trees that exist to be broken.
func pulaDir(nome string) bool {
	switch nome {
	case ".git", "node_modules", "vendor", "testdata", ".trilha":
		return true
	}
	return false
}
