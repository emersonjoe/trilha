package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// DeferOpts is what stands in the hole until the slow part arrives, and what
// is said when it never does. The zero value is a skeleton and the kit's own
// sentences.
type DeferOpts struct {
	// Placeholder is what is drawn while the fragment is on its way. Nil
	// draws a Skeleton of Height.
	Placeholder h.Node
	// Height is how tall that skeleton is — "12rem". It is not decoration:
	// a placeholder shorter than what replaces it makes the page jump under
	// somebody's cursor, which is the whole reason the placeholder exists.
	// Empty means 8rem.
	Height string
	// Then is what the fragment does after it has arrived — Poll("30s", src)
	// to keep watching. The attributes travel on the same element, so the
	// route's answer decides the rest.
	Then h.Node
	// Load is the label of the link a visitor without JavaScript sees.
	Load string
	// Error and Retry are the two sentences of the failure: what happened,
	// and the button that asks again.
	Error, Retry string
}

// Defer serves the page now and fills this part of it a moment later.
//
//	ui.Container(
//		ui.Grid(stats...),
//		ui.Defer(c, "insights", "/panel/insights", ui.DeferOpts{Height: "12rem"}),
//	)
//
// The dashboard that needs seven queries to draw does not have to hold the
// whole page for the one that takes two seconds. The fast part is rendered on
// the server as usual; this one is a placeholder that the kit swaps as soon as
// the page has loaded — once, with no clock, carrying the session and the
// headers of the page it sits in.
//
// src is an ordinary route that answers Ctx.Fragment, the same route Poll would
// ask, and it must return an element with this id — that is what is being
// replaced. Without JavaScript the placeholder carries a link to src, which
// answers as a page like any other: the slow part is one click away instead of
// missing.
//
// A fragment that fails puts a message and a "try again" in the hole rather
// than pulsing for ever, and both sentences are rendered here, in the app's
// language, so the behaviour never has to invent text.
//
// Load LiveScript once in the layout — Defer is the same machinery as Poll.
//
//	see: ui.Poll, ui.LiveScript, ui.Skeleton
func Defer(c *trilha.Ctx, id, src string, opts ...DeferOpts) h.Node {
	var o DeferOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	pt := langOf(c) == "pt-BR"
	wait := o.Placeholder
	if wait == nil {
		height := o.Height
		if height == "" {
			height = "8rem"
		}
		wait = Skeleton(h.StyleAttr("height: " + height))
	}
	load := o.Load
	if load == "" {
		load = word(pt, "Load", "Carregar")
	}
	fail := o.Error
	if fail == "" {
		fail = word(pt, "This part could not be loaded.", "Não deu para carregar esta parte.")
	}
	retry := o.Retry
	if retry == "" {
		retry = word(pt, "Try again", "Tentar de novo")
	}

	kids := []h.Node{
		h.ID(id),
		h.Data("trilha-defer", ""),
		h.Data("trilha-src", src),
	}
	if o.Then != nil {
		kids = append(kids, o.Then)
	}
	kids = append(kids,
		h.Div(h.Data("trilha-defer-wait", ""), wait,
			// The link is inside <noscript>, so the browser that is going to
			// swap this in a moment never shows a button that would take the
			// person off the page they are already on.
			h.Noscript(h.A(h.Href(src), h.Class("ui-btn ui-btn-outline ui-btn-sm"), h.Text(load)))),
		// The failure is rendered now and hidden: the behaviour only unhides
		// it, and every sentence and class stays on this side.
		h.Div(h.Data("trilha-defer-fail", ""), h.Hidden(),
			EmptyError(c, fail, nil, Button(Outline(), Sm(), h.Type("button"),
				h.Data("trilha-defer-retry", ""), h.Text(retry)))),
	)
	return h.Div(kids...)
}
