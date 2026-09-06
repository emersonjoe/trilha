package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const successJSON = `{"type":"result","subtype":"success","is_error":false,"duration_ms":184000,"num_turns":23,"result":"Done.\nAdded the route.","total_cost_usd":0.42,"usage":{"input_tokens":1200,"cache_creation_input_tokens":30000,"cache_read_input_tokens":410000,"output_tokens":9800},"modelUsage":{"claude-sonnet-5":{}},"permission_denials":[{"tool_name":"Bash","tool_input":{"command":"go install ./..."}}]}`

const authErrorJSON = `{"type":"result","subtype":"success","is_error":true,"duration_ms":73,"num_turns":1,"result":"Failed to authenticate: OAuth session expired and could not be refreshed","total_cost_usd":0,"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"modelUsage":{},"permission_denials":[]}`

func TestParseResult(t *testing.T) {
	u, err := ParseResult([]byte(successJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := Usage{Input: 31200, CacheRead: 410000, Output: 9800, Turns: 23, DurationMs: 184000, CostUSD: 0.42, Model: "claude-sonnet-5", Denials: 1, Denied: []string{"Bash: go install ./..."}}
	if !reflect.DeepEqual(u, want) {
		t.Fatalf("got %+v\nwant %+v", u, want)
	}
	u, err = ParseResult([]byte(authErrorJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(u.Error, "authenticate") || u.Turns != 1 {
		t.Fatalf("auth failure must be recorded as Error: %+v", u)
	}
	if _, err := ParseResult([]byte("not json")); err == nil {
		t.Fatal("garbage must be an error")
	}
	if _, err := ParseResult([]byte(`{"type":"assistant"}`)); err == nil {
		t.Fatal("a non-result message must be an error")
	}
	if s := summary([]byte(successJSON)); s != "Done." {
		t.Fatalf("summary = %q", s)
	}
}

func TestMedian(t *testing.T) {
	for _, tc := range []struct {
		in   []float64
		want float64
	}{
		{nil, 0}, {[]float64{7}, 7}, {[]float64{9, 1, 5}, 5}, {[]float64{4, 1, 3, 2}, 2.5},
	} {
		if got := Median(tc.in); got != tc.want {
			t.Fatalf("Median(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRender(t *testing.T) {
	r := Results{Trilha: "v0.33.0", Agent: "claude 2.1", Model: "claude-sonnet-5", Machine: "go1.25 darwin/arm64", Date: "2026-09-06"}
	for i, in := range []int{3000, 1000, 2000} {
		r.Runs = append(r.Runs, Run{Scenario: "comments", N: i + 1, Usage: Usage{Input: in, CacheRead: 100000, Output: 500 + i, Turns: 10 + i, DurationMs: 60000, CostUSD: 0.5}, Passed: i != 1})
	}
	md := Render(r, Scenarios())
	for _, want := range []string{
		"| `comments` | 2.0k | 100.0k | 501 | 11 | 0 | 60 | 0.50 | 2/3 |",
		"Trilha v0.33.0, agente claude 2.1, modelo claude-sonnet-5",
		"## Metodologia", "### `pagination`", "### `cognito`",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("RESULTS.md lacks %q:\n%s", want, md)
		}
	}
	if !strings.Contains(Render(Results{}, Scenarios()), "Ainda sem medição") {
		t.Fatal("empty results must say so")
	}
	// Scenarios are the contract: four, in order, each with a hidden test.
	names := []string{"comments", "contact-form", "cognito", "pagination"}
	for i, s := range Scenarios() {
		if s.Name != names[i] || len(s.Tests) == 0 || s.Prompt == "" {
			t.Fatalf("scenario %d = %+v", i, s.Name)
		}
	}
}

// TestFixturesFailWithoutTheAgent is the proof that the ruler measures
// something: every fixture vets clean, and every hidden test fails on it.
func TestFixturesFailWithoutTheAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("builds four projects")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	for _, s := range Scenarios() {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "proj")
			if err := Build(repo, s, dir); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, ".trilha")); err == nil {
				t.Fatal("the dev cache must not be copied")
			}
			if err := Vet(dir); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			ok, why := Verify(ctx, dir, s)
			if ok {
				t.Fatal("the hidden test passes on the untouched fixture; it measures nothing")
			}
			if !strings.Contains(why, "FAIL") && !strings.Contains(why, "undefined") && !strings.Contains(why, "cannot") {
				t.Fatalf("unexpected verify output: %s", why)
			}
		})
	}
}
