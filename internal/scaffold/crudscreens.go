package scaffold

import (
	"fmt"
	"strings"
)

// list is the screen every management app has: filter, sortable table,
// pagination — all of it in the URL, so a link to a filtered page is a link
// that works.
func (p crudPlan) list() string {
	var cols strings.Builder
	for _, f := range p.Columns {
		fmt.Fprintf(&cols, "\t{Key: %q, Label: %q, Sort: true%s, Cell: func(v %s) h.Node {\n\t\treturn %s\n\t}},\n",
			f.Form, labelOf(f), numeric(f), p.Ref, cellNode(f))
	}
	// The dates the store stamps are read-only and belong on the listing: a
	// field that is in the struct and on no screen is a field somebody looks
	// for and does not find.
	for _, f := range p.System {
		if strings.TrimPrefix(f.Type, "*") != "time.Time" {
			continue
		}
		fmt.Fprintf(&cols, "\t{Key: %q, Label: %q, Sort: true, Cell: func(v %s) h.Node {\n\t\treturn ui.Date(c, v.%s)\n\t}},\n",
			f.Form, labelOf(f), p.Ref, f.Name)
	}
	// The delete button belongs in the table, beside the row it deletes. A
	// POST handler with no button is a handler nobody can reach, and a delete
	// with no confirmation is the one mistake a listing cannot undo.
	fmt.Fprintf(&cols, "\t{Key: \"\", Label: \"\", Cell: func(v %s) h.Node {\n\t\treturn excluir(v)\n\t}},\n", p.Ref)
	// The imports come from what the columns ended up needing: a number in a
	// cell reaches the table through fmt, and a struct of only strings must
	// not import it.
	extra := ""
	if strings.Contains(cols.String(), "fmt.Sprint") {
		extra = "\t\"fmt\"\n"
	}
	return fmt.Sprintf(`// Package %s is the listing of %s: filter, sortable table and pagination,
// all of it in the address.
package %s

import (
%s	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"

	"%s"
)

// columns declares what the table shows and, with Sort, what the store may be
// ordered by: a column that is not here is dropped before the query, so a
// crooked address answers unordered instead of reaching the data.
//
// It takes the request because a date belongs to whoever is reading it: the
// time zone and the language of the app decide how it is written, and both
// live on the Ctx.
func columns(c *trilha.Ctx) ui.Columns[%s] {
	return ui.Columns[%s]{
%s	}
}

// Page answers GET %s: the whole screen, or only the table when the script
// asks for that fragment.
func Page(c *trilha.Ctx) (h.Node, error) {
	var q trilha.ListParams
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	table := list(c, q)
	if c.Fragment() == %q {
		return table, nil
	}
	c.SetTitle(%q)
	return ui.Stack(
		ui.PageHeader(%q, ui.ButtonLink(%q, h.Text(%q))),
		table,
	), nil
}

// POST deletes the row the form names and lands back on the listing, which is
// what makes reloading the page harmless.
func POST(c *trilha.Ctx) error {
	if !trilha.Use[%s.%sStore](c).Delete(c.Form("id")) {
		return trilha.Errorf(http.StatusNotFound, "%%s", %q)
	}
	c.Flash(ui.FlashSuccess, %q)
	return c.Redirect(%q)
}

// excluir is the button on the row. The confirmation is the kit's, and it is
// not optional: a listing that deletes on one click is a listing somebody
// deletes from by accident.
func excluir(v %s) h.Node {
	return h.Form(h.Method("post"), h.Action(%q), h.Class("ui-inline-form"),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(v.ID)),
		ui.Confirm(%q, %q),
		ui.Button(ui.Ghost(), ui.Destructive(), h.Type("submit"), h.Text(%q)),
	)
}

func list(c *trilha.Ctx, q trilha.ListParams) h.Node {
	rows, total := trilha.Use[%s.%sStore](c).List(%s.%sQuery{
		Q: q.Q, Sort: q.Sort, Asc: q.Asc(), Offset: q.Offset(), Limit: q.Limit(),
	})
	return ui.DataTable(c, columns(c), rows, ui.ListState{
		Params:  q,
		Total:   total,
		ID:      %q,
		Search:  %q,
		Caption: %q,
		RowHref: func(i int) string { return %q + "/" + rows[i].ID },
		Empty:   ui.Muted(h.Text(%q)),
	})
}
%s`, p.ListPkg, p.Title, p.ListPkg, extra, p.Import, p.Ref, p.Ref, cols.String(), p.URL, p.ListPkg,
		p.Title, p.Title, p.URL+"/new", p.T["crud_new"]+" "+strings.ToLower(p.One),
		p.Pkg, p.Type, p.T["not_found"], p.T["app_deleted"], p.URL,
		p.Ref, p.URL, p.T["crud_delete"]+"?", p.T["crud_no_undo"], p.T["crud_delete"],
		p.Pkg, p.Type, p.Pkg, p.Type,
		p.ListPkg, p.T["app_search"], p.Title, p.URL, p.T["app_empty"], p.simHelper(cols.String()))
}

// simHelper is the yes/no of a boolean column, in the app's language. It is
// written into the file only when a column needs it — a helper nobody calls is
// a compile error, and the generated project has to be green without an edit.
func (p crudPlan) simHelper(cols string) string {
	if !strings.Contains(cols, "sim(v.") {
		return ""
	}
	return fmt.Sprintf(`
// sim writes a boolean the way somebody reads it. "true" on a screen is the
// name of a variable, not an answer.
func sim(on bool) h.Node {
	if on {
		return ui.Badge(h.Text(%q))
	}
	return ui.Badge(ui.Outline(), h.Text(%q))
}
`, p.T["crud_yes"], p.T["crud_no"])
}

// form is both writing screens: /new creates, /{id} edits. They are one
// function because they are one form — the difference is where it posts and
// whether it starts filled — and two copies of a form is where the two screens
// start disagreeing about which fields are required.
func (p crudPlan) form(novo bool) string {
	pkg, dir := "new"+strings.ToLower(p.ListPkg), "new"
	titulo := p.T["crud_new"] + " " + strings.ToLower(p.One)
	if !novo {
		pkg, dir = "id", "id_"
		titulo = p.T["crud_edit"] + " " + strings.ToLower(p.One)
	}
	_ = dir
	campos, precisaHelper := formFields(boundType{Fields: p.Fields}, p.T)
	helper := ""
	if precisaHelper {
		helper = checkedHelper
	}
	imports := p.formImports(campos, novo)

	corpo := p.formBody(novo, titulo)
	return fmt.Sprintf(`// Package %s is the form that %s a %s.
package %s

%s
%s
func form(c *trilha.Ctx, in %s, errs trilha.FieldErrors, action, titulo string) h.Node {
	c.SetTitle(titulo)
	return ui.Card(
		ui.CardHeader(ui.CardTitle(titulo)),
		ui.CardContent(h.Form(h.Method("post"), h.Action(action), h.Class("ui-stack"),
			trilha.CSRFInput(c),
%s			h.Div(ui.Submit(h.Text(%q))),
		)),
	)
}
%s`, pkg, map[bool]string{true: "creates", false: "edits"}[novo], strings.ToLower(p.One),
		pkg, imports, corpo, p.Ref, campos, p.T["crud_save"], helper)
}

// formBody is the pair of handlers. The new screen posts to itself; the edit
// screen reads the row first and 404s when it is gone, which is the difference
// between an empty form and a lie.
func (p crudPlan) formBody(novo bool, titulo string) string {
	if novo {
		return fmt.Sprintf(`
// Page renders GET %s/new.
func Page(c *trilha.Ctx) (h.Node, error) {
	return form(c, %s{}, nil, %q, %q), nil
}

// POST creates it: 422 with the messages next to the fields, or
// POST -> redirect -> GET, so reloading does not send the form again.
func POST(c *trilha.Ctx) error {
	var in %s
	if err := c.Bind(&in); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, form(c, in, errs, %q, %q))
	}
	if _, err := trilha.Use[%s.%sStore](c).Create(in); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, %q)
	return c.Redirect(%q)
}
`, p.URL, p.Ref, p.URL+"/new", titulo, p.Ref, p.URL+"/new", titulo,
			p.Pkg, p.Type, p.T["app_saved"], p.URL)
	}
	return fmt.Sprintf(`
// Page renders GET %s/{id} with the row already in the fields.
func Page(c *trilha.Ctx) (h.Node, error) {
	v, ok := trilha.Use[%s.%sStore](c).Get(c.Param("id"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	return form(c, v, nil, c.Request().URL.Path, %q), nil
}

// POST saves the change. The key comes from the address and never from the
// form: an id the browser sends is an id the browser can change.
func POST(c *trilha.Ctx) error {
	id := c.Param("id")
	if _, ok := trilha.Use[%s.%sStore](c).Get(id); !ok {
		return trilha.ErrNotFound
	}
	var in %s
	if err := c.Bind(&in); err != nil {
		errs, ok := err.(trilha.FieldErrors)
		if !ok {
			return err
		}
		return c.Render(http.StatusUnprocessableEntity, form(c, in, errs, c.Request().URL.Path, %q))
	}
	if _, err := trilha.Use[%s.%sStore](c).Update(id, in); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, %q)
	return c.Redirect(%q)
}
`, p.URL, p.Pkg, p.Type, titulo, p.Pkg, p.Type, p.Ref, titulo,
		p.Pkg, p.Type, p.T["app_saved"], p.URL)
}

// formImports lists what the file ended up using. Deciding from the body is
// the only way this stays right for every shape of struct.
func (p crudPlan) formImports(campos string, novo bool) string {
	std := []string{"net/http"}
	if strings.Contains(campos, "fmt.Sprint") {
		std = append(std, "fmt")
	}
	var sb strings.Builder
	sb.WriteString("import (\n")
	for _, s := range sorted(std) {
		fmt.Fprintf(&sb, "\t%q\n", s)
	}
	sb.WriteString("\n\t\"github.com/emersonjoe/trilha\"\n")
	sb.WriteString("\t\"github.com/emersonjoe/trilha/h\"\n")
	sb.WriteString("\t\"github.com/emersonjoe/trilha/ui\"\n")
	fmt.Fprintf(&sb, "\n\t%q\n)\n", p.Import)
	return sb.String()
}

// test is the one that proves the screens agree with each other: an empty
// list, a creation that shows up in it, an edit that sticks, a 422 that comes
// back with the message in the field, and a delete that removes the row.
//
// It is generated because the CRUD is generated: a skeleton whose test
// somebody has to write first is a skeleton nobody trusts.
func (p crudPlan) test() string {
	// Every field the form asks for, filled with a value its own rules
	// accept: a test that fills one field and expects a 303 is a test that
	// fails on the second required field, and the failure is about the test.
	criar, _, procura := p.exampleForm("")
	_, editar, procura2 := p.exampleForm("-2")
	return fmt.Sprintf(`package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
%s)

// TestCRUD%s walks the whole thing: the empty listing, a creation that shows
// up in it, an edit that sticks, and a delete that removes it.
//
// It lives at the root of the project because that is where newApp() is: the
// screens are three packages, and what is worth testing is that they agree
// with each other.
func TestCRUD%s(t *testing.T) {
	t.Setenv("TRILHA_ENV", "prod")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
%s%s	c := trilha.NewTestClient(t, newApp())
%s
	c.Get(%q).WantStatus(200).WantContains(%q)

	c.PostForm(%q, url.Values{
%s	}).WantStatus(http.StatusSeeOther)
	corpo := c.Get(%q).WantStatus(200).Body.String()
	if !strings.Contains(corpo, %q) {
		t.Fatalf("o que foi criado não apareceu na lista:\n%%s", corpo)
	}

	// O id vem do botão de excluir da linha, que é o único lugar da página
	// onde ele aparece sozinho — o link "novo" também começa com o endereço da
	// lista, e procurar por ele acharia "new".
	id := depoisDe(corpo, "name=\"id\" value=\"")
	if id == "" {
		t.Fatalf("a lista não trouxe o botão de excluir da linha:\n%%s", corpo)
	}
	c.Get(%q + "/" + id).WantStatus(200).WantContains(%q)

	c.PostForm(%q+"/"+id, url.Values{
%s	}).WantStatus(http.StatusSeeOther)
	c.Get(%q).WantStatus(200).WantContains(%q)

	// Um campo obrigatório vazio volta 422 com a mensagem no campo, e não
	// grava nada.
	c.PostForm(%q, url.Values{%q: {""}}).WantStatus(http.StatusUnprocessableEntity)

	c.PostForm(%q, url.Values{"id": {id}}).WantStatus(http.StatusSeeOther)
	if depois := c.Get(%q).Body.String(); strings.Contains(depois, %q) {
		t.Fatalf("o que foi excluído continua na lista:\n%%s", depois)
	}
}

// depoisDe devolve o que vem logo depois do prefixo, até a próxima aspa: é
// como o teste acha o id sem depender do formato da tabela.
func depoisDe(corpo, prefixo string) string {
	i := strings.Index(corpo, prefixo)
	if i < 0 {
		return ""
	}
	resto := corpo[i+len(prefixo):]
	fim := strings.IndexAny(resto, "\"'>")
	if fim <= 0 {
		return ""
	}
	return resto[:fim]
}
`, p.testImport(), p.Type, p.Type, p.testSkip(), p.Env, p.testLogin(),
		p.URL, p.T["app_empty"],
		p.URL+"/new", criar,
		p.URL, procura,
		p.URL, procura,
		p.URL, editar,
		p.URL, procura2,
		p.URL+"/new", p.Fields[0].Form,
		p.URL,
		p.URL, procura2)
}

// exampleForm is the body of a form that passes: every field with a value its
// own validate rules accept. It answers the url.Values lines, and the value of
// the first text field, which is what the test looks for in the listing.
func (p crudPlan) exampleForm(sufixo string) (linhas, editadas, procura string) {
	var sb, sb2 strings.Builder
	for _, f := range p.Fields {
		v, ok := exampleValue(f)
		if !ok {
			continue
		}
		s := fmt.Sprint(v)
		if f.Type == "bool" {
			s = "on"
		}
		alt := s
		if procura == "" && strings.TrimPrefix(f.Type, "*") == "string" {
			procura = s + sufixo
			alt = s + sufixo
		}
		fmt.Fprintf(&sb, "\t\t%q: {%q},\n", f.Form, s)
		fmt.Fprintf(&sb2, "\t\t%q: {%q},\n", f.Form, alt)
	}
	if procura == "" {
		procura = p.Title
	}
	return sb.String(), sb2.String(), procura
}

// numeric marks the columns a table right-aligns: a number in a left-aligned
// column is a number nobody can compare down the page.
func numeric(f typeField) string {
	t := strings.TrimPrefix(f.Type, "*")
	if strings.HasPrefix(t, "int") || strings.HasPrefix(t, "uint") || strings.HasPrefix(t, "float") {
		return ", Num: true"
	}
	return ""
}

// cellExpr is how one value reaches the table as text.
// cellNode is the node a cell renders. It is a node and not a string because
// what a person reads is not always the value: a bool written "true" is the
// name of a variable, and a date is written the way the reader's language and
// zone write it.
func cellNode(f typeField) string {
	t := strings.TrimPrefix(f.Type, "*")
	switch {
	case t == "bool":
		return "sim(v." + f.Name + ")"
	case t == "time.Time":
		return "ui.Date(c, v." + f.Name + ")"
	}
	return "h.Text(" + cellExpr(f) + ")"
}

// cellExpr is the value of a cell as a string.
func cellExpr(f typeField) string {
	t := strings.TrimPrefix(f.Type, "*")
	switch {
	case t == "string":
		return "v." + f.Name
	case strings.HasPrefix(t, "int"), strings.HasPrefix(t, "uint"), strings.HasPrefix(t, "float"):
		return "fmt.Sprint(v." + f.Name + ")"
	}
	return "fmt.Sprint(v." + f.Name + ")"
}

// testImport is the helper's import line, when the project has one.
func (p crudPlan) testImport() string {
	if p.Helper == "" {
		return ""
	}
	return fmt.Sprintf("\n\t%q\n", p.Helper)
}

// testSkip is what a generated test says when the folder is closed and there
// is no known way to open a session: it names the file that closes it. A Skip
// that explains is better than a 401 that does not — a test failing for a
// reason outside itself teaches the wrong lesson.
func (p crudPlan) testSkip() string {
	if p.Guard == "" || p.Helper != "" {
		return ""
	}
	razao := p.Guard + ": " + p.T["crud_guard_skip"]
	return fmt.Sprintf("\t// %s\n\tt.Skip(%q)\n", razao, razao)
}

// testLogin opens the session before the first request, when the project has
// a helper that knows how.
func (p crudPlan) testLogin() string {
	if p.Call == "" {
		return ""
	}
	return fmt.Sprintf("\t// %s\n\t%s\n", p.T["crud_guard_login"], p.Call)
}
