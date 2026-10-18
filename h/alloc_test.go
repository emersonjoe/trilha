package h

import (
	"bytes"
	"html"
	"strings"
	"testing"
)

func allocTree() Node {
	items := make([]Node, 0, 20)
	for i := 0; i < 20; i++ {
		items = append(items, Li(Class("item"), Class("ativo"), A(Href("/posts/um?x=1&y=2"), Text("Tom & Jerry <3"))))
	}
	return Div(Class("lista"), ID("posts"), Data("ui", "x"),
		H1(Text("Título")),
		Ul(items...),
		Input(Type("text"), Name("q"), Value(`"aspas" & <tags>`), Bool("required")),
	)
}

// Spec 177: rendering an already built tree into a reused buffer does not
// allocate. Every page renders a few hundred elements, and the allocations of
// building "<"+tag, the class slice and the escaped copy of every string were
// most of the garbage a request leaves behind.
func TestRenderDoesNotAllocate(t *testing.T) {
	tree := allocTree()
	var buf bytes.Buffer
	buf.Grow(1 << 14)
	for name, n := range map[string]Node{
		"tree":         tree,
		"escaped text": Text(`<script>alert("x") & 'y'</script>`),
		"one class":    Span(Class("ui-badge"), Text("ok")),
	} {
		allocs := testing.AllocsPerRun(50, func() {
			buf.Reset()
			if err := n.Render(&buf); err != nil {
				t.Fatal(err)
			}
		})
		if allocs != 0 {
			t.Errorf("%s: %.0f allocations per render, want 0", name, allocs)
		}
	}
}

// The escaper writing straight to the writer must say exactly what
// html.EscapeString says, for text and for attribute values.
func TestEscapeMatchesStdlib(t *testing.T) {
	for _, s := range []string{"", "plain", `<a href="x">&'</a>`, "&&&", "çãé <>", strings.Repeat("<", 100)} {
		got, _ := Render(Text(s))
		if want := html.EscapeString(s); got != want {
			t.Errorf("Text(%q) = %q, want %q", s, got, want)
		}
		got, _ = Render(Span(Attr("title", s)))
		if want := `<span title="` + html.EscapeString(s) + `"></span>`; got != want {
			t.Errorf("Attr(%q) = %q, want %q", s, got, want)
		}
	}
}

func BenchmarkRenderTree(b *testing.B) {
	tree := allocTree()
	var buf bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		tree.Render(&buf)
	}
}
