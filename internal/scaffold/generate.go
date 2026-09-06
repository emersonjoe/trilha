package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"unicode"

	"github.com/emersonjoe/trilha/internal/scan"
)

// Errors Generate returns for the two refusals a caller has to tell apart.
var (
	// ErrGenExists is a file already there: --force overrides it.
	ErrGenExists = errors.New("scaffold: file exists")
	// ErrGenConflict is page.go and route.go in the same folder, which the
	// scanner rejects. --force does not override a convention.
	ErrGenConflict = errors.New("scaffold: page and route in the same folder")
)

// DefaultComponentDir is where a component goes when nobody says otherwise.
const DefaultComponentDir = "internal/components"

// GenOptions is one generate invocation.
type GenOptions struct {
	Kind    string   // page | route | component | test
	Arg     string   // URL for page, route and test; exported name for component
	Force   bool     // overwrite an existing file
	Dir     string   // component destination; DefaultComponentDir when empty
	Methods []string // route: one handler per method
	Bind    string   // route: the type the body binds to
	Form    string   // page: the type the form binds to
	Layout  string   // page: layout.go to write when the folder has none
	Module  string   // import path of the project, for a type in another package
	Lang    string   // language of the generated comments; "en" when empty
}

// GenResult says what was written.
type GenResult struct {
	File    string // slash path relative to root
	Pattern string // URL the file answers; empty for a component
	Package string
	Extra   []string // other files this call wrote, like a missing layout
}

// texts returns the comments of the skeleton in the language asked for.
func (o GenOptions) texts() (map[string]string, error) {
	lang := o.Lang
	if lang == "" {
		lang = "en"
	}
	t, ok := texts[lang]
	if !ok {
		return nil, errors.New("scaffold: unknown language " + lang)
	}
	return t, nil
}

// check refuses a flag on a kind that has no use for it, before any file is
// written: a flag silently ignored is a lesson learned twice.
func (o GenOptions) check() error {
	switch o.Kind {
	case "route":
		if o.Form != "" || o.Layout != "" {
			return errors.New("--form and --layout are for a page; a route binds with --bind")
		}
	case "page":
		if o.Bind != "" {
			return errors.New("--bind is for a route; a page binds the form with --form")
		}
		if len(o.Methods) > 0 {
			return errors.New("a page answers GET, and POST when it has --form; any other method is a route")
		}
	default:
		if len(o.Methods) > 0 || o.Bind != "" || o.Form != "" || o.Layout != "" {
			return fmt.Errorf("--methods, --bind, --form and --layout are for a page or a route, not for %s", o.Kind)
		}
	}
	return nil
}

// normalizeMethods puts the methods asked for in the order the generated file
// lists them, and refuses what route.go could not export anyway.
func normalizeMethods(list []string) ([]string, error) {
	seen := map[string]bool{}
	for _, m := range list {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if !slices.Contains(scan.Methods, m) {
			return nil, fmt.Errorf("%s: route.go exports handlers named %s", m, strings.Join(scan.Methods, ", "))
		}
		if seen[m] {
			return nil, fmt.Errorf("%s: asked for twice", m)
		}
		seen[m] = true
	}
	var out []string
	for _, m := range scan.Methods {
		if seen[m] {
			out = append(out, m)
		}
	}
	return out, nil
}

// Generate writes one skeleton in the project at root. The argument of a page
// or a route is the URL: the folder convention is derived from it, which is
// the whole point — the convention is what costs to remember, not the file.
func Generate(root string, o GenOptions) (GenResult, error) {
	if err := o.check(); err != nil {
		return GenResult{}, err
	}
	switch o.Kind {
	case "page", "route":
		return generateRoute(root, o)
	case "component":
		return generateComponent(root, o)
	case "test":
		return generateTest(root, o)
	default:
		return GenResult{}, fmt.Errorf("unknown kind %q: page, route, component or test", o.Kind)
	}
}

type urlSegment struct {
	dir     string // folder name on disk
	pattern string // URL segment, empty for a route group
}

// parseURL turns a URL into the folders that answer it, the inverse of what
// internal/scan reads. Everything it refuses here is something the scanner
// would refuse later, with the app already half written.
func parseURL(u string) ([]urlSegment, error) {
	u = strings.TrimSpace(u)
	if u == "" {
		return nil, errors.New("empty path: give the URL the file answers, like /blog/{slug}")
	}
	if strings.Contains(u, `\`) {
		return nil, fmt.Errorf("%q: use / to separate segments", u)
	}
	trimmed := strings.Trim(u, "/")
	if trimmed == "" {
		return nil, nil // the root page
	}
	var segs []urlSegment
	for _, raw := range strings.Split(trimmed, "/") {
		if raw == "" {
			return nil, fmt.Errorf("%q: empty segment", u)
		}
		if raw == "." || raw == ".." {
			return nil, fmt.Errorf("%q: %q is not a path segment", u, raw)
		}
		seg, err := parseURLSegment(raw)
		if err != nil {
			return nil, err
		}
		if len(segs) > 0 && strings.HasSuffix(segs[len(segs)-1].dir, "__") {
			return nil, fmt.Errorf("%q: a catch-all must be the last segment", u)
		}
		segs = append(segs, seg)
	}
	return segs, nil
}

func parseURLSegment(raw string) (urlSegment, error) {
	invalid := func(reason string) (urlSegment, error) {
		return urlSegment{}, fmt.Errorf("%q: %s", raw, reason)
	}
	switch {
	case strings.HasSuffix(raw, "-"):
		// A route group: it wraps the subtree with a layout and adds no URL
		// segment.
		name := strings.TrimSuffix(raw, "-")
		if !token.IsIdentifier(name) {
			return invalid("a route group is a plain name followed by \"-\"")
		}
		return urlSegment{dir: raw}, nil
	case strings.HasPrefix(raw, "{"):
		if !strings.HasSuffix(raw, "}") {
			return invalid("unclosed {")
		}
		name := raw[1 : len(raw)-1]
		catchAll := strings.HasSuffix(name, "...")
		name = strings.TrimSuffix(name, "...")
		if !token.IsIdentifier(name) {
			return invalid("a parameter name has to be a Go identifier")
		}
		if catchAll {
			return urlSegment{dir: name + "__", pattern: "{" + name + "...}"}, nil
		}
		return urlSegment{dir: name + "_", pattern: "{" + name + "}"}, nil
	case strings.ContainsAny(raw, "{}"):
		return invalid("a parameter is written {name} or {name...}")
	case strings.HasSuffix(raw, "_"):
		// A folder ending in "_" is a parameter for the scanner; asking for it
		// as a literal is almost always a typo.
		return invalid("write a parameter as {name}, not as a folder ending in _")
	default:
		return urlSegment{dir: raw, pattern: raw}, nil
	}
}

func generateRoute(root string, o GenOptions) (GenResult, error) {
	segs, err := parseURL(o.Arg)
	if err != nil {
		return GenResult{}, err
	}
	dirs := []string{"app"}
	var patternSegs, params []string
	for _, s := range segs {
		dirs = append(dirs, s.dir)
		if s.pattern == "" {
			continue
		}
		patternSegs = append(patternSegs, s.pattern)
		if strings.HasPrefix(s.pattern, "{") {
			params = append(params, strings.TrimSuffix(strings.Trim(s.pattern, "{}"), "..."))
		}
	}
	dir := path.Join(dirs...)
	file := "page.go"
	other := "route.go"
	if o.Kind == "route" {
		file, other = other, file
	}
	abs := filepath.Join(root, filepath.FromSlash(dir))
	if _, err := os.Stat(filepath.Join(abs, other)); err == nil {
		return GenResult{}, fmt.Errorf("%s/%s: %w", dir, other, ErrGenConflict)
	}
	pkg, err := packageFor(abs, dirs[len(dirs)-1])
	if err != nil {
		return GenResult{}, err
	}
	pattern := "/" + strings.Join(patternSegs, "/")
	title := "home"
	if n := len(dirs); n > 1 {
		title = strings.TrimRight(dirs[n-1], "_-")
	}
	data := map[string]any{
		"Package": pkg,
		"Pattern": pattern,
		"Title":   title,
		"Params":  params,
	}
	rel := path.Join(dir, file)
	res := GenResult{File: rel, Pattern: pattern, Package: pkg}
	// A layout is written before the page: a page that answers without the
	// layout it was asked for is the surprise this flag exists to avoid.
	if o.Layout != "" {
		extra, err := ensureLayout(root, o, dir)
		if err != nil {
			return GenResult{}, err
		}
		res.Extra = extra
	}
	switch {
	case o.Kind == "route" && (len(o.Methods) > 0 || o.Bind != ""):
		if err := writeRouteContract(root, o, dir, rel, pkg, pattern, params); err != nil {
			return GenResult{}, err
		}
		return res, nil
	case o.Kind == "page" && o.Form != "":
		if err := writePageForm(root, o, dir, rel, pkg, title); err != nil {
			return GenResult{}, err
		}
		return res, nil
	}
	tmpl := pageTemplate
	if o.Kind == "route" {
		tmpl = routeTemplate
	}
	if err := writeGo(root, rel, tmpl, data, o.Force); err != nil {
		return GenResult{}, err
	}
	return res, nil
}

// writeRouteContract writes route.go with one handler per method and, when a
// type was named, the bind that turns the tags into a 422.
func writeRouteContract(root string, o GenOptions, dir, rel, pkg, pattern string, params []string) error {
	t, err := o.texts()
	if err != nil {
		return err
	}
	ms, err := normalizeMethods(o.Methods)
	if err != nil {
		return err
	}
	if len(ms) == 0 {
		ms = []string{"GET"}
	}
	var bound *boundType
	imps := []string{"github.com/emersonjoe/trilha"}
	data := routeContract{Package: pkg, Pattern: pattern}
	if o.Bind != "" {
		b, err := resolveBound(root, o.Module, dir, o.Bind, "json", t["gen_input_route"])
		if err != nil {
			return err
		}
		bound = &b
		data.Decl = strings.TrimRight(b.Decl, "\n")
		imps = append(imps, b.Import)
	}
	data.Imports = imports(imps...)
	for _, m := range ms {
		data.Handlers = append(data.Handlers, handler{Method: m, Body: handlerBody(m, params, bound, t)})
	}
	return writeGo(root, rel, routeContractTemplate, data, o.Force)
}

// writePageForm writes page.go with the whole round trip of a form.
func writePageForm(root string, o GenOptions, dir, rel, pkg, title string) error {
	t, err := o.texts()
	if err != nil {
		return err
	}
	b, err := resolveBound(root, o.Module, dir, o.Form, "form", t["gen_input_form"])
	if err != nil {
		return err
	}
	fields, needsHelper := formFields(b, t)
	imps := []string{"net/http", "github.com/emersonjoe/trilha", "github.com/emersonjoe/trilha/h", "github.com/emersonjoe/trilha/ui", b.Import}
	if strings.Contains(fields, "fmt.Sprint(") {
		imps = append(imps, "fmt")
	}
	data := pageForm{
		Package: pkg,
		Imports: imports(imps...),
		Decl:    strings.TrimRight(b.Decl, "\n"),
		Type:    b.Ref,
		Title:   title,
		Fields:  fields,
		PageDoc: t["gen_page_form"],
		PostDoc: t["gen_post_form"],
		FormDoc: t["gen_redirect"],
		Submit:  t["gen_submit"],
	}
	if needsHelper {
		data.Helper = checkedHelper
	}
	return writeGo(root, rel, pageFormTemplate, data, o.Force)
}

// ensureLayout writes the layout the page was asked to sit under, when the
// folder has none. An existing layout is never touched: --force is about the
// file the command came to write.
func ensureLayout(root string, o GenOptions, pageDir string) ([]string, error) {
	t, err := o.texts()
	if err != nil {
		return nil, err
	}
	rel, err := layoutPath(o.Layout, pageDir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		return nil, nil
	}
	abs := filepath.Join(root, filepath.FromSlash(path.Dir(rel)))
	pkg, err := packageFor(abs, path.Base(path.Dir(rel)))
	if err != nil {
		return nil, err
	}
	if err := writeGo(root, rel, layoutTemplate, map[string]any{"Package": pkg, "Doc": t["gen_layout"]}, false); err != nil {
		return nil, err
	}
	return []string{rel}, nil
}

func generateComponent(root string, o GenOptions) (GenResult, error) {
	name := strings.TrimSpace(o.Arg)
	if !token.IsIdentifier(name) {
		return GenResult{}, fmt.Errorf("%q: a component name has to be a Go identifier", name)
	}
	if !ast.IsExported(name) {
		return GenResult{}, fmt.Errorf("%q: a component is called from outside its package, so it starts with a capital", name)
	}
	dir := o.Dir
	if dir == "" {
		dir = DefaultComponentDir
	}
	dir = path.Clean(filepath.ToSlash(dir))
	if dir == "." || strings.HasPrefix(dir, "..") || filepath.IsAbs(dir) {
		return GenResult{}, fmt.Errorf("%q: give a folder inside the project", o.Dir)
	}
	abs := filepath.Join(root, filepath.FromSlash(dir))
	pkg, err := packageFor(abs, path.Base(dir))
	if err != nil {
		return GenResult{}, err
	}
	rel := path.Join(dir, snakeCase(name)+".go")
	data := map[string]any{"Package": pkg, "Name": name, "Class": strings.ReplaceAll(snakeCase(name), "_", "-")}
	if err := writeGo(root, rel, componentTemplate, data, o.Force); err != nil {
		return GenResult{}, err
	}
	return GenResult{File: rel, Package: pkg}, nil
}

// packageFor answers with the package already declared in the folder, when
// there is one: Go refuses a directory whose files disagree, and the folder
// name is only a guess about what was chosen there.
func packageFor(abs, dirName string) (string, error) {
	entries, err := os.ReadDir(abs)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			names = append(names, n)
		}
	}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(abs, n))
		if err != nil {
			return "", err
		}
		if pkg := packageClause(string(b)); pkg != "" {
			return pkg, nil
		}
	}
	return packageName(dirName), nil
}

func packageClause(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "package ")
		if !ok {
			continue
		}
		if i := strings.Index(rest, "//"); i >= 0 {
			rest = rest[:i]
		}
		if name := strings.TrimSpace(rest); token.IsIdentifier(name) {
			return name
		}
	}
	return ""
}

// packageName derives a package name from a folder name: the conventions put
// suffixes and dots in folders that an identifier cannot carry.
func packageName(dir string) string {
	dir = strings.TrimSuffix(strings.TrimSuffix(dir, "-"), "__")
	dir = strings.TrimSuffix(dir, "_")
	var sb strings.Builder
	for _, r := range strings.ToLower(dir) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(r)
		}
	}
	name := sb.String()
	switch {
	case name == "":
		return "app"
	case unicode.IsDigit(rune(name[0])):
		name = "p" + name
	}
	if token.IsKeyword(name) {
		name += "_"
	}
	return name
}

// snakeCase turns AvisoGrande into aviso_grande, the file naming the framework
// itself uses (not_found.go).
func snakeCase(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(rune(s[i-1])) {
			sb.WriteByte('_')
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	return sb.String()
}

func writeGo(root, rel, tmpl string, data any, force bool) error {
	dst := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(dst); err == nil && !force {
		return fmt.Errorf("%s: %w", rel, ErrGenExists)
	}
	t, err := template.New(rel).Parse(tmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, src, 0o644)
}

const pageTemplate = `package {{.Package}}

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders GET {{.Pattern}}.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("{{.Title}}")
	return h.Div(
		h.H1(h.Text("{{.Title}}")),
{{- range .Params}}
		h.P(h.Text(c.Param("{{.}}"))),
{{- end}}
	), nil
}
`

const routeTemplate = `package {{.Package}}

import "github.com/emersonjoe/trilha"

// GET {{.Pattern}}
func GET(c *trilha.Ctx) error {
	return c.JSON(200, map[string]any{
{{- range .Params}}
		"{{.}}": c.Param("{{.}}"),
{{- end}}
		"ok": true,
	})
}
`

const componentTemplate = `package {{.Package}}

import "github.com/emersonjoe/trilha/h"

// {{.Name}} returns a node, so it composes like any other.
func {{.Name}}(children ...h.Node) h.Node {
	return h.Div(h.Class("{{.Class}}"), h.Group(children...))
}
`
