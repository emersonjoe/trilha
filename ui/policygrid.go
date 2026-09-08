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
	// CSRF is the hidden token field, and a form that changes permissions has
	// to carry one: pass trilha.CSRFInput(c). It is an option and not a Ctx
	// argument because no other component of the kit takes a Ctx, and the one
	// that quietly did would be the odd one out.
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
			cells = append(cells, h.Td(Select(attrs...)))
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
