package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// ComboboxOpts describes the field. Name is the name the form posts and the
// key of its message; Value and Label are what is already chosen — the value
// the app stores and the text a person reads — so a form that came back with
// messages comes back showing the choice instead of an identifier.
type ComboboxOpts struct {
	Name  string
	Value string // what the app stores, posted in the hidden input
	Label string // what the person sees, posted as Name+"_q"
	// Source is a GET route that answers ui.ComboboxOptions for ?q=. Empty
	// means the list is Options and the filtering happens in the browser.
	Source string
	// Options is a short list, filtered without a round trip.
	Options []Option
	// With names other fields of the same form whose values travel with the
	// search (?q=…&uf=SP), which is what makes a dependent picker stop being
	// twenty lines of JavaScript in the app.
	With []string
	// MinChars is how much has to be typed before the first search (1 when
	// zero); Debounce is how long the field waits for the typing to stop
	// (250ms when zero).
	MinChars int
	Debounce time.Duration
	// Placeholder, Required and Attrs go on the text input.
	Placeholder string
	Required    bool
	Attrs       []h.Node
}

// Combobox is a text field that searches: the server does the searching and
// answers a list of options, the browser only shows it. It is the picker of a
// classification among hundreds, of a city among thousands.
//
//	ui.Field("city", "City", ui.Combobox(ui.ComboboxOpts{
//		Name: "city_id", Value: doc.CityID, Label: doc.CityName,
//		Source: "/city/search", MinChars: 2,
//	}, ui.InvalidIf(errs, "city_id")))
//
//	// app/city/search/route.go
//	func GET(c *trilha.Ctx) error {
//		return c.HTML(200, ui.ComboboxOptions(find(c.Query("q")), func(ci City) (string, string) {
//			return ci.ID, ci.Name
//		}))
//	}
//
// The value the form posts is the hidden input, so the app reads Name as any
// other field. With this script off, or before anything was picked, the hidden
// input is empty and what the person typed arrives as Name+"_q" — the route
// resolves that text or answers 422, and the Label brings the field back the
// way it was.
func Combobox(o ComboboxOpts, attrs ...h.Node) h.Node {
	id := o.Name
	list := id + "-list"
	min := o.MinChars
	if min < 1 {
		min = 1
	}
	wait := o.Debounce
	if wait <= 0 {
		wait = 250 * time.Millisecond
	}
	text := []h.Node{
		h.ID(id), h.Name(o.Name + "_q"), h.Type("text"), h.Value(o.Label),
		h.Role("combobox"), h.Aria("expanded", "false"), h.Aria("controls", list),
		h.Aria("autocomplete", "list"), h.Attr("autocomplete", "off"),
	}
	if o.Placeholder != "" {
		text = append(text, h.Placeholder(o.Placeholder))
	}
	if o.Required {
		text = append(text, h.Required())
	}
	text = append(text, o.Attrs...)
	text = append(text, attrs...)

	box := []h.Node{
		h.Class("ui-combobox"), h.Data("ui-combo", ""),
		h.Data("ui-combo-min", strconv.Itoa(min)),
		h.Data("ui-combo-wait", strconv.Itoa(int(wait/time.Millisecond))),
	}
	if o.Source != "" {
		box = append(box, h.Data("ui-combo-src", o.Source))
	}
	if len(o.With) > 0 {
		box = append(box, h.Data("ui-combo-with", strings.Join(o.With, " ")))
	}
	box = append(box,
		Input(text...),
		h.Input(h.Type("hidden"), h.Name(o.Name), h.Value(o.Value)),
		optionList(list, o.Options),
	)
	return h.Div(box...)
}

// ComboboxOptions is the answer of a search route: the options themselves, as
// HTML, because the browser has nothing to decide about them.
func ComboboxOptions[T any](items []T, of func(T) (value, label string)) h.Node {
	opts := make([]Option, 0, len(items))
	for _, it := range items {
		v, l := of(it)
		opts = append(opts, Option{Value: v, Label: l})
	}
	return comboOptions(opts)
}

// optionList is the <ul> the search fills in; it starts closed, and the static
// options of a short list are already inside it.
func optionList(id string, opts []Option) h.Node {
	return h.Ul(h.ID(id), h.Class("ui-listbox"), h.Role("listbox"), h.Hidden(), comboOptions(opts))
}

func comboOptions(opts []Option) h.Node {
	items := make([]h.Node, 0, len(opts))
	for _, o := range opts {
		items = append(items, h.Li(h.Class("ui-option"), h.Role("option"),
			h.Aria("selected", "false"), h.Data("value", o.Value), h.Text(o.Label)))
	}
	return h.Group(items...)
}
