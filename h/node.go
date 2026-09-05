// Package h is a small, dependency-free HTML DSL: every element, attribute and
// piece of text is a Node that knows how to render itself to an io.Writer.
// Text and attribute values are escaped by default; Raw is the only way to
// emit unescaped HTML.
package h

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// Node is anything that can render itself as HTML.
type Node interface {
	Render(w io.Writer) error
}

// attrNode marks nodes that render inside the opening tag.
type attrNode interface {
	Node
	isAttr()
}

// Render renders a node to a string. Convenient for tests and small snippets.
func Render(n Node) (string, error) {
	var sb strings.Builder
	if n == nil {
		return "", nil
	}
	if err := n.Render(&sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// ---- element ---------------------------------------------------------------

type element struct {
	tag      string
	void     bool
	children []Node
}

// El creates an element with an arbitrary tag. Attributes and children can be
// mixed in any order; attributes always end up in the opening tag.
func El(tag string, children ...Node) Node {
	return &element{tag: tag, children: children}
}

// Void creates a void element (no closing tag, no children).
func Void(tag string, attrs ...Node) Node {
	return &element{tag: tag, void: true, children: attrs}
}

func (e *element) Render(w io.Writer) error {
	if _, err := io.WriteString(w, "<"+e.tag); err != nil {
		return err
	}
	for _, c := range e.children {
		if a, ok := c.(attrNode); ok {
			if err := a.Render(w); err != nil {
				return err
			}
		}
	}
	if _, err := io.WriteString(w, ">"); err != nil {
		return err
	}
	if e.void {
		return nil
	}
	for _, c := range e.children {
		if c == nil {
			continue
		}
		if _, ok := c.(attrNode); ok {
			continue
		}
		if err := c.Render(w); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "</"+e.tag+">")
	return err
}

// ---- attributes ------------------------------------------------------------

type attr struct {
	name    string
	value   string
	boolean bool
}

func (attr) isAttr() {}

func (a attr) Render(w io.Writer) error {
	if a.boolean {
		_, err := io.WriteString(w, " "+a.name)
		return err
	}
	_, err := io.WriteString(w, " "+a.name+`="`+html.EscapeString(a.value)+`"`)
	return err
}

// Attr creates a name="value" attribute. The value is escaped.
func Attr(name, value string) Node { return attr{name: name, value: value} }

// Bool creates a boolean attribute such as disabled or checked.
func Bool(name string) Node { return attr{name: name, boolean: true} }

// ---- text ------------------------------------------------------------------

type text string

func (t text) Render(w io.Writer) error {
	_, err := io.WriteString(w, html.EscapeString(string(t)))
	return err
}

// Text renders escaped text.
func Text(s string) Node { return text(s) }

// Textf renders escaped formatted text.
func Textf(format string, a ...any) Node { return text(fmt.Sprintf(format, a...)) }

type raw string

func (r raw) Render(w io.Writer) error {
	_, err := io.WriteString(w, string(r))
	return err
}

// Raw renders HTML without escaping. Never pass user input to Raw.
func Raw(s string) Node { return raw(s) }

// ---- fragment / doctype ----------------------------------------------------

type fragment []Node

func (f fragment) Render(w io.Writer) error {
	for _, n := range f {
		if n == nil {
			continue
		}
		if err := n.Render(w); err != nil {
			return err
		}
	}
	return nil
}

// Fragment renders its children in sequence without a wrapping element.
func Fragment(children ...Node) Node { return fragment(children) }

// Group is an alias of Fragment.
func Group(children ...Node) Node { return fragment(children) }

type doctype struct{}

func (doctype) Render(w io.Writer) error {
	_, err := io.WriteString(w, "<!doctype html>")
	return err
}

// Doctype renders <!doctype html>.
func Doctype() Node { return doctype{} }

// Nil is a node that renders nothing.
var Nil Node = fragment(nil)
