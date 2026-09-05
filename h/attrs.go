package h

import "strings"

// Class joins the given classes with a space, skipping empty strings.
func Class(v ...string) Node {
	parts := v[:0:0]
	for _, s := range v {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return Attr("class", strings.Join(parts, " "))
}

// Data creates a data-<key> attribute.
func Data(key, value string) Node { return Attr("data-"+key, value) }

// Aria creates an aria-<key> attribute.
func Aria(key, value string) Node { return Attr("aria-"+key, value) }

func ID(v string) Node          { return Attr("id", v) }
func Href(v string) Node        { return Attr("href", v) }
func Src(v string) Node         { return Attr("src", v) }
func Alt(v string) Node         { return Attr("alt", v) }
func Type(v string) Node        { return Attr("type", v) }
func Name(v string) Node        { return Attr("name", v) }
func Value(v string) Node       { return Attr("value", v) }
func Placeholder(v string) Node { return Attr("placeholder", v) }
func Action(v string) Node      { return Attr("action", v) }
func Method(v string) Node      { return Attr("method", v) }
func Rel(v string) Node         { return Attr("rel", v) }
func Lang(v string) Node        { return Attr("lang", v) }
func Charset(v string) Node     { return Attr("charset", v) }
func Content(v string) Node     { return Attr("content", v) }
func For(v string) Node         { return Attr("for", v) }
func Role(v string) Node        { return Attr("role", v) }
func Target(v string) Node      { return Attr("target", v) }
func Width(v string) Node       { return Attr("width", v) }
func Height(v string) Node      { return Attr("height", v) }
func Rows(v string) Node        { return Attr("rows", v) }
func Cols(v string) Node        { return Attr("cols", v) }
func Min(v string) Node         { return Attr("min", v) }
func Max(v string) Node         { return Attr("max", v) }
func Step(v string) Node        { return Attr("step", v) }
func Pattern(v string) Node     { return Attr("pattern", v) }
func Enctype(v string) Node     { return Attr("enctype", v) }
func Accept(v string) Node      { return Attr("accept", v) }
func Datetime(v string) Node    { return Attr("datetime", v) }
func Tabindex(v string) Node    { return Attr("tabindex", v) }
func Onclick(v string) Node     { return Attr("onclick", v) }

// StyleAttr sets the style attribute (Style is the <style> element).
func StyleAttr(v string) Node { return Attr("style", v) }

// TitleAttr sets the title attribute (Title is the <title> element).
func TitleAttr(v string) Node { return Attr("title", v) }

// LabelAttr sets the label attribute (Label is the <label> element).
func LabelAttr(v string) Node { return Attr("label", v) }

// Boolean attributes.
func Disabled() Node   { return Bool("disabled") }
func Checked() Node    { return Bool("checked") }
func Selected() Node   { return Bool("selected") }
func Required() Node   { return Bool("required") }
func Autofocus() Node  { return Bool("autofocus") }
func Hidden() Node     { return Bool("hidden") }
func Readonly() Node   { return Bool("readonly") }
func Multiple() Node   { return Bool("multiple") }
func Open() Node       { return Bool("open") }
func Defer() Node      { return Bool("defer") }
func Async() Node      { return Bool("async") }
func Autoplay() Node   { return Bool("autoplay") }
func Controls() Node   { return Bool("controls") }
func Novalidate() Node { return Bool("novalidate") }
