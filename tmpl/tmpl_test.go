package tmpl

import (
	"html/template"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

var fsys = fstest.MapFS{
	"rel.html":      {Data: []byte(`{{define "rel"}}<p>{{.Nome}}</p>{{end}}`)},
	"quebrado.html": {Data: []byte(`{{define "quebrado"}}{{.Nao.Existe}}{{end}}`)},
}

func TestNodeRendersInsideDSL(t *testing.T) {
	tpl := Must(fsys, "*.html")
	got, err := h.Render(h.Div(h.Class("x"), Node(tpl, "rel", map[string]string{"Nome": "<b>Ana</b>"})))
	if err != nil {
		t.Fatal(err)
	}
	if got != `<div class="x"><p>&lt;b&gt;Ana&lt;/b&gt;</p></div>` {
		t.Fatal(got)
	}
}

func TestMissingTemplateIsAnError(t *testing.T) {
	tpl := Must(fsys, "*.html")
	_, err := h.Render(Node(tpl, "nao-existe", nil))
	if err == nil || !strings.Contains(err.Error(), "nao-existe") {
		t.Fatal(err)
	}
	_, err = h.Render(Node(tpl, "quebrado", struct{}{}))
	if err == nil {
		t.Fatal("expected execution error")
	}
	if _, err := h.Render(Node(nil, "x", nil)); err == nil {
		t.Fatal("nil template must error")
	}
}

func TestMustPanicsOnBadPattern(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Must(fsys, "*.nada")
}

func TestPartialOutputIsDiscarded(t *testing.T) {
	tpl := template.Must(template.New("p").Parse(`antes {{.Nao.Existe}} depois`))
	var sb strings.Builder
	err := Node(tpl, "p", struct{}{}).Render(&sb)
	if err == nil || sb.Len() != 0 {
		t.Fatalf("err=%v out=%q", err, sb.String())
	}
}
