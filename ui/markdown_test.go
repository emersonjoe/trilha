package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// TestMarkdownGolden renders a fixed corpus. Markdown is read by looking at
// it, so the review of a change is the diff of the HTML.
func TestMarkdownGolden(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "markdown.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := render(t, Markdown(string(src), MarkdownOpts{}))
	// One tag per line: a golden that diffs by line is a golden somebody
	// reviews.
	got = strings.ReplaceAll(got, "><", ">\n<")
	p := filepath.Join("testdata", "markdown.html")
	if *update {
		if err := os.WriteFile(p, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%v (run: go test ./ui -run TestMarkdownGolden -update)", err)
	}
	if got != strings.TrimRight(string(want), "\n") {
		t.Errorf("markdown changed:\ngot:\n%s\n\nwant:\n%s", got, want)
	}
}

// mdOurTags is every tag Markdown itself can emit. The invariant the fuzz
// target and the escaping test check is the strong one: every "<" in the
// output opens one of these, so nothing in the source became markup.
var mdOurTags = map[string]bool{
	"div": true, "p": true, "br": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"ul": true, "ol": true, "li": true, "blockquote": true,
	"pre": true, "code": true, "a": true, "img": true,
	"strong": true, "em": true, "del": true,
	"table": true, "thead": true, "tbody": true, "tr": true, "th": true, "td": true,
}

// mdStrayTag returns the first tag in out that Markdown does not emit itself.
func mdStrayTag(out string) string {
	for i := 0; i < len(out); i++ {
		if out[i] != '<' {
			continue
		}
		rest := out[i+1:]
		rest = strings.TrimPrefix(rest, "/")
		j := strings.IndexAny(rest, " >/\t\n")
		if j < 0 {
			return out[i:]
		}
		if !mdOurTags[strings.ToLower(rest[:j])] {
			return out[i : i+1+j+1]
		}
	}
	return ""
}

// TestMarkdownHasNoRawHTML is the rule with no exception: the text came from a
// model or from a stranger, so a tag in it is a tag on the screen, not in the
// document.
func TestMarkdownHasNoRawHTML(t *testing.T) {
	for _, src := range []string{
		`<script>alert(1)</script>`,
		`<img src=x onerror="alert(1)">`,
		`</p><div onclick="x">`,
		"`<script>`",
		"> <script>alert(1)</script>",
		"| <script> | b |\n|---|---|\n| c | d |",
		"# <script>alert(1)</script>",
		"```\n<script>alert(1)</script>\n```",
	} {
		got := render(t, Markdown(src, MarkdownOpts{}))
		if stray := mdStrayTag(got); stray != "" {
			t.Fatalf("%q produced the tag %q in %q", src, stray, got)
		}
		if !strings.Contains(got, "&lt;") {
			t.Fatalf("%q lost the escaped tag: %q", src, got)
		}
	}
}

func TestMarkdownRefusesADangerousLink(t *testing.T) {
	for _, src := range []string{
		`[x](javascript:alert(1))`,
		`[x](JavaScript:alert(1))`,
		`[x](data:text/html,<script>alert(1)</script>)`,
		`[x](vbscript:msgbox)`,
	} {
		got := render(t, Markdown(src, MarkdownOpts{}))
		if strings.Contains(got, "<a ") {
			t.Fatalf("%q became a link: %q", src, got)
		}
		if !strings.Contains(got, "x") {
			t.Fatalf("%q lost its text: %q", src, got)
		}
	}
	// What is on the list keeps working, and what points outside says so.
	got := render(t, Markdown(`[a](https://x.test/p) [b](/local) [c](#here) [d](mailto:a@b.test)`, MarkdownOpts{}))
	for _, want := range []string{
		`<a href="https://x.test/p" rel="noopener nofollow ugc">a</a>`,
		`<a href="/local">b</a>`,
		`<a href="#here">c</a>`,
		`<a href="mailto:a@b.test">d</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestMarkdownDemotesHeadings(t *testing.T) {
	got := render(t, Markdown("# One\n\n## Two", MarkdownOpts{}))
	if !strings.Contains(got, "<h3>One</h3>") || !strings.Contains(got, "<h4>Two</h4>") {
		t.Fatalf("default demotion: %q", got)
	}
	got = render(t, Markdown("# One", MarkdownOpts{HeadingBase: 1}))
	if !strings.Contains(got, "<h1>One</h1>") {
		t.Fatalf("HeadingBase: %q", got)
	}
	// Six levels from a base of three still stop at h6, which is the last
	// heading HTML has.
	got = render(t, Markdown("###### Deep", MarkdownOpts{}))
	if !strings.Contains(got, "<h6>Deep</h6>") {
		t.Fatalf("floor: %q", got)
	}
	// Seven hashes is not a heading at all.
	got = render(t, Markdown("####### Nope", MarkdownOpts{}))
	if strings.Contains(got, "<h") {
		t.Fatalf("seven hashes: %q", got)
	}
}

func TestMarkdownImagesAreOptIn(t *testing.T) {
	src := `![a picture](https://x.test/p.png)`
	got := render(t, Markdown(src, MarkdownOpts{}))
	if strings.Contains(got, "<img") {
		t.Fatalf("image without asking: %q", got)
	}
	if !strings.Contains(got, "a picture") {
		t.Fatalf("alt text lost: %q", got)
	}
	got = render(t, Markdown(src, MarkdownOpts{Images: true}))
	if !strings.Contains(got, `<img src="https://x.test/p.png"`) || !strings.Contains(got, `alt="a picture"`) {
		t.Fatalf("image with Images: %q", got)
	}
	// The URL is checked even when images are on.
	got = render(t, Markdown(`![x](javascript:alert(1))`, MarkdownOpts{Images: true}))
	if strings.Contains(got, "<img") {
		t.Fatalf("dangerous src passed: %q", got)
	}
}

func TestMarkdownFenceInsideAListItem(t *testing.T) {
	src := "- first, with code:\n\n  ```go\n  x := 1\n  ```\n\n- second"
	got := render(t, Markdown(src, MarkdownOpts{}))
	if !strings.Contains(got, `<pre class="ui-pre"><code class="ui-code language-go" data-lang="go">x := 1</code></pre>`) {
		t.Fatalf("fence in list: %q", got)
	}
	if strings.Count(got, "<li>") != 2 {
		t.Fatalf("items: %q", got)
	}
}

func TestMarkdownTable(t *testing.T) {
	got := render(t, Markdown("| a | b |\n|:--|--:|\n| 1 | 2 |\n| 3 | 4 |", MarkdownOpts{}))
	for _, want := range []string{"<th>a</th>", "<th>b</th>", "<td>1</td>", "<td>4</td>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Count(got, "<tr>") != 3 {
		t.Fatalf("rows: %q", got)
	}
}

func TestMarkdownKeepsPlainTextPlain(t *testing.T) {
	// Text with no markup in it comes out as one paragraph, not as a pile of
	// empty tags: the common case for a model answer is prose.
	got := render(t, Markdown("Just a sentence about 2 * 3 * 4 and snake_case_names.", MarkdownOpts{}))
	want := `<div class="ui-md"><p>Just a sentence about 2 * 3 * 4 and snake_case_names.</p></div>`
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// renderNode is h.Render without a *testing.T, for the fuzz target.
func renderNode(n h.Node) (string, error) { return h.Render(n) }

func FuzzMarkdown(f *testing.F) {
	for _, s := range []string{
		"# hi\n\n- a\n- b\n",
		"[x](javascript:alert(1))",
		"`a``b`",
		"**a*b**",
		"| a |\n|---|\n| <script> |",
		"![a](data:text/html,<script>)",
		"> quote\n> more",
		"```\nunclosed",
		"1. one\n2. two\n",
		"~~~js\nx\n~~~",
		strings.Repeat("*", 200),
		strings.Repeat("[", 100) + "x",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 8<<10 {
			return
		}
		// No input panics, and none of them produces markup: the output is a
		// tree the h package escapes, so this asserts that nothing found a
		// way around it.
		out, err := renderNode(Markdown(src, MarkdownOpts{Images: true}))
		if err != nil {
			t.Fatal(err)
		}
		if stray := mdStrayTag(out); stray != "" {
			t.Fatalf("input %q produced the tag %q\n%s", src, stray, out)
		}
		low := strings.ToLower(out)
		for _, bad := range []string{`href="javascript:`, `href="data:`, `href="vbscript:`, `src="javascript:`, `src="data:`} {
			if strings.Contains(low, bad) {
				t.Fatalf("input %q produced %q\n%s", src, bad, out)
			}
		}
	})
}
