// Command agent measures what a coding agent spends to add a feature to a
// Trilha project: tokens in, tokens out, turns, time, and whether the hidden
// test passes afterwards. It is the ruler of Fase 5: every item there is
// compared against the numbers this program wrote before it.
//
// The runtime and the CLI never import this package; it lives in the bench
// module, which is separate on purpose.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Usage is what one agent run cost. Input is what was sent fresh (including
// what was written to the cache); CacheRead is what was read back from it.
type Usage struct {
	Input      int     `json:"input"`
	CacheRead  int     `json:"cache_read"`
	Output     int     `json:"output"`
	Turns      int     `json:"turns"`
	DurationMs int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd"`
	Model      string  `json:"model"`
	Denials    int     `json:"denials"`
	Error      string  `json:"error,omitempty"`
}

// claudeResult is the shape of `claude -p --output-format json`: one object,
// the last message plus the usage. Fields we do not read are left out.
type claudeResult struct {
	Type       string  `json:"type"`
	IsError    bool    `json:"is_error"`
	Result     string  `json:"result"`
	NumTurns   int     `json:"num_turns"`
	DurationMs int64   `json:"duration_ms"`
	TotalCost  float64 `json:"total_cost_usd"`
	Usage      struct {
		Input       int `json:"input_tokens"`
		CacheCreate int `json:"cache_creation_input_tokens"`
		CacheRead   int `json:"cache_read_input_tokens"`
		Output      int `json:"output_tokens"`
	} `json:"usage"`
	ModelUsage        map[string]json.RawMessage `json:"modelUsage"`
	PermissionDenials []json.RawMessage          `json:"permission_denials"`
}

// ParseResult reads the agent's JSON. An authentication failure is a result
// too: it is reported as Error, not as a parse problem, so the run is
// recorded and the table says why it is empty.
func ParseResult(b []byte) (Usage, error) {
	var r claudeResult
	if err := json.Unmarshal(bytes.TrimSpace(b), &r); err != nil {
		return Usage{}, fmt.Errorf("agent output is not the result JSON: %w\n%s", err, head(b))
	}
	if r.Type != "result" {
		return Usage{}, fmt.Errorf("agent output type %q, want \"result\"\n%s", r.Type, head(b))
	}
	u := Usage{
		Input:      r.Usage.Input + r.Usage.CacheCreate,
		CacheRead:  r.Usage.CacheRead,
		Output:     r.Usage.Output,
		Turns:      r.NumTurns,
		DurationMs: r.DurationMs,
		CostUSD:    r.TotalCost,
		Denials:    len(r.PermissionDenials),
	}
	models := make([]string, 0, len(r.ModelUsage))
	for m := range r.ModelUsage {
		models = append(models, m)
	}
	sort.Strings(models)
	u.Model = strings.Join(models, "+")
	if r.IsError {
		u.Error = r.Result
	}
	return u, nil
}

// AgentOptions is how the agent is invoked. Everything that could leak the
// user's own setup into the measurement is switched off: no MCP servers, no
// plugins or hooks, no user memory. Only what is in the project counts.
type AgentOptions struct {
	Command  string // "claude" by default
	Model    string // empty: the CLI's default; recorded from the result anyway
	MaxTurns int
	Path     string // prepended to PATH so `trilha` resolves
}

// allowedTools is what the agent may run without asking. In -p mode a tool
// outside this list is denied, which shows up as a denial in the table.
var allowedTools = []string{
	"Bash(go:*)", "Bash(gofmt:*)", "Bash(trilha:*)", "Bash(make:*)",
	"Bash(ls:*)", "Bash(cat:*)", "Bash(head:*)", "Bash(tail:*)", "Bash(sed:*)",
	"Bash(grep:*)", "Bash(find:*)", "Bash(wc:*)", "Bash(tree:*)", "Bash(mkdir:*)",
}

// RunAgent runs one task in dir and returns what it cost plus the raw JSON.
func RunAgent(ctx context.Context, dir, prompt string, o AgentOptions) (Usage, []byte, error) {
	cmd := or(o.Command, "claude")
	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--permission-mode", "acceptEdits",
		"--allowedTools", strings.Join(allowedTools, ","),
		"--strict-mcp-config",
		"--disable-slash-commands",
		"--no-session-persistence",
		"--setting-sources", "project",
		"--max-turns", fmt.Sprint(orInt(o.MaxTurns, 80)),
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	c := exec.CommandContext(ctx, cmd, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "PATH="+o.Path+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	err := c.Run()
	if out.Len() == 0 {
		if err == nil {
			err = errors.New("no output")
		}
		return Usage{}, nil, fmt.Errorf("%s: %w\n%s", cmd, err, head(errb.Bytes()))
	}
	u, perr := ParseResult(out.Bytes())
	if perr != nil {
		return Usage{}, out.Bytes(), perr
	}
	return u, out.Bytes(), nil
}

func head(b []byte) string {
	if len(b) > 600 {
		b = b[:600]
	}
	return string(b)
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func orInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
