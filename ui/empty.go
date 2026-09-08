package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// EmptyOpts is what an empty state says. Only Title is required.
type EmptyOpts struct {
	// Icon is a name from the kit's set. Empty draws none.
	Icon string
	// Title is the sentence in bold: "No documents yet".
	Title string
	// Hint is the smaller line under it, and it is the one that earns its
	// place: "Send the first PDF and classification starts on its own" tells
	// somebody what to do, where "no results" only tells them to leave.
	Hint string
	// Action is the way out — a link, a button, a form. Nil draws none.
	Action h.Node
}

// Empty is the state of a screen with nothing on it: centred icon, a title, a
// hint and a way out.
//
//	ui.Empty(ui.EmptyOpts{
//		Icon:   "info",
//		Title:  "No documents yet",
//		Hint:   "Send the first PDF and classification starts on its own.",
//		Action: ui.ButtonLink("/upload", h.Text("Send a document")),
//	})
//
// It exists because the alternative is what every application actually has: one
// hand-written <p> per screen, none of them alike, and none of them saying what
// to do next.
//
//	see: ui.EmptyError, ui.DataTable
func Empty(opts EmptyOpts) h.Node {
	kids := make([]h.Node, 0, 4)
	// A name the kit does not have would panic, and panicking on the screen
	// that is already empty is the worst place for it: no icon is a fine empty
	// state, a 500 is not.
	if opts.Icon != "" && hasIcon(opts.Icon) {
		kids = append(kids, h.Div(h.Class("ui-empty-icon"), Icon(opts.Icon)))
	}
	if opts.Title != "" {
		kids = append(kids, h.P(h.Class("ui-empty-title"), h.Text(opts.Title)))
	}
	if opts.Hint != "" {
		kids = append(kids, h.P(h.Class("ui-empty-hint"), h.Text(opts.Hint)))
	}
	if opts.Action != nil {
		kids = append(kids, h.Div(h.Class("ui-empty-action"), opts.Action))
	}
	return h.Div(append([]h.Node{h.Class("ui-empty")}, kids...)...)
}

// EmptyError is the state of a screen that could not load. The message is what
// the person reads; the error itself is shown only in development, because a
// stack or a driver's sentence on a production page is an information leak with
// a friendly font.
//
//	return ui.EmptyError(c, "Could not load the documents", err,
//		ui.ButtonLink(c.Request().URL.String(), h.Text("Try again")))
//
//	see: ui.Empty
func EmptyError(c *trilha.Ctx, title string, err error, action h.Node) h.Node {
	hint := ""
	if err != nil && c != nil && c.Env() == trilha.Dev {
		hint = err.Error()
	}
	return h.Div(h.Class("ui-empty ui-empty-error"), h.Role("alert"),
		Empty(EmptyOpts{Icon: "triangle-alert", Title: title, Hint: hint, Action: action}))
}

// emptyFor is what DataTable shows when there are no rows and the caller said
// nothing. The distinction is the point: a list that is empty and a list that
// was filtered down to empty are two different screens, and only one of them
// has a way out that is not "give up".
func emptyFor(st ListState) h.Node {
	if q := st.Params.Q; q != "" {
		return Empty(EmptyOpts{
			Icon:   "search",
			Title:  "No results for “" + q + "”",
			Hint:   "Try another term, or clear the search to see everything.",
			Action: h.A(h.Href(st.Params.Href("q", "", "page", "")), h.Class("ui-btn ui-btn-outline ui-btn-sm"), h.Text("Clear the search")),
		})
	}
	return Empty(EmptyOpts{Icon: "info", Title: "Nothing here yet"})
}

// hasIcon reports whether the kit carries that name. Icon panics on one it does
// not, which is right for a page the developer is writing and wrong for a
// component that draws whatever an option happens to hold.
func hasIcon(name string) bool {
	_, ok := icons[name]
	return ok
}
