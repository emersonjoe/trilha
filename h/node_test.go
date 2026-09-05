package h

import "testing"

func mustRender(t *testing.T, n Node) string {
	t.Helper()
	s, err := Render(n)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEscapesTextAndAttributes(t *testing.T) {
	got := mustRender(t, Div(Class("a", "", "b"), TitleAttr(`x"y<z>`), Text("<script>&'")))
	want := `<div class="a b" title="x&#34;y&lt;z&gt;">&lt;script&gt;&amp;&#39;</div>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAttributesBeforeChildrenRegardlessOfOrder(t *testing.T) {
	got := mustRender(t, A(Text("x"), Href("/y"), Span(Text("z")), ID("k")))
	want := `<a href="/y" id="k">x<span>z</span></a>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestVoidElementsAndBooleans(t *testing.T) {
	got := mustRender(t, Fragment(Input(Type("checkbox"), Checked(), Text("ignored")), Br(), Img(Src("a.png"), Alt(""))))
	want := `<input type="checkbox" checked><br><img src="a.png" alt="">`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNilAndControlFlow(t *testing.T) {
	items := []string{"a", "b"}
	got := mustRender(t, Ul(nil, If(false, Li(Text("no"))), IfElse(true, Li(Text("yes")), nil),
		Map(items, func(s string) Node { return Li(Text(s)) })))
	want := `<ul><li>yes</li><li>a</li><li>b</li></ul>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRawAndDoctype(t *testing.T) {
	got := mustRender(t, Fragment(Doctype(), Html(Lang("pt-BR"), Body(Raw("<b>ok</b>")))))
	want := `<!doctype html><html lang="pt-BR"><body><b>ok</b></body></html>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestTextf(t *testing.T) {
	if got := mustRender(t, Textf("%d < %d", 1, 2)); got != "1 &lt; 2" {
		t.Fatal(got)
	}
}

func TestRenderNil(t *testing.T) {
	if got := mustRender(t, nil); got != "" {
		t.Fatal(got)
	}
}
