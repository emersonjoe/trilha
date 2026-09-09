package trilha

import (
	"strings"
	"sync"

	"github.com/emersonjoe/trilha/h"
)

// EnumValue is one value of a domain list: what the database holds, what the
// person reads, and which of the theme's tones it wears.
type EnumValue struct {
	Value string
	Label string
	// Tone is one of muted, info, success, warning, danger and accent — a
	// name from the theme and never a CSS class. Whoever declares a status
	// picks a meaning; picking a colour is how two screens end up with two
	// different greens. Empty means muted.
	Tone string
}

// Enum is a domain list declared once: the status of a document, the type of a
// contract, the stage of a pipeline.
//
//	var Status = trilha.Enum{
//		{Value: "draft",     Label: "Draft"},
//		{Value: "published", Label: "Published", Tone: "success"},
//	}
//
// The same declaration answers the four places an application repeats itself:
// the badge on the table, the cell of a DataTable, the options of a select
// (Options) and the validation of the form (the enum= tag). Written four times,
// they drift; the fourth screen is where the label is wrong.
//
// Translation is not here on purpose: Label is a string, and an application
// with two languages passes two enums or builds one from its own table. A
// translation table inside this would be a second, worse i18n.
type Enum []EnumValue

// Has reports whether the value is one of the list.
func (e Enum) Has(value string) bool {
	for _, v := range e {
		if v.Value == value {
			return true
		}
	}
	return false
}

// Label is what a person reads, or the raw value when the list does not know
// it. A row written before this value was retired must not take the screen
// down: it shows as it is, in the default tone.
func (e Enum) Label(value string) string {
	for _, v := range e {
		if v.Value == value {
			if v.Label != "" {
				return v.Label
			}
			return v.Value
		}
	}
	return value
}

// Tone is the tone of a value, and "muted" for one the list does not know.
func (e Enum) Tone(value string) string {
	for _, v := range e {
		if v.Value == value {
			if v.Tone != "" {
				return v.Tone
			}
			return "muted"
		}
	}
	return "muted"
}

// Labels is every label, in declaration order. It is what the validation
// message lists, because a person choosing from a form saw labels and not
// values.
func (e Enum) Labels() []string {
	out := make([]string, 0, len(e))
	for _, v := range e {
		out = append(out, e.Label(v.Value))
	}
	return out
}

// Values is every value, in declaration order.
func (e Enum) Values() []string {
	out := make([]string, 0, len(e))
	for _, v := range e {
		out = append(out, v.Value)
	}
	return out
}

// Options renders the <option> list of a select, marking the current value.
// A placeholder passed as the second argument becomes an empty first option.
//
//	ui.Select(docs.Status.Options(form.Status))
//	ui.Select(docs.Status.Options(form.Status, "— choose —"))
//
// It returns nodes and not a slice of some Option type so that it composes
// with the kit's Select without either package having to know the other's.
func (e Enum) Options(current string, placeholder ...string) h.Node {
	out := make([]h.Node, 0, len(e)+1)
	if len(placeholder) > 0 {
		attrs := []h.Node{h.Value("")}
		if current == "" {
			attrs = append(attrs, h.Selected())
		}
		out = append(out, h.Option(append(attrs, h.Text(placeholder[0]))...))
	}
	for _, v := range e {
		attrs := []h.Node{h.Value(v.Value)}
		if v.Value == current {
			attrs = append(attrs, h.Selected())
		}
		out = append(out, h.Option(append(attrs, h.Text(e.Label(v.Value)))...))
	}
	return h.Fragment(out...)
}

// enums is the registry the validate tag reads.
var (
	enumsMu sync.RWMutex
	enums   = map[string]Enum{}
)

// RegisterEnum names an enum so a validate tag can cite it.
//
//	trilha.RegisterEnum("docs.Status", docs.Status)   // in Setup
//	type Form struct {
//		Status string `validate:"required,enum=docs.Status"`
//	}
//
// Registering the same list again is fine, and it has to be: Setup is where
// this belongs, and a test suite that boots the app once per test runs Setup
// once per test. What is refused is two different lists behind one name, which
// is a bug nobody finds later — the form would validate against one and the
// select would draw the other.
func RegisterEnum(name string, e Enum) {
	enumsMu.Lock()
	defer enumsMu.Unlock()
	if old, ok := enums[name]; ok && !old.same(e) {
		panic("trilha: enum " + name + " is already registered with different values")
	}
	enums[name] = e
}

// same reports whether two declarations are the same list, value for value.
func (e Enum) same(other Enum) bool {
	if len(e) != len(other) {
		return false
	}
	for i := range e {
		if e[i] != other[i] {
			return false
		}
	}
	return true
}

// LookupEnum returns a registered enum.
//
// It was unexported until something outside needed it, which was the condition
// written here in 0.46.0: an exported symbol with no consumer is a promise
// nobody asked for. `trilha ctx` reads the same lists from source now, and an
// application that wants them at runtime — a screen offering every value of a
// status, a report grouping by it — has this.
func LookupEnum(name string) (Enum, bool) {
	enumsMu.RLock()
	defer enumsMu.RUnlock()
	e, ok := enums[name]
	return e, ok
}

// RegisteredEnums is every list this application registered, by name. It is a
// copy: a caller that ranged over the real map while a Setup was still running
// would be reading a map somebody else is writing.
func RegisteredEnums() map[string]Enum {
	enumsMu.RLock()
	defer enumsMu.RUnlock()
	out := make(map[string]Enum, len(enums))
	for k, v := range enums {
		out[k] = append(Enum(nil), v...)
	}
	return out
}

// ruleEnum is the enum= tag. The message lists the labels and not the values,
// because the person filling the form read labels.
//
// It renders the message itself and hands it back as the key, which the message
// lookup passes through untouched — that fallback exists so an unknown key is
// visible instead of silent, and it is what lets this rule say something the
// static table cannot: the list depends on which enum the tag named.
func ruleEnum(f Field) (bool, string) {
	e, ok := LookupEnum(f.Param)
	if !ok {
		// A tag naming an enum nobody registered would otherwise accept
		// anything, which is the failure this refuses to have quietly.
		return false, message("enum unknown", f.Param)
	}
	if e.Has(f.Text) {
		return true, "enum"
	}
	return false, message("enum", strings.Join(e.Labels(), ", "))
}
