package ui

import "github.com/emersonjoe/trilha/h"

// Policy is what PolicyGrid needs to draw a permission matrix: the columns, the
// values a cell may hold, the rows, and what is in each cell.
//
// It is an interface and not auth.Policy because the kit must not drag
// authentication into an application that only draws buttons. auth.Policy
// satisfies it as it is.
type Policy interface {
	ModuleNames() []string
	LevelNames() []string
	RolesSorted() []string
	LevelOf(role, module string) string
}

// ScopedPolicy is a Policy whose cells also say how far a grant reaches: the
// whole organisation, the person's own unit, or that unit and everything under
// it. A policy that satisfies it gets a second select per cell; one that does
// not is drawn exactly as before.
//
// auth.Policy satisfies it with ScopeNameOf and ScopeNames.
type ScopedPolicy interface {
	Policy
	ScopeNameOf(role, module string) string
	ScopeNames() []string
}

// PolicyGridOpts configures the grid.
type PolicyGridOpts struct {
	// Action is where the form posts. Required.
	Action string
	// Labels renames a module for the screen: the code says "docs", the person
	// reading says "Documents". A module with no label shows its own name.
	Labels map[string]string
	// None is what the empty option says. Default: "no access".
	None string
	// Submit is the button's text. Default: "Save".
	Submit string
	// ReadOnly draws the same grid with every control disabled, which is what
	// somebody who may see the matrix but not change it should get.
	ReadOnly bool
	// ScopeLabels renames a scope for the screen, by the value the policy
	// gives: "" is the organisation, "unit" the person's own unit, "tree" that
	// unit and everything under it. Defaults are in English, and a scope with
	// no label shows its own name.
	ScopeLabels map[string]string
	// CSRF is the hidden token field, and a form that changes permissions has
	// to carry one: pass trilha.CSRFInput(c). It is an option and not a Ctx
	// argument so that the grid can be rendered — read-only, in a test, in a
	// preview — without one.
	CSRF h.Node
}

// PolicyGrid renders the permission matrix as a form: one row per role, one
// column per module, a select of levels in each cell.
//
//	ui.PolicyGrid(acesso.Policy, ui.PolicyGridOpts{Action: "/admin/permissoes"})
//
// The fields are named grant.<role>.<module>, which is what auth.BindPolicy
// reads on the other side. There is no JavaScript: it is a form, it posts, the
// handler saves and redirects.
//
// Whoever draws this screen has to guard it — a grid that edits the matrix is
// the most valuable screen in the application, and it must sit behind the
// module that administers it.
func PolicyGrid(p Policy, opts PolicyGridOpts) h.Node {
	if p == nil {
		return h.Fragment()
	}
	none, submit := opts.None, opts.Submit
	if none == "" {
		none = "no access"
	}
	if submit == "" {
		submit = "Save"
	}
	modules, levels, roles := p.ModuleNames(), p.LevelNames(), p.RolesSorted()
	scoped, _ := p.(ScopedPolicy)

	head := []h.Node{h.Th(h.Text("Role"))}
	for _, m := range modules {
		label := m
		if l, ok := opts.Labels[m]; ok && l != "" {
			label = l
		}
		head = append(head, h.Th(h.Text(label)))
	}

	rows := make([]h.Node, 0, len(roles))
	for _, role := range roles {
		cells := []h.Node{h.Td(h.Strong(h.Text(role)))}
		for _, m := range modules {
			id := "grant." + role + "." + m
			options := []Option{{Value: "", Label: none}}
			for _, l := range levels {
				options = append(options, Option{Value: l, Label: l})
			}
			attrs := []h.Node{h.ID(id), h.Name(id),
				h.Aria("label", role+" — "+m),
				SelectOptions(options, p.LevelOf(role, m))}
			if opts.ReadOnly {
				attrs = append(attrs, h.Attr("disabled", ""))
			}
			cell := []h.Node{Select(attrs...)}
			if scoped != nil {
				cell = append(cell, scopeSelect(scoped, opts, role, m))
			}
			cells = append(cells, h.Td(h.Class("ui-policy-cell"), h.Fragment(cell...)))
		}
		rows = append(rows, h.Tr(cells...))
	}

	grid := Table(h.Thead(h.Tr(head...)), h.Tbody(h.Fragment(rows...)))
	if opts.ReadOnly {
		return grid
	}
	return h.Form(h.Method("post"), h.Action(opts.Action), h.Class("ui-stack"),
		opts.CSRF,
		grid,
		h.Div(Submit(h.Text(submit))),
	)
}

// scopeLabels is what each scope is called when the screen says nothing else.
// A grant with no scope is the organisation, which is what a matrix without
// units always meant.
var scopeLabels = map[string]string{
	"":     "organisation",
	"unit": "unit",
	"tree": "unit and below",
}

// scopeSelect is the second half of a cell: how far that grant reaches. It is
// a select beside the level and not a column of its own, because the two
// answers are one sentence — "edit, in their own unit" — and a person reading
// a matrix reads it cell by cell.
func scopeSelect(p ScopedPolicy, opts PolicyGridOpts, role, module string) h.Node {
	id := "scope." + role + "." + module
	current := p.ScopeNameOf(role, module)
	// The options are written here and not with SelectOptions because the
	// empty value is a real answer — the whole organisation — and there it is
	// the disabled placeholder of a field nobody chose yet.
	options := make([]h.Node, 0, 3)
	for _, s := range p.ScopeNames() {
		label := scopeLabels[s]
		if l, ok := opts.ScopeLabels[s]; ok && l != "" {
			label = l
		}
		if label == "" {
			label = s
		}
		option := []h.Node{h.Value(s), h.Text(label)}
		if s == current {
			option = append(option, h.Selected())
		}
		options = append(options, h.Option(option...))
	}
	attrs := []h.Node{h.ID(id), h.Name(id),
		h.Class("ui-policy-scope"),
		h.Aria("label", role+" — "+module+" — scope"),
		h.Fragment(options...)}
	if opts.ReadOnly {
		attrs = append(attrs, h.Attr("disabled", ""))
	}
	return Select(attrs...)
}
