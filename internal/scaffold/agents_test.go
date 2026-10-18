package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/tokbudget"
)

func TestWriteAgents(t *testing.T) {
	dir := t.TempDir()
	res, err := WriteAgents(dir, Data{Name: "loja", Lang: "en"}, false)
	if err != nil || len(res) != 3 {
		t.Fatal(err, res)
	}
	if res[0].File != "AGENTS.md" || res[0].Action != UICreated || res[1].File != "CLAUDE.md" {
		t.Fatalf("first run = %v", res)
	}
	agents := filepath.Join(dir, "AGENTS.md")
	b, _ := os.ReadFile(agents)
	if !mdStamp.untouched(b) {
		t.Fatal("a fresh AGENTS.md must carry its own stamp")
	}
	if !strings.Contains(string(b), "trilha gen") || !strings.Contains(string(b), "trilha_gen.go") {
		t.Fatalf("AGENTS.md says nothing about the generator:\n%s", b)
	}
	// A second run changes nothing.
	if res, err := WriteAgents(dir, Data{Name: "loja"}, false); err != nil || res[0].Action != UIKept {
		t.Fatal(err, res)
	}
	// An older, untouched copy is refreshed in silence.
	os.WriteFile(agents, mdStamp.apply([]byte("# old\n")), 0o644)
	if res, err := WriteAgents(dir, Data{Name: "loja"}, false); err != nil || res[0].Action != UIUpdated {
		t.Fatal(err, res)
	}
	// A locally edited one is refused without force, and yields with it.
	b, _ = os.ReadFile(agents)
	os.WriteFile(agents, append(b, []byte("\n## our rules\n")...), 0o644)
	res, err = WriteAgents(dir, Data{Name: "loja"}, false)
	if !errors.Is(err, ErrAgentsModified) || res[0].Action != UIModified {
		t.Fatal(err, res)
	}
	if res, err := WriteAgents(dir, Data{Name: "loja"}, true); err != nil || res[0].Action != UIUpdated {
		t.Fatal(err, res)
	}
	// CLAUDE.md belongs to the project from the first line it gains.
	claude := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(claude, []byte("# mine\n"), 0o644)
	res, err = WriteAgents(dir, Data{Name: "loja"}, true)
	if err != nil || res[1].Action != UIKeptOwn {
		t.Fatal(err, res)
	}
	if b, _ := os.ReadFile(claude); string(b) != "# mine\n" {
		t.Fatalf("CLAUDE.md was overwritten: %s", b)
	}
	if _, err := WriteAgents(dir, Data{Name: "loja", Lang: "kl"}, false); err == nil {
		t.Fatal("unknown language must fail")
	}
}

func TestAgentsPortuguese(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteAgents(dir, Data{Name: "loja", Lang: "pt"}, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(b), "pasta") {
		t.Fatalf("pt version is not in Portuguese:\n%s", b)
	}
	en := t.TempDir()
	WriteAgents(en, Data{Name: "loja", Lang: "en"}, false)
	a, _ := os.ReadFile(filepath.Join(en, "AGENTS.md"))
	if string(a) == string(b) {
		t.Fatal("both languages produced the same file")
	}
}

// TestAgentsMdBudget is the spec 160 contract: the AGENTS.md the scaffold
// writes stays under the 2,500-token estimate, and it carries the four fixed
// sections — the map first, the recipes, the gates, and the narrow-reading
// rules. A bigger file is a regression even when it is right: the whole point
// of the v2 file is that the map moved into `trilha ctx`.
func TestAgentsMdBudget(t *testing.T) {
	for _, lang := range []string{"en", "pt"} {
		dir := t.TempDir()
		if _, err := WriteAgents(dir, Data{Name: "loja", Lang: lang}, false); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		if n := tokbudget.Estimate(string(b)); n > 2500 {
			t.Errorf("%s: AGENTS.md = %d est. tokens, over the 2,500 budget", lang, n)
		}
		for _, section := range []string{"## ", "ctx --json", "check"} {
			if !strings.Contains(string(b), section) {
				t.Errorf("%s: AGENTS.md lacks %q", lang, section)
			}
		}
	}
}

// Spec 178: the quality checklist goes with AGENTS.md, as a skill Claude Code
// finds in .claude/skills and any other agent reads by the link AGENTS.md
// gives. It belongs to the framework like AGENTS.md: refreshed when untouched,
// refused when edited. The stamp sits inside the front matter, which a skill
// has to open with.
func TestAgentsWritesTheQualitySkill(t *testing.T) {
	for _, lang := range []string{"en", "pt"} {
		dir := t.TempDir()
		res, err := WriteAgents(dir, Data{Name: "loja", Lang: lang}, false)
		if err != nil || len(res) != 3 || res[2].File != SkillPath || res[2].Action != UICreated {
			t.Fatalf("%s: %v %v", lang, err, res)
		}
		skill := filepath.Join(dir, filepath.FromSlash(SkillPath))
		b, err := os.ReadFile(skill)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if !strings.HasPrefix(s, "---\n# trilha agents ") || !strings.Contains(s, "\nname: trilha-quality\ndescription: ") {
			t.Fatalf("%s: the skill does not open with its front matter:\n%.200s", lang, s)
		}
		if !skillStamp.untouched(b) {
			t.Fatalf("%s: a fresh skill must carry its own stamp", lang)
		}
		// Loaded on demand, but loaded whole: it is budgeted like AGENTS.md.
		if n := tokbudget.Estimate(s); n > 2500 {
			t.Errorf("%s: the skill = %d est. tokens, over the 2,500 budget", lang, n)
		}
		// The ten rules, each with its section, and the name of the app.
		for _, want := range []string{"loja", "## 1.", "## 5.", "## 10.", "N+1", "OWASP", "NIST", "c.Asset", "cache.Do", "LIMIT", "trilha check"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s: the skill says nothing about %q", lang, want)
			}
		}
		agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		if !strings.Contains(string(agents), SkillPath) {
			t.Errorf("%s: AGENTS.md does not point at the skill", lang)
		}

		if res, err := WriteAgents(dir, Data{Name: "loja", Lang: lang}, false); err != nil || res[2].Action != UIKept {
			t.Fatalf("%s: second run %v %v", lang, err, res)
		}
		os.WriteFile(skill, skillStamp.apply([]byte("name: trilha-quality\ndescription: old\n---\n")), 0o644)
		if res, err := WriteAgents(dir, Data{Name: "loja", Lang: lang}, false); err != nil || res[2].Action != UIUpdated {
			t.Fatalf("%s: an untouched older skill must be refreshed: %v %v", lang, err, res)
		}
		b, _ = os.ReadFile(skill)
		os.WriteFile(skill, append(b, "\n## our rule\n"...), 0o644)
		if res, err := WriteAgents(dir, Data{Name: "loja", Lang: lang}, false); !errors.Is(err, ErrAgentsModified) || res[2].Action != UIModified {
			t.Fatalf("%s: an edited skill must be refused: %v %v", lang, err, res)
		}
	}
}
