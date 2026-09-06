package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Column is one column of DataTable. Key is the name the URL uses to order by
// it, Cell renders the value of one row.
type Column[T any] struct {
	Key   string         // column name in the address; also what Sort accepts
	Label string         // header text
	Sort  bool           // the header is a link that orders by this column
	Num   bool           // numeric column: right-aligned
	Cell  func(T) h.Node // the cell of one row
}

// Columns is the columns of a listing, in the order they appear.
type Columns[T any] []Column[T]

// ListSelect adds a checkbox to each row and a bar with the actions that take
// the selection. The form is a POST with the CSRF token, so the action is a
// route like any other.
type ListSelect struct {
	Name   string           // name of the checkbox field, read with c.Request().Form[Name]
	Value  func(int) string // value of the row at that position (its id)
	Action string           // where the form posts; the current address when empty
	Label  string           // label of the select-all box; "Select all" when empty
	Bar    h.Node           // the buttons of the bar
}

// ListState is the listing minus the rows: where it is, how big it is, what it
// shows when it is empty, and which id the fragment swaps.
type ListState struct {
	Params  trilha.ListParams // what Bind read from the query
	Total   int               // how many rows there are in all, for the pagination
	ID      string            // id of the fragment; with it, ordering and paging do not reload
	Search  string            // placeholder of the q field; no search field when empty
	Filters h.Node            // the other fields of the filter form
	Empty   h.Node            // shown instead of the rows when there are none
	Caption string            // <caption> of the table, read by a screen reader
	RowHref func(int) string  // address the row at that position links to
	Select  *ListSelect       // row selection, off when nil
}

// DataTable renders the screen every management app has: filter on top, table
// in the middle, pagination at the foot, and all of it in the address — a page
// that can be shared, reloaded and used with the back button.
//
//	return ui.DataTable(c, ui.Columns[Doc]{
//		{Key: "name", Label: "File", Sort: true, Cell: func(d Doc) h.Node { return h.Text(d.Name) }},
//		{Key: "size", Label: "Size", Sort: true, Num: true, Cell: func(d Doc) h.Node { return h.Text(d.Size) }},
//	}, docs, ui.ListState{Params: q.ListParams, Total: total, ID: "list", Search: "Search"}), nil
//
// With ID set, the ordering links, the filter form and the pagination ask for
// the fragment and swap it in place; with JavaScript off the same links and the
// same form navigate, and the route answers the whole page. There is no new
// script here either way.
//
// A Sort that is not one of the columns marked Sort is dropped before anything
// is rendered, and reported in the request's log: the repository never receives
// a column name that nobody declared.
func DataTable[T any](c *trilha.Ctx, cols Columns[T], rows []T, st ListState) h.Node {
	var sortable []string
	for _, col := range cols {
		if col.Sort {
			sortable = append(sortable, col.Key)
		}
	}
	asked := st.Params.Sort
	if st.Params.Restrict(sortable...) {
		c.Log().Warn("ui: listing asked for a column that is not sortable", "sort", asked)
	}
	p := st.Params

	table := Table(
		caption(st),
		h.Thead(h.Tr(headCells(cols, p, st)...)),
		h.Tbody(bodyRows(cols, rows, st)...),
	)
	if st.Select != nil {
		table = h.Form(h.Class("ui-list-form"), h.Method("post"), action(st.Select.Action),
			trilha.CSRFInput(c),
			h.Div(h.Class("ui-list-bulk"), st.Select.Bar),
			table)
	}

	return h.Div(h.Class("ui-list"), listID(st.ID),
		filterForm(p, st),
		table,
		h.Footer(h.Class("ui-list-foot"),
			Muted(h.Text(count(st.Total))),
			Pagination(Pages{
				Page:  p.Page,
				Total: pagesOf(p, st.Total),
				Href:  p.PageHref,
				Attrs: swapAttr(st.ID),
			}),
		),
	)
}

func listID(id string) h.Node {
	if id == "" {
		return h.Group()
	}
	return h.ID(id)
}

func swapAttr(id string) h.Node {
	if id == "" {
		return h.Group()
	}
	return Swap(id)
}

func action(a string) h.Node {
	if a == "" {
		return h.Group()
	}
	return h.Action(a)
}

func caption(st ListState) h.Node {
	if st.Caption == "" {
		return h.Group()
	}
	return h.Caption(h.Text(st.Caption))
}

func count(total int) string {
	if total == 1 {
		return "1 result"
	}
	return strconv.Itoa(total) + " results"
}

func pagesOf(p trilha.ListParams, total int) int {
	if total <= 0 {
		return 0 // Pagination renders nothing below two pages
	}
	return p.TotalPages(total)
}

// headCells renders one <th> per column; a sortable one is a real link that
// turns the order around and goes back to the first page.
func headCells[T any](cols Columns[T], p trilha.ListParams, st ListState) []h.Node {
	var cells []h.Node
	if st.Select != nil {
		cells = append(cells, h.Th(h.Class("ui-list-check"), h.Attr("scope", "col"),
			h.Span(h.Class("ui-sr"), h.Text(label(st.Select.Label, "Select all")))))
	}
	for _, col := range cols {
		attrs := []h.Node{h.Attr("scope", "col")}
		if col.Num {
			attrs = append(attrs, Num())
		}
		if !col.Sort {
			cells = append(cells, h.Th(append(attrs, h.Text(col.Label))...))
			continue
		}
		next := "asc"
		if p.Sort == col.Key {
			if p.Asc() {
				next = "desc"
				attrs = append(attrs, h.Aria("sort", "ascending"))
			} else {
				attrs = append(attrs, h.Aria("sort", "descending"))
			}
		}
		href := p.Href("sort", col.Key, "dir", next, "page", "")
		attrs = append(attrs, h.A(h.Class("ui-sort"), h.Href(href), swapAttr(st.ID), h.Text(col.Label)))
		cells = append(cells, h.Th(attrs...))
	}
	return cells
}

// bodyRows renders the rows, or the empty state in a single cell.
func bodyRows[T any](cols Columns[T], rows []T, st ListState) []h.Node {
	span := len(cols)
	if st.Select != nil {
		span++
	}
	if len(rows) == 0 {
		empty := st.Empty
		if empty == nil {
			empty = Muted(h.Text("Nothing here."))
		}
		return []h.Node{h.Tr(h.Td(h.Class("ui-list-empty"), h.Attr("colspan", strconv.Itoa(span)), empty))}
	}
	out := make([]h.Node, 0, len(rows))
	for i, row := range rows {
		cells := make([]h.Node, 0, span)
		if st.Select != nil && st.Select.Value != nil {
			cells = append(cells, h.Td(h.Class("ui-list-check"),
				Checkbox(h.Name(st.Select.Name), h.Value(st.Select.Value(i)))))
		}
		for j, col := range cols {
			attrs := []h.Node{}
			if col.Num {
				attrs = append(attrs, Num())
			}
			var cell h.Node = h.Group()
			if col.Cell != nil {
				cell = col.Cell(row)
			}
			// The row link is a real link over the first cell, stretched over
			// the row by the stylesheet: clickable with the mouse, reachable
			// with the keyboard, and no script.
			if j == 0 && st.RowHref != nil {
				cell = h.A(h.Class("ui-row-link"), h.Href(st.RowHref(i)), cell)
			}
			cells = append(cells, h.Td(append(attrs, cell)...))
		}
		tr := []h.Node{}
		if st.RowHref != nil {
			tr = append(tr, h.Class("ui-row-linked"))
		}
		out = append(out, h.Tr(append(tr, cells...)...))
	}
	return out
}

// filterForm is a GET form: what the visitor types becomes the address, so the
// filtered listing can be shared and reloaded. What is not a field of the form
// travels in hidden inputs, or ordering would be lost at every search.
func filterForm(p trilha.ListParams, st ListState) h.Node {
	if st.Search == "" && st.Filters == nil {
		return h.Group()
	}
	fields := []h.Node{h.Class("ui-list-filters"), h.Method("get"), swapAttr(st.ID)}
	if st.Search != "" {
		fields = append(fields, Input(h.Type("search"), h.Name("q"), h.Value(p.Q),
			h.Aria("label", st.Search), h.Placeholder(st.Search)))
	}
	if st.Filters != nil {
		fields = append(fields, st.Filters)
	}
	if p.Sort != "" {
		fields = append(fields,
			h.Input(h.Type("hidden"), h.Name("sort"), h.Value(p.Sort)),
			h.Input(h.Type("hidden"), h.Name("dir"), h.Value(p.Dir)))
	}
	if p.PerPage != trilha.DefaultPerPage {
		fields = append(fields, h.Input(h.Type("hidden"), h.Name("per_page"), h.Value(strconv.Itoa(p.PerPage))))
	}
	fields = append(fields, Submit(h.Text("Filter")))
	return h.Form(fields...)
}
