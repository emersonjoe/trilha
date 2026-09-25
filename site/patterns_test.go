package main

import (
	"html"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/site/internal/demos"
	"github.com/emersonjoe/trilha/site/internal/docs"
	kit "github.com/emersonjoe/trilha/ui"
)

// TestPatternsPageBilingual is spec 163 on the site: every screen pattern is
// shown under a demo that exists in both locales and is embedded in a page of
// each, its section says the pattern in the page's language — every sentence
// of the pattern has its Portuguese — and carries the page.go whole.
func TestPatternsPageBilingual(t *testing.T) {
	shown := map[string]string{}
	for _, name := range demos.Names("en") {
		if p, ok := demos.PatternOf(name); ok {
			if other, dup := shown[p]; dup {
				t.Errorf("%s is shown under %s and %s", p, other, name)
			}
			shown[p] = name
		}
	}
	for _, p := range kit.Patterns() {
		demo, ok := shown[p.Name]
		if !ok {
			t.Errorf("%s is under no demo", p.Name)
			continue
		}
		for _, en := range append([]string{p.Summary}, p.A11y...) {
			if demos.PatternPT[en] == "" {
				t.Errorf("%s: no Portuguese for %q", p.Name, en)
			}
		}
		for _, l := range docs.Locales {
			if !embedded(l.Code, demo) {
				t.Errorf("%s: no %s page embeds the demo %s", p.Name, l.Code, demo)
			}
			out := html.UnescapeString(demos.Renderer(l.Code)(demo))
			heading, summary := "Use this pattern", p.Summary
			if l.Code == "pt" {
				heading, summary = "Usar este padrão", demos.PatternPT[p.Summary]
			}
			for _, want := range []string{heading, summary, "func Page(c *trilha.Ctx) (h.Node, error)", "ui." + p.Components[0], "trilha ui patterns " + p.Name} {
				if !strings.Contains(stripTags(out), want) {
					t.Errorf("%s (%s): the section lacks %q", p.Name, l.Code, want)
				}
			}
		}
	}
}

// embedded says whether some page of the locale shows the demo.
func embedded(locale, demo string) bool {
	for _, p := range docs.Pages(locale) {
		if strings.Contains(p.Body, "@demo "+demo+"\n") || strings.HasSuffix(strings.TrimSpace(p.Body), "@demo "+demo) {
			return true
		}
	}
	return false
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
