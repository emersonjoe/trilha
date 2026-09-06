package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAgents(t *testing.T) {
	dir := t.TempDir()
	res, err := WriteAgents(dir, Data{Name: "loja", Lang: "en"}, false)
	if err != nil || len(res) != 2 {
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
