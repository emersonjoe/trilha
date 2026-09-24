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

// TestAgentsMatchesUsage keeps AGENTS.md from inventing commands: every
// command the file names has to be one the CLI's own usage names, in both
// languages. Since spec 160 shrank the file on purpose — the map moved into
// `trilha ctx` — the invariant is one-way: the file names a subset of the
// CLI, never a typo of one. A command the file must mention again fails
// TestAgentsMdBudget's section check instead.
func TestAgentsMatchesUsage(t *testing.T) {
	for _, l := range []string{"en", "pt"} {
		dir := t.TempDir()
		if _, err := scaffold.WriteAgents(dir, scaffold.Data{Name: "loja", Lang: l}, false); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range commands(string(b)) {
			if !strings.Contains(msgs["usage"][0], c) && !strings.Contains(msgs["usage"][1], c) {
				t.Errorf("AGENTS.md (%s) names %q, which the CLI does not have", l, c)
			}
		}
	}
}
