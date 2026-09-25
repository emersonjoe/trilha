package uidoc

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/ui"
)

// TestPatternsNameRealComponents holds each pattern to the catalog: every
// component it names is in `trilha ui components --json`, spelled the way
// the catalog spells it.
func TestPatternsNameRealComponents(t *testing.T) {
	for _, p := range ui.Patterns() {
		for _, name := range p.Components {
			c, ok := Lookup(name)
			if !ok || c.Name != name {
				t.Errorf("%s: %s is not in the catalog (did you mean %v?)", p.Name, name, Similar(name))
			}
		}
	}
}

// FindPattern and PatternMarkdown are what both MCP servers answer.
func TestFindPatternAndMarkdown(t *testing.T) {
	p, _, ok := FindPattern(" Async-Form ")
	if !ok || p.Name != "async-form" {
		t.Fatalf("FindPattern = %+v %v", p, ok)
	}
	md := PatternMarkdown(p)
	for _, want := range []string{"# async-form", "- ui.Swap", "## Data", "## Accessibility", "```go\n// Package asyncform"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
	if _, names, ok := FindPattern("carousel"); ok || len(names) != len(ui.Patterns()) {
		t.Fatalf("unknown pattern: ok=%v names=%v", ok, names)
	}
}
