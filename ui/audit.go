package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// AuditOpts is the screen around the trail: how much of it there is, what can
// be filtered, and where the export lives.
type AuditOpts struct {
	// Params is what Bind read from the query — the same ListParams the
	// listing of any other screen uses, so ordering and paging are in the
	// address and the back button works.
	Params trilha.ListParams
	// Total is how many records the filter matched, for the pagination.
	Total int
	// ID is the fragment the table swaps into; empty uses "auditoria".
	ID string
	// Actions fills the action filter. Empty draws no select: a list of
	// actions has to come from the application, and inventing one from the
	// page being shown would offer a filter that disappears when it works.
	Actions []string
	// Action is the one currently chosen, so the select comes back on the
	// option that is filtering. It is read by the application's own Bind —
	// the trail's query is theirs, and this component does not guess at it.
	Action string
	// Export is the address that downloads this same selection as a
	// spreadsheet — a route answering with Ctx.CSV. Empty draws no button.
	Export string
	// Empty replaces the default empty state.
	Empty h.Node
}

// AuditTable is the screen every application with c.Audit ends up writing by
// hand: who did what, to what, when and from where.
//
//	regs, total, err := auditoria.Buscar(c, q)
//	if err != nil {
//		return nil, err
//	}
//	return ui.AuditTable(c, regs, ui.AuditOpts{
//		Params: q.ListParams,
//		Total:  total,
//		Export: "/auditoria.csv",
//	}), nil
//
// It is a DataTable underneath, which is the point: the filter form, the
// ordering links, the pagination and the fragment swap are the ones every
// other listing already has, and a trail that behaved differently from the
// rest of the app would be a second thing to learn.
//
// Reading the records is the application's job. Config.AuditSink is a write
// interface with one method, and it stays that way: the framework has no
// database, and the query behind this screen — a period, an actor, a table
// this app chose — is not something it could write.
//
// Fields is a detail and not a column. Each action carries its own keys, so a
// column per key is a table that grows a column every time somebody audits
// something new.
//
//	see: trilha.Ctx.Audit, ui.DataTable, trilha.Ctx.CSV
func AuditTable(c *trilha.Ctx, records []trilha.AuditRecord, opts ...AuditOpts) h.Node {
	var o AuditOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	pt := langOf(c) == "pt-BR"
	id := o.ID
	if id == "" {
		id = "auditoria"
	}

	cols := Columns[trilha.AuditRecord]{
		{Key: "at", Label: word(pt, "When", "Quando"), Sort: true, Cell: func(r trilha.AuditRecord) h.Node {
			// Relative, because the question is almost always "was this now or
			// last week"; the exact instant is in the title of the same
			// element, for whoever needs to quote it.
			return Date(c, r.At, Relative())
		}},
		{Key: "actor", Label: word(pt, "Who", "Quem"), Sort: true, Cell: func(r trilha.AuditRecord) h.Node {
			return actorCell(r.Actor, pt)
		}},
		{Key: "action", Label: word(pt, "Action", "Ação"), Sort: true, Cell: func(r trilha.AuditRecord) h.Node {
			return Code(r.Action)
		}},
		{Key: "target", Label: word(pt, "Target", "Alvo"), Cell: func(r trilha.AuditRecord) h.Node {
			return h.Text(r.Target)
		}},
		{Key: "ip", Label: "IP", Cell: func(r trilha.AuditRecord) h.Node {
			return h.Span(h.Class("ui-muted"), h.Text(r.IP))
		}},
		{Key: "fields", Label: word(pt, "Detail", "Detalhe"), Cell: func(r trilha.AuditRecord) h.Node {
			return fieldsCell(r, pt)
		}},
	}

	empty := o.Empty
	if empty == nil {
		empty = Empty(EmptyOpts{
			Icon:  "info",
			Title: word(pt, "Nothing recorded in this selection", "Nada registrado neste recorte"),
			Hint: word(pt,
				"The trail keeps what the application audits; another period or another action may have something.",
				"A trilha guarda o que a aplicação audita; outro período ou outra ação podem ter algo."),
		})
	}

	return DataTable(c, cols, records, ListState{
		Params:  o.Params,
		Total:   o.Total,
		ID:      id,
		Search:  word(pt, "Actor, action or target", "Ator, ação ou alvo"),
		Filters: auditFilters(o, pt),
		Caption: word(pt, "Audit trail", "Trilha de auditoria"),
		Empty:   empty,
	})
}

// actorCell says who, and how they were recognised. Nobody recognised is
// written down as such — a trail that silently drops the anonymous action has
// a hole exactly where somebody would look.
func actorCell(a trilha.Actor, pt bool) h.Node {
	name := a.Name
	if name == "" {
		name = a.Email
	}
	if name == "" {
		name = a.Subject
	}
	if name == "" {
		name = word(pt, "anonymous", "anônimo")
	}
	kids := []h.Node{h.Text(name)}
	// The badge is only for what is not a person at a keyboard: a key, the
	// system, nobody. A badge on every row would be noise on the common case.
	if a.Via != "" && a.Via != "session" {
		kids = append(kids, h.Text(" "), Badge(Outline(), Sm(), h.Text(a.Via)))
	}
	return h.Span(kids...)
}

func fieldsCell(r trilha.AuditRecord, pt bool) h.Node {
	if len(r.Fields) == 0 {
		if r.Route == "" {
			return h.Text("")
		}
		return h.Span(h.Class("ui-muted"), h.Text(r.Route))
	}
	keys := make([]string, 0, len(r.Fields))
	for k := range r.Fields {
		keys = append(keys, k)
	}
	// In order, always the same: a detail that shuffles between two loads of
	// the same screen is a detail nobody trusts.
	sort.Strings(keys)
	rows := make([]h.Node, 0, len(keys)+1)
	for _, k := range keys {
		rows = append(rows, h.Div(h.Span(h.Class("ui-muted"), h.Text(k+": ")), h.Text(fieldText(r.Fields[k]))))
	}
	if r.Route != "" {
		rows = append(rows, h.Div(h.Span(h.Class("ui-muted"), h.Text("route: ")), h.Text(r.Route)))
	}
	return Collapsible(word(pt, "detail", "detalhe"),
		h.Div(append([]h.Node{h.Class("ui-audit-fields")}, rows...)...))
}

// fieldText writes a value the way a person reads it, without dragging fmt's
// %v into a screen: a map printed raw is not a detail, it is a wall.
func fieldText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// auditFilters is the action select and the export button, both carrying the
// query that is already in the address — an export that ignores the filter on
// screen is an export of the wrong thing.
func auditFilters(o AuditOpts, pt bool) h.Node {
	var kids []h.Node
	if len(o.Actions) > 0 {
		options := []Option{{Value: "", Label: word(pt, "Every action", "Todas as ações")}}
		for _, a := range o.Actions {
			options = append(options, Option{Value: a, Label: a})
		}
		kids = append(kids, Select(h.Name("action"), h.Aria("label", word(pt, "Action", "Ação")),
			SelectOptions(options, o.Action)))
	}
	if o.Export != "" {
		// The export carries the query that is on screen: an export that
		// ignores the filter in front of somebody is an export of the wrong
		// thing, and they only find out in the spreadsheet.
		href := o.Export + o.Params.Href("page", "")
		kids = append(kids, h.A(h.Href(href), h.Class("ui-btn ui-btn-outline ui-btn-sm"),
			h.Text(word(pt, "Export CSV", "Exportar CSV"))))
	}
	if len(kids) == 0 {
		return nil
	}
	return h.Fragment(kids...)
}
