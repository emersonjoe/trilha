package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	frameworkui "github.com/emersonjoe/trilha/ui"
)

// TestPatternsJSONStable holds `trilha ui patterns --json` to a golden: the
// shape an agent parses changes only in a diff somebody reviews (make golden
// rewrites it).
func TestPatternsJSONStable(t *testing.T) {
	var b bytes.Buffer
	if err := writePatterns(&b, true, ""); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "ui-patterns.json")
	if *update {
		if err := os.WriteFile(golden, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.Bytes(), want) {
		t.Fatalf("trilha ui patterns --json changed shape; run make golden if on purpose")
	}
	var parsed []frameworkui.Pattern
	if err := json.Unmarshal(b.Bytes(), &parsed); err != nil || len(parsed) != len(frameworkui.Patterns()) {
		t.Fatalf("not a list of every pattern: %v", err)
	}
	for _, key := range []string{`"name"`, `"summary"`, `"components"`, `"snippet"`, `"data"`, `"a11y"`} {
		if !strings.Contains(b.String(), key) {
			t.Errorf("the JSON has no %s", key)
		}
	}
}

// One pattern by name answers the object alone, and a name nobody wrote says
// which ones exist.
func TestPatternsByName(t *testing.T) {
	var b bytes.Buffer
	if err := writePatterns(&b, true, "master-detail"); err != nil {
		t.Fatal(err)
	}
	var p frameworkui.Pattern
	if err := json.Unmarshal(b.Bytes(), &p); err != nil || p.Name != "master-detail" || p.Snippet == "" {
		t.Fatalf("pattern = %+v %v", p, err)
	}
	b.Reset()
	if err := writePatterns(&b, false, "master-detail"); err != nil || !strings.Contains(b.String(), "func Page(c *trilha.Ctx)") {
		t.Fatalf("the human form does not carry the snippet: %v\n%s", err, b.String())
	}
	err := writePatterns(&b, false, "nope")
	if err == nil || !strings.Contains(err.Error(), "list-with-filter") {
		t.Fatalf("err = %v", err)
	}
}
