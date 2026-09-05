// Package tmpl adapts html/template to the h.Node pipeline, for developers
// who prefer template files over the Go DSL. Templates keep html/template's
// contextual escaping.
package tmpl

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"

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
