package md

import (
	"strings"
	"testing"
)

func TestBlocks(t *testing.T) {
	src := `# Título com ` + "`código`" + `

Um parágrafo com **negrito**, *itálico*, ` + "`x < y`" + ` e [link](/aprender) e [fora](https://go.dev).
Segunda linha do mesmo parágrafo.

## Lista

- um
- dois com ` + "`code`" + `
  continuação

1. primeiro
2. segundo

| Arquivo | Função |
|---------|--------|
| page.go | Page   |

> citação

:::dica
Texto da dica.
:::

:::solucao
` + "```go" + `
func Page() {}
` + "```" + `
:::

---

` + "```bash" + `
trilha dev <x>
` + "```" + `
`
	out, hs := Render(src, Options{Base: "/trilha"})
	checks := []string{
		`<h1 id="titulo-com-codigo">Título com <code>código</code><a class="ancora" href="#titulo-com-codigo"`,
		`<strong>negrito</strong>`, `<em>itálico</em>`, `<code>x &lt; y</code>`,
		`<a href="/trilha/aprender">link</a>`, `<a href="https://go.dev" rel="noopener">fora</a>`,
		`Segunda linha do mesmo parágrafo.</p>`,
		`<ul>` + "\n" + `<li>um</li>`, `<li>dois com <code>code</code> continuação</li>`,
		`<ol>` + "\n" + `<li>primeiro</li>`,
		`<th>Arquivo</th>`, `<td>page.go</td>`,
		`<blockquote><p>citação</p>`,
		`<aside class="aviso dica"><strong>Dica</strong>`,
		`<details class="solucao"><summary>Mostrar solução</summary>`,
		`<span class="k">func</span> <span class="f">Page</span>() {}`,
		`<hr>`, `<div class="codigo" data-lang="bash"><pre><code class="lang-bash">trilha dev &lt;x&gt;`,
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in:\n%s", c, out)
		}
	}
	if len(hs) != 2 || hs[0].Text != "Título com código" || hs[1].ID != "lista" || hs[1].Level != 2 {
		t.Fatalf("%+v", hs)
	}
}

func TestDuplicateHeadingIDs(t *testing.T) {
	out, hs := Render("## Desafio\n\n## Desafio\n", Options{})
	if hs[1].ID != "desafio-2" || !strings.Contains(out, `id="desafio-2"`) {
		t.Fatal(hs)
	}
}

func TestDemoDirective(t *testing.T) {
	out, _ := Render("@demo ola\n", Options{Demo: func(n string) string { return "<div>" + n + "</div>" }})
	if out != "<div>ola</div>" {
		t.Fatal(out)
	}
	if out, _ := Render("@demo ola\n", Options{}); out != "" {
		t.Fatal(out)
	}
}

func TestHighlightGo(t *testing.T) {
	got := HighlightGo(`s := "a\"b" // c
x := 42; t := ` + "`raw`" + `
return h.Div(nil)`)
	for _, want := range []string{`<span class="s">&#34;a\&#34;b&#34;</span>`, `<span class="c">// c</span>`, `<span class="n">42</span>`, "<span class=\"s\">`raw`</span>", `<span class="k">return</span>`, `<span class="f">Div</span>(<span class="t">nil</span>)`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
}

func TestSlug(t *testing.T) {
	if s := Slug("Páginas e rotas: `page.go`!"); s != "paginas-e-rotas-page-go" {
		t.Fatal(s)
	}
}

func TestEscapesHTMLInText(t *testing.T) {
	out, _ := Render("<script>alert(1)</script> e **<b>**\n", Options{})
	if strings.Contains(out, "<script>") || !strings.Contains(out, "&lt;script&gt;") || !strings.Contains(out, "<strong>&lt;b&gt;</strong>") {
		t.Fatal(out)
	}
}
