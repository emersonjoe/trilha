package scaffold

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/emersonjoe/trilha/internal/scan"
)

// generateTest writes the test next to the route. The route comes from the
// same scanner that writes trilha_gen.go, so the file knows the methods that
// exist on disk, not the ones a flag asked for.
func generateTest(root string, o GenOptions) (GenResult, error) {
	t, err := o.texts()
	if err != nil {
		return GenResult{}, err
	}
	segs, err := parseURL(o.Arg)
	if err != nil {
		return GenResult{}, err
	}
	var patternSegs []string
	for _, s := range segs {
		if s.pattern != "" {
			patternSegs = append(patternSegs, s.pattern)
		}
	}
	pattern := "/" + strings.Join(patternSegs, "/")
	res, err := scan.Scan(root, o.Module)
	if err != nil {
		return GenResult{}, err
	}
	var r *scan.Route
	for i := range res.Routes {
		if res.Routes[i].Pattern == pattern {
			r = &res.Routes[i]
			break
		}
	}
	if r == nil {
		return GenResult{}, fmt.Errorf("no route answers %s: write the page or the route first", pattern)
	}
	abs := filepath.Join(root, filepath.FromSlash(r.Dir))
	pkg, err := packageFor(abs, path.Base(r.Dir))
	if err != nil {
		return GenResult{}, err
	}
	file := "route_test.go"
	if r.HasPage {
		file = "page_test.go"
	}
	rel := path.Join(r.Dir, file)
	src, err := testSource(root, o, r, pkg, t)
	if err != nil {
		return GenResult{}, err
	}
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return GenResult{}, fmt.Errorf("%s: %w", rel, err)
	}
	if err := writeFile(root, rel, formatted, o.Force); err != nil {
		return GenResult{}, err
	}
	return GenResult{File: rel, Pattern: pattern, Package: pkg}, nil
}

// testSource writes the file: one case per method, with the target the router
// resolves and, when the handler binds a type this project declares, a body
// built from its tags.
func testSource(root string, o GenOptions, r *scan.Route, pkg string, t map[string]string) (string, error) {
	target := testTarget(r.Pattern)
	name := "Test" + camel(pkg)
	var body, imps strings.Builder
	uses := map[string]bool{}
	if r.HasPage {
		fmt.Fprintf(&body, "// %sPage %s\nfunc %sPage(t *testing.T) {\n", name, t["gen_test_page"], name)
		fmt.Fprintf(&body, "\tres := trilha.TestPage(t, trilha.Route{Pattern: %q, Page: Page}, %q)\n", r.Pattern, target)
		body.WriteString("\tres.WantStatus(200)\n\tif res.Node == nil {\n\t\tt.Fatal(\"the page returned no node\")\n\t}\n}\n")
	}
	if len(r.Methods) > 0 {
		if r.HasPage {
			body.WriteString("\n")
		}
		fmt.Fprintf(&body, "// %sRoute %s\nfunc %sRoute(t *testing.T) {\n", name, t["gen_test_route"], name)
		fmt.Fprintf(&body, "\troute := trilha.Route{Pattern: %q%s, Methods: map[string]trilha.HandlerFunc{%s}}\n",
			r.Pattern, pageField(r.HasPage), methodMap(r.Methods))
		for _, m := range r.Methods {
			call, err := testCall(root, o, r, m, target)
			if err != nil {
				return "", err
			}
			body.WriteString(call)
			if strings.Contains(call, "url.Values") {
				uses["url"] = true
			}
		}
		body.WriteString("}\n")
	}
	imps.WriteString("import (\n")
	if uses["url"] {
		imps.WriteString("\t\"net/url\"\n")
	}
	imps.WriteString("\t\"testing\"\n\n\t\"github.com/emersonjoe/trilha\"\n)\n")
	return fmt.Sprintf("package %s\n\n%s\n%s", pkg, imps.String(), body.String()), nil
}

func pageField(hasPage bool) string {
	if hasPage {
		return ", Page: Page"
	}
	return ""
}

func methodMap(methods []string) string {
	var parts []string
	for _, m := range methods {
		parts = append(parts, fmt.Sprintf("%q: %s", m, m))
	}
	return strings.Join(parts, ", ")
}

// testCall writes one request. A method with a body sends what the type it
// binds accepts; when the type is not this project's to read, it sends the
// emptiest body there is and checks the status the handler answers to it.
func testCall(root string, o GenOptions, r *scan.Route, method, target string) (string, error) {
	if !hasBody(method) {
		return fmt.Sprintf("\ttrilha.TestRoute(t, route, %q, %q).WantStatus(200)\n", method, target), nil
	}
	info, found := bindInfo(root, o, r, method)
	if r.HasPage {
		// A page answers a form, and the answer ends in the redirect of POST → GET.
		return fmt.Sprintf("\ttrilha.TestRoute(t, route, %q, %q,\n\t\ttrilha.WithForm(url.Values{%s})).WantStatus(303)\n",
			method, target, formValues(info, found)), nil
	}
	status := "200"
	if found {
		status = successStatus(method)
	}
	return fmt.Sprintf("\ttrilha.TestRoute(t, route, %q, %q,\n\t\ttrilha.WithBody(\"application/json\", `%s`)).WantStatus(%s)\n",
		method, target, jsonBody(info, found), status), nil
}

// bindInfo reads the handler to find what it binds, and looks that type up in
// the project.
func bindInfo(root string, o GenOptions, r *scan.Route, method string) (typeInfo, bool) {
	file := "route.go"
	if r.HasPage {
		file = "page.go"
	}
	name := bindTypeName(filepath.Join(root, filepath.FromSlash(r.Dir)), file, method)
	if name == "" {
		return typeInfo{}, false
	}
	info, err := findType(root, o.Module, name)
	if err != nil {
		return typeInfo{}, false // a type this project does not declare
	}
	return info, true
}

// required lists the fields that have to be sent, with a value each that the
// validation accepts.
func required(info typeInfo, found bool) []typeField {
	if !found {
		return nil
	}
	var out []typeField
	for _, f := range info.Fields {
		if !f.Required() {
			continue
		}
		if _, ok := exampleValue(f); ok {
			out = append(out, f)
		}
	}
	return out
}

// jsonBody writes the body of an API request, sorted by field name so the file
// is the same on every run.
func jsonBody(info typeInfo, found bool) string {
	fields := required(info, found)
	sort.Slice(fields, func(i, j int) bool { return fields[i].JSON < fields[j].JSON })
	var parts []string
	for _, f := range fields {
		v, _ := exampleValue(f)
		b, _ := json.Marshal(v)
		parts = append(parts, fmt.Sprintf("%q: %s", f.JSON, b))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// formValues writes the body of a form request: in a form everything travels
// as text.
func formValues(info typeInfo, found bool) string {
	fields := required(info, found)
	sort.Slice(fields, func(i, j int) bool { return fields[i].Form < fields[j].Form })
	var parts []string
	for _, f := range fields {
		v, _ := exampleValue(f)
		parts = append(parts, fmt.Sprintf("%q: {%q}", f.Form, fmt.Sprint(v)))
	}
	return strings.Join(parts, ", ")
}

// bindTypeName finds the type of the local variable the handler binds, which
// is the "var in X" the skeleton writes.
func bindTypeName(dir, file, method string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(dir, file), nil, 0)
	if err != nil {
		return ""
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != method || fn.Body == nil {
			continue
		}
		var found string
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if found != "" {
				return false
			}
			vs, ok := n.(*ast.ValueSpec)
			if !ok || vs.Type == nil {
				return true
			}
			switch t := vs.Type.(type) {
			case *ast.Ident:
				found = t.Name
			case *ast.SelectorExpr:
				if id, ok := t.X.(*ast.Ident); ok {
					found = id.Name + "." + t.Sel.Name
				}
			}
			return true
		})
		return found
	}
	return ""
}

// testTarget turns a pattern into a URL the router resolves: a parameter
// becomes its own name, which any handler written by this command accepts.
func testTarget(pattern string) string {
	var out []string
	for _, seg := range strings.Split(strings.Trim(pattern, "/"), "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") {
			seg = strings.TrimSuffix(strings.Trim(seg, "{}"), "...")
		}
		out = append(out, seg)
	}
	return "/" + strings.Join(out, "/")
}

// camel turns a package name into the middle of a test name: api_v2 becomes
// ApiV2.
func camel(s string) string {
	var sb strings.Builder
	up := true
	for _, r := range s {
		if r == '_' || r == '-' || r == '.' {
			up = true
			continue
		}
		if up {
			sb.WriteRune(unicode.ToUpper(r))
			up = false
			continue
		}
		sb.WriteRune(r)
	}
	if sb.Len() == 0 {
		return "App"
	}
	return sb.String()
}
