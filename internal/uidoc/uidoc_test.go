package uidoc

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite catalog.json")

// TestCatalogIsCurrent regenerates the catalogue and compares. It is the same
// deal as api/current.txt: the file is committed, and a doc comment that
// changed without it fails here instead of shipping a stale answer.
func TestCatalogIsCurrent(t *testing.T) {
	got, err := extract()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(got); err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile("catalog.json", buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !bytes.Equal(bytes.TrimSpace(buf.Bytes()), bytes.TrimSpace(JSON())) {
		t.Fatal("catalog.json is out of date; run: go test ./internal/uidoc -update")
	}
}

// TestEverySymbolIsDocumented: a symbol with no comment has no summary, and a
// catalogue with no summary is a catalogue nobody can use. A comment shared by
// a group counts, as long as it names the member.
func TestEverySymbolIsDocumented(t *testing.T) {
	// The source, not the shipped file: this is a rule about ui/, and it has
	// to hold before the catalogue is written.
	all, err := extract()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range all {
		if c.Summary == "" {
			t.Errorf("%s (%s) has no doc comment", c.Name, c.Group)
			continue
		}
		if !strings.Contains(c.Doc, c.Name) {
			t.Errorf("%s is documented by a comment that does not name it: %q", c.Name, c.Summary)
		}
	}
}

// TestWhatNeedsAnExampleHasOne is the rule of spec 063: a signature with more
// than two parameters, or with an options struct in it, is not obvious from
// the signature alone.
func TestWhatNeedsAnExampleHasOne(t *testing.T) {
	all, err := extract()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range all {
		if c.Kind != "func" || c.Example != "" {
			continue
		}
		if needsExample(c.Signature) {
			t.Errorf("%s needs an example in its doc comment: %s", c.Name, c.Signature)
		}
	}
}

// needsExample counts the parameters of a rendered signature.
func needsExample(sig string) bool {
	open := strings.Index(sig, "(")
	depth, end := 0, -1
	for i := open; i < len(sig); i++ {
		switch sig[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return false
	}
	params := strings.TrimSpace(sig[open+1 : end])
	if params == "" {
		return false
	}
	if strings.Contains(params, "Opts") {
		return true
	}
	n := 0
	for _, part := range strings.Split(params, ",") {
		n += len(strings.Fields(strings.TrimSpace(part))) - 1
		if strings.Contains(part, "...") {
			n -= 0
		}
	}
	return n > 2
}

// TestLookupAndSimilar is what the CLI does with what somebody typed.
func TestLookupAndSimilar(t *testing.T) {
	if *update {
		t.Skip("the embedded catalogue is the one from before this run")
	}
	for _, name := range []string{"Field", "field", "ui.Field", " ui.field "} {
		if c, ok := Lookup(name); !ok || c.Name != "Field" {
			t.Fatalf("Lookup(%q) = %v, %v", name, c.Name, ok)
		}
	}
	if _, ok := Lookup("Fieldy"); ok {
		t.Fatal("Fieldy should not exist")
	}
	got := strings.Join(Similar("dailog"), " ")
	if !strings.Contains(got, "Dialog") {
		t.Fatalf("Similar(\"dailog\") = %q", got)
	}
	if len(Similar("")) != 0 {
		t.Fatal("an empty name has no neighbours")
	}
}
