// Package tmpl adapts html/template to the h.Node pipeline, for developers
// who prefer template files over the Go DSL. Templates keep html/template's
// contextual escaping.
package tmpl

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"strings"

	"github.com/emersonjoe/trilha/h"
)

type node struct {
	t    *template.Template
	name string
	data any
}

// Node renders the named template with data as an h.Node. Errors surface as
// render errors (a 500 page), never as partial output.
func Node(t *template.Template, name string, data any) h.Node {
	return node{t: t, name: name, data: data}
}

func (n node) Render(w io.Writer) error {
	if n.t == nil {
		return errors.New("tmpl: nil template")
	}
	var buf bytes.Buffer
	if err := n.t.ExecuteTemplate(&buf, n.name, n.data); err != nil {
		return fmt.Errorf("tmpl: %w", err)
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// Must parses templates from fsys (typically an embed.FS) matching the
// patterns and panics on error. Call it at package level so a broken
// template fails at startup, not on a request.
func Must(fsys fs.FS, patterns ...string) *template.Template {
	return template.Must(template.ParseFS(fsys, patterns...))
}

// Shell is a template that surrounds an h.Node: the shell of an app that
// already exists in html/template, with the new screens written in h. Build it
// with Wrap.
type Shell struct {
	t      *template.Template
	name   string
	marker string
}

// Wrap prepares the template name to receive an h.Node wherever it calls
// {{template slot .}}. Call it at package level, next to Must — it clones the
// template set, and html/template only clones a set that has not run yet:
//
//	var shell = tmpl.Wrap(t, "layout.html", "content")
//
//	func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
//		return shell.Node(page(c.Request()), children), nil
//	}
//
// It panics on a nil or already executed template, the same way Must does: a
// shell that cannot be prepared is a mistake to see at startup, not on a
// request. Nothing in the app touches template.HTML, and the shell keeps its
// shape — the slot stays the {{template}} call it already was.
func Wrap(t *template.Template, name, slot string) *Shell {
	if t == nil {
		panic("tmpl: Wrap: nil template")
	}
	c, err := t.Clone()
	if err != nil {
		panic("tmpl: Wrap must run before the template is executed (call it at package level): " + err.Error())
	}
	// The slot renders a marker of 16 random bytes, and Node writes the child
	// where the marker landed. Preparing the shell once and cutting its output
	// costs a fraction of cloning the set on every request, and hexadecimal
	// crosses html/template's escaping unchanged in any context.
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("tmpl: Wrap: " + err.Error())
	}
	marker := "trilha-slot-" + hex.EncodeToString(b[:])
	if _, err := c.Parse(fmt.Sprintf("{{define %q}}%s{{end}}", slot, marker)); err != nil {
		panic("tmpl: Wrap: " + err.Error())
	}
	return &Shell{t: c, name: name, marker: marker}
}

// Node renders the shell with data, putting children in the slot. Errors
// surface as render errors, never as partial output.
func (s *Shell) Node(data any, children h.Node) h.Node {
	return shellNode{s: s, data: data, children: children}
}

type shellNode struct {
	s        *Shell
	data     any
	children h.Node
}

func (n shellNode) Render(w io.Writer) error {
	var shell, inner bytes.Buffer
	if err := n.s.t.ExecuteTemplate(&shell, n.s.name, n.data); err != nil {
		return fmt.Errorf("tmpl: %w", err)
	}
	if n.children != nil {
		if err := n.children.Render(&inner); err != nil {
			return err
		}
	}
	out := shell.String()
	if !strings.Contains(out, n.s.marker) {
		// A shell that never calls the slot would answer a page without its
		// content, and the browser would show it as if that were the page.
		return fmt.Errorf("tmpl: template %q never rendered the slot", n.s.name)
	}
	for {
		i := strings.Index(out, n.s.marker)
		if i < 0 {
			break
		}
		if _, err := io.WriteString(w, out[:i]); err != nil {
			return err
		}
		if _, err := w.Write(inner.Bytes()); err != nil {
			return err
		}
		out = out[i+len(n.s.marker):]
	}
	_, err := io.WriteString(w, out)
	return err
}

// HTML renders a node as template.HTML, for data handed to a template the app
// executes itself. The h package escaped what had to be escaped on the way in;
// this is the one place that says so, instead of a conversion in the app.
func HTML(n h.Node) (template.HTML, error) {
	if n == nil {
		return "", nil
	}
	var buf bytes.Buffer
	if err := n.Render(&buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
