package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

var citedCmd = regexp.MustCompile(`trilha ([a-z]+)`)

// commands returns the trilha subcommands a text names, sorted and deduplicated.
func commands(text string) []string {
	set := map[string]bool{}
	for _, m := range citedCmd.FindAllStringSubmatch(text, -1) {
		set[m[1]] = true
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// TestAgentsMatchesUsage keeps AGENTS.md from aging: it names exactly the
// commands the CLI's own usage names, in both languages. A command added,
// renamed or dropped fails here until the file follows.
func TestAgentsMatchesUsage(t *testing.T) {
	for i, l := range []string{"en", "pt"} {
		dir := t.TempDir()
		if _, err := scaffold.WriteAgents(dir, scaffold.Data{Name: "loja", Lang: l}, false); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(commands(string(b)), " ")
		want := strings.Join(commands(msgs["usage"][i]), " ")
		if got != want {
			t.Errorf("AGENTS.md (%s) names\n  %s\nusage names\n  %s", l, got, want)
		}
	}
}
