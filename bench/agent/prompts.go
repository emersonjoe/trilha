package main

import (
	"embed"
	"fmt"
	"strings"
)

// The prompts are part of the contract: they do not change without reopening
// the historical series (PLANO-TOKENS-70 §1, rule 1). They live in files, one
// per side of each scenario, and CHECKSUMS.txt pins their bytes — the test
// refuses to run a ruler whose contract moved.

//go:embed prompts
var promptFS embed.FS

// mustPrompt reads a frozen prompt. A missing file is a broken ruler, and a
// broken ruler fails at startup, not three hours into a measurement.
func mustPrompt(rel string) string {
	b, err := promptFS.ReadFile(rel)
	if err != nil {
		panic("bench/agent: frozen prompt " + rel + " is gone: " + err.Error())
	}
	return string(b)
}

// promptFiles lists every file under prompts/, sorted: the .md prompts and
// CHECKSUMS.txt last, which is how the freeze test tells a missing pin from a
// stray file.
func promptFiles() ([]string, error) {
	entries, err := promptFS.ReadDir("prompts")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		out = append(out, "prompts/"+e.Name())
	}
	return out, nil
}

// checksums reads CHECKSUMS.txt into a map of file name -> sha256 hex. The
// format is the sha256sum one, "<hex>  <name>", so `sha256sum -c` from
// bench/agent checks the same file by hand.
func checksums() (map[string]string, error) {
	b, err := promptFS.ReadFile("prompts/CHECKSUMS.txt")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hex, name, found := strings.Cut(line, "  ")
		if !found {
			return nil, fmt.Errorf("prompts/CHECKSUMS.txt: line %q is not \"<hex>  <name>\"", line)
		}
		out["prompts/"+strings.TrimSpace(name)] = strings.TrimSpace(hex)
	}
	return out, nil
}
