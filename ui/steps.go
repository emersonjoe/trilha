package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha/h"
)

// Step is one screen of a multi-step form.
type Step struct {
	// Label is what it is called: "File", "Mapping", "Confirm".
	Label string
	// Href takes somebody back to a step already done. A step ahead of the
	// current one never links, whatever this says: a wizard where step three
	// is one click away is a wizard whose steps did not have to happen in
	// order.
	Href string
}

// Steps is the indicator of a form in several screens: where somebody is, what
// is behind them, and what is left.
//
//	var passos = []ui.Step{
//		{Label: "File", Href: "/import/1"},
//		{Label: "Mapping", Href: "/import/2"},
//		{Label: "Confirm"},
//	}
//
//	ui.Steps(passos, 2)
//
// current is 1-based and is the step being filled. Behind it the steps are
// links (they were done, and going back is normal); ahead of it they are
// plain text. The current one carries aria-current="step", which is how a
// screen reader says "you are here" in a list of five.
//
// It draws the indicator and nothing else: the state between one screen and
// the next is trilha.Ctx.Draft, and the form is yours.
//
//	see: trilha.Ctx.Draft
func Steps(steps []Step, current int) h.Node {
	items := make([]h.Node, 0, len(steps))
	for i, s := range steps {
		n := i + 1
		kids := []h.Node{
			h.Class("ui-step"),
			h.Data("state", stepState(n, current)),
			h.Span(h.Class("ui-step-mark"), h.Aria("hidden", "true"), h.Text(strconv.Itoa(n))),
		}
		if n == current {
			kids = append(kids, h.Aria("current", "step"))
		}
		// Only a step already done is a link. A step ahead has nothing to show
		// yet, and a link to it is an invitation to skip the one being filled.
		if s.Href != "" && n < current {
			kids = append(kids, h.A(h.Href(s.Href), h.Text(s.Label)))
		} else {
			kids = append(kids, h.Span(h.Text(s.Label)))
		}
		items = append(items, h.Li(kids...))
	}
	return h.Ol(append([]h.Node{h.Class("ui-steps")}, items...)...)
}

func stepState(n, current int) string {
	switch {
	case n < current:
		return "done"
	case n == current:
		return "current"
	}
	return "todo"
}
