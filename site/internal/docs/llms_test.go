package docs

import (
	"strings"
	"testing"
)

func TestLLMsIndex(t *testing.T) {
	got := LLMs("en", "/trilha")
	if !strings.HasPrefix(got, "# Trilha\n") || !strings.Contains(got, "\n> ") {
		t.Fatalf("llms.txt must open with the name and a one-line summary:\n%s", got[:200])
	}
	for _, p := range Pages("en") {
		line := "](/trilha" + p.Path() + "):"
		if !strings.Contains(got, line) {
			t.Errorf("index misses %s", p.Path())
		}
		if p.Description != "" && !strings.Contains(got, p.Description) {
			t.Errorf("index misses the description of %s", p.Path())
		}
	}
	if strings.Contains(got, "/trilha/pt/") {
		t.Error("the English index must not link into the Portuguese site")
	}
	pt := LLMs("pt", "/trilha")
	if !strings.Contains(pt, "](/trilha/pt/aprender):") {
		t.Errorf("pt index misses its own section:\n%s", pt[:300])
	}
	// Without a base path the links are still absolute paths of the site.
	if !strings.Contains(LLMs("en", ""), "](/learn):") {
		t.Error("empty base must still produce rooted links")
	}
}

func TestLLMsFull(t *testing.T) {
	got := LLMsFull("en", "")
	for _, p := range Pages("en") {
		if p.Body != "" && !strings.Contains(got, strings.TrimSpace(p.Body)) {
			t.Errorf("full text misses the body of %s", p.Path())
		}
	}
	// Code blocks survive whole: fenced Go is what the reader came for.
	if n := strings.Count(got, "```go"); n < 20 {
		t.Errorf("only %d go blocks in the full text", n)
	}
	if strings.Contains(got, "](/pt/") {
		t.Error("the English full text must not link into the Portuguese site")
	}
	// Links inside a body get the base path too.
	based := LLMsFull("en", "/trilha")
	if strings.Contains(based, "](/learn/") {
		t.Error("body links must carry the base path")
	}
	if !strings.Contains(based, "](/trilha/learn/") {
		t.Error("no body link carried the base path")
	}
}
