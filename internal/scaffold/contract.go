package scaffold

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

// A skeleton with a contract is a second template, not a branch inside the
// first: without flags, generate keeps writing byte for byte what it wrote in
// 0.27.0, and that promise is easier to keep with the old template untouched.

// boundType is the type a handler binds the body to, seen from the file being
// written: the name to write in the code, the import it needs, and the fields
// a form has to render.
type boundType struct {
	Ref    string      // Comment, or posts.Comment
	Decl   string      // source of the struct to declare, empty when imported
	Import string      // import path, empty when it is in the same package
	Fields []typeField // what it carries
}

// resolveBound finds the type in the project or invents it. tag is "json" for
// a route and "form" for a page: the same struct read by different doors.
func resolveBound(root, module, dir, name, tag, doc string) (boundType, error) {
	info, err := findType(root, module, name)
	switch {
	case errors.Is(err, errTypeNotFound):
		bare := name[strings.LastIndex(name, ".")+1:]
		fields := exampleFields(tag)
		return boundType{Ref: bare, Decl: structSource(bare, fields, tag, doc), Fields: fields}, nil
	case err != nil:
		return boundType{}, err
	}
	if info.Dir == dir {
		return boundType{Ref: info.Name, Fields: info.Fields}, nil
	}
	if info.Import == "" {
		return boundType{}, fmt.Errorf("%s is in %s, and importing it needs the module path from go.mod", info.Name, info.Dir)
	}
	return boundType{Ref: info.Pkg + "." + info.Name, Import: info.Import, Fields: info.Fields}, nil
}

// exampleFields is the struct a type that does not exist yet is born with: two
// fields, one required with a size and one free, which is enough for the
// validation to be visible and short enough to be replaced.
func exampleFields(tag string) []typeField {
	if tag == "form" {
		return []typeField{
			{Name: "Name", Type: "string", JSON: "name", Form: "name", Validate: "required,min=3,max=80"},
			{Name: "Email", Type: "string", JSON: "email", Form: "email", Validate: "required,email"},
		}
	}
	return []typeField{
		{Name: "Title", Type: "string", JSON: "title", Form: "title", Validate: "required,max=80"},
		{Name: "Body", Type: "string", JSON: "body", Form: "body", Validate: "required"},
	}
}

// structSource writes the declaration. The tags carry backquotes, which a Go
// raw literal cannot, so this is built here instead of inside a template.
func structSource(name string, fields []typeField, tag, doc string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "// %s %s\n", name, doc)
	fmt.Fprintf(&sb, "type %s struct {\n", name)
	for _, f := range fields {
		key := f.JSON
		if tag == "form" {
			key = f.Form
		}
		fmt.Fprintf(&sb, "\t%s %s `%s:%q validate:%q`\n", f.Name, f.Type, tag, key, f.Validate)
	}
	sb.WriteString("}\n")
	return sb.String()
}

// imports writes the import block: standard library first, the rest after a
// blank line, each group sorted — the shape gofmt keeps.
func imports(paths ...string) string {
	var std, ext []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if strings.Contains(strings.SplitN(p, "/", 2)[0], ".") {
			ext = append(ext, p)
		} else {
			std = append(std, p)
		}
	}
	sort.Strings(std)
	sort.Strings(ext)
	if len(std) == 0 && len(ext) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("import (\n")
	for _, p := range std {
		fmt.Fprintf(&sb, "\t%q\n", p)
	}
	if len(std) > 0 && len(ext) > 0 {
		sb.WriteString("\n")
	}
	for _, p := range ext {
		fmt.Fprintf(&sb, "\t%q\n", p)
	}
	sb.WriteString(")\n")
	return sb.String()
}

// hasBody reports whether the method carries one, which is the only reason a
// handler binds anything.
func hasBody(m string) bool { return m == "POST" || m == "PUT" || m == "PATCH" }

// successStatus is what the skeleton answers: a POST creates, the rest replace.
func successStatus(m string) string {
	if m == "POST" {
		return "201"
	}
	return "200"
}

// handlerBody writes what goes inside one handler.
func handlerBody(m string, params []string, b *boundType, t map[string]string) string {
	var sb strings.Builder
	if b != nil && hasBody(m) {
		fmt.Fprintf(&sb, "\tvar in %s\n", b.Ref)
		fmt.Fprintf(&sb, "\t// %s\n", t["gen_bind"])
		sb.WriteString("\tif err := c.BindJSON(&in); err != nil {\n\t\treturn err\n\t}\n")
		fmt.Fprintf(&sb, "\treturn c.JSON(%s, in)\n", successStatus(m))
		return sb.String()
	}
	sb.WriteString("\treturn c.JSON(200, map[string]any{\n")
	for _, p := range params {
		fmt.Fprintf(&sb, "\t\t%q: c.Param(%q),\n", p, p)
	}
	sb.WriteString("\t\t\"ok\": true,\n\t})\n")
	return sb.String()
}

// routeContract is the data of the template below.
type routeContract struct {
	Package  string
	Imports  string
	Decl     string
	Pattern  string
	Handlers []handler
}

type handler struct {
	Method, Body string
}

const routeContractTemplate = `package {{.Package}}

{{.Imports}}
{{with .Decl}}
{{.}}
{{end}}{{range .Handlers}}
// {{.Method}} {{$.Pattern}}
func {{.Method}}(c *trilha.Ctx) error {
{{.Body}}}
{{end}}`

// pageForm is the data of the form page template.
type pageForm struct {
	Package string
	Imports string
	Decl    string
	Type    string
	Pattern string
	Title   string
	Fields  string // the ui.Field calls, already written
	Helper  string // checked(), when a bool field needs it
	PageDoc string
	PostDoc string
	FormDoc string
	Submit  string
}

const pageFormTemplate = `package {{.Package}}

{{.Imports}}
{{with .Decl}}
{{.}}
{{end}}
// Page {{.PageDoc}}
func Page(c *trilha.Ctx) (h.Node, error) {
	return form(c, {{.Type}}{}, nil), nil
}

// POST {{.PostDoc}}
func POST(c *trilha.Ctx) error {
	var in {{.Type}}
	if err := c.Bind(&in); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, form(c, in, errs))
	}
	// {{.FormDoc}}
	return c.Redirect(c.Request().URL.Path)
}

func form(c *trilha.Ctx, in {{.Type}}, errs trilha.FieldErrors) h.Node {
	c.SetTitle("{{.Title}}")
	return ui.Card(
		ui.CardHeader(ui.CardTitle("{{.Title}}")),
		ui.CardContent(h.Form(h.Method("post"), h.Action(c.Request().URL.Path), h.Class("ui-stack"),
			trilha.CSRFInput(c),
{{.Fields}}			h.Div(ui.Submit(h.Text("{{.Submit}}"))),
		)),
	)
}
{{.Helper}}`

const layoutTemplate = `package {{.Package}}

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Layout {{.Doc}}
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Div(children), nil
}
`

// formFields writes one ui.Field per field of the type. A field whose control
// the kit has no obvious answer for is left with a line saying so: a wrong
// control is worse than an absent one, because it silently drops what the user
// typed.
func formFields(b boundType, t map[string]string) (string, bool) {
	var sb strings.Builder
	needsHelper := false
	for _, f := range b.Fields {
		name := f.Form
		label := label(f.Name)
		switch value, kind := formControl(f); kind {
		case "text":
			fmt.Fprintf(&sb, "\t\t\tui.Field(%q, %q,\n\t\t\t\tui.Input(h.ID(%q), h.Name(%q), h.Value(%s), ui.InvalidIf(errs, %q)),\n\t\t\t\tui.Errors(errs, %q)),\n",
				name, label, name, name, value, name, name)
		case "number":
			fmt.Fprintf(&sb, "\t\t\tui.Field(%q, %q,\n\t\t\t\tui.Input(h.ID(%q), h.Name(%q), h.Type(\"number\"), h.Value(%s), ui.InvalidIf(errs, %q)),\n\t\t\t\tui.Errors(errs, %q)),\n",
				name, label, name, name, value, name, name)
		case "check":
			needsHelper = true
			fmt.Fprintf(&sb, "\t\t\tui.CheckRow(ui.Checkbox(h.ID(%q), h.Name(%q), checked(%s)), %q, %q),\n",
				name, name, value, label, name)
		default:
			fmt.Fprintf(&sb, "\t\t\t// %s: %s\n", f.Name, t["gen_no_control"])
		}
	}
	return sb.String(), needsHelper
}

// formControl decides the control and how the current value reaches it.
func formControl(f typeField) (value, kind string) {
	t := strings.TrimPrefix(f.Type, "*")
	switch {
	case t == "string":
		return "in." + f.Name, "text"
	case t == "bool":
		return "in." + f.Name, "check"
	case strings.HasPrefix(t, "int") || strings.HasPrefix(t, "uint") || strings.HasPrefix(t, "float"):
		return "fmt.Sprint(in." + f.Name + ")", "number"
	}
	return "", ""
}

// label turns a field name into what the person filling the form reads.
func label(name string) string {
	var sb strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' && name[i-1] >= 'a' && name[i-1] <= 'z' {
			sb.WriteByte(' ')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

const checkedHelper = `
// checked marks the box when the value is on.
func checked(on bool) h.Node {
	if on {
		return h.Checked()
	}
	return h.Nil
}
`

// layoutPath validates --layout: it has to be a layout.go in a folder that
// wraps the page, inside app/. Anywhere else the scanner would never apply it,
// and finding that out costs a round trip.
func layoutPath(raw, pageDir string) (string, error) {
	p := path.Clean(strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/"))
	if path.Base(p) != "layout.go" {
		return "", fmt.Errorf("%s: a layout is a file called layout.go", raw)
	}
	dir := path.Dir(p)
	if dir != "app" && !strings.HasPrefix(dir, "app/") {
		return "", fmt.Errorf("%s: a layout lives inside app/", raw)
	}
	if pageDir != dir && !strings.HasPrefix(pageDir+"/", dir+"/") {
		return "", fmt.Errorf("%s: a layout only wraps the pages under it, and %s is not under %s", raw, pageDir, dir)
	}
	return p, nil
}
