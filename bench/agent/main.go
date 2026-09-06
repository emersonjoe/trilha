package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func main() {
	var (
		scenario = flag.String("scenario", "all", "scenario name, or all")
		runs     = flag.Int("runs", 3, "executions per scenario")
		model    = flag.String("model", "", "agent model (default: the agent's own default)")
		agent    = flag.String("agent", "claude", "agent command; must print the claude -p result JSON")
		maxTurns = flag.Int("max-turns", 80, "turn cap per run")
		timeout  = flag.Duration("timeout", 25*time.Minute, "wall-clock cap per run")
		out      = flag.String("out", "results.json", "where runs are appended")
		md       = flag.String("md", "RESULTS.md", "table rendered from -out")
		keep     = flag.Bool("keep", false, "keep the temporary copies and print their paths")
		dry      = flag.Bool("dry", false, "build every fixture and prove the hidden test fails; no agent")
		render   = flag.Bool("render", false, "only rewrite -md from -out")
		list     = flag.Bool("list", false, "print the scenarios and exit")
	)
	flag.Parse()
	if err := run(*scenario, *runs, *model, *agent, *maxTurns, *timeout, *out, *md, *keep, *dry, *render, *list); err != nil {
		fmt.Fprintln(os.Stderr, "agent:", err)
		os.Exit(1)
	}
}

func run(scenario string, runs int, model, agent string, maxTurns int, timeout time.Duration, out, md string, keep, dry, render, list bool) error {
	all := Scenarios()
	if list {
		for _, s := range all {
			fmt.Printf("%-14s %s (%s)\n", s.Name, s.Title, s.Example)
		}
		return nil
	}
	if render {
		r, err := Load(out)
		if err != nil {
			return err
		}
		return os.WriteFile(md, []byte(Render(r, all)), 0o644)
	}
	repo, err := repoRoot()
	if err != nil {
		return err
	}
	selected := all
	if scenario != "all" {
		s, ok := ScenarioByName(scenario)
		if !ok {
			return fmt.Errorf("unknown scenario %q; -list shows them", scenario)
		}
		selected = []Scenario{s}
	}
	work, err := os.MkdirTemp("", "trilha-agent-")
	if err != nil {
		return err
	}
	if !keep {
		defer os.RemoveAll(work)
		defer Unlock(work)
	}
	trilhaVersion := version(repo)
	if repo, err = Workspace(repo, work); err != nil {
		return err
	}
	bin := filepath.Join(work, "bin")
	if err := BuildCLI(repo, bin); err != nil {
		return err
	}
	if dry {
		for _, s := range selected {
			dir := filepath.Join(work, s.Name)
			if err := Build(repo, s, dir); err != nil {
				return err
			}
			if err := Vet(dir); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ok, why := Verify(ctx, dir, s)
			cancel()
			if ok {
				return fmt.Errorf("%s: the hidden test passes on the untouched fixture; it measures nothing", s.Name)
			}
			fmt.Printf("%-14s verify: FAIL (expected) — %s\n", s.Name, failLine(why))
			if keep {
				fmt.Printf("%-14s %s\n", "", dir)
			}
		}
		return nil
	}
	results, err := Load(out)
	if err != nil {
		return err
	}
	results.Trilha = trilhaVersion
	results.Agent = agentVersion(agent)
	results.Machine = machine()
	results.Date = time.Now().UTC().Format("2006-01-02")
	for _, s := range selected {
		for n := 1; n <= runs; n++ {
			dir := filepath.Join(work, fmt.Sprintf("%s-%d", s.Name, n))
			if err := Build(repo, s, dir); err != nil {
				return err
			}
			fmt.Printf("%-14s run %d/%d … ", s.Name, n, runs)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			u, raw, err := RunAgent(ctx, dir, s.Prompt, AgentOptions{Command: agent, Model: model, MaxTurns: maxTurns, Path: bin, Dirs: []string{repo}})
			if err != nil {
				cancel()
				return err
			}
			r := Run{Scenario: s.Name, N: n, At: time.Now().UTC(), Usage: u, Summary: summary(raw)}
			// The raw result lives next to the fixture: with -keep it is
			// there to read, without it it goes with the rest.
			_ = os.WriteFile(dir+".agent.json", raw, 0o644)
			if u.Error == "" {
				r.Passed, r.Verify = Verify(ctx, dir, s)
			}
			cancel()
			if u.Model != "" {
				results.Model = u.Model
			}
			results.Runs = append(results.Runs, r)
			if err := Save(out, results); err != nil {
				return err
			}
			status := "PASS"
			if !r.Passed {
				status = "FAIL"
			}
			if u.Error != "" {
				status = "ERROR: " + firstLine(u.Error)
			}
			fmt.Printf("%s  in=%d cache=%d out=%d turns=%d %.0fs\n", status, u.Input, u.CacheRead, u.Output, u.Turns, float64(u.DurationMs)/1000)
			if keep {
				fmt.Printf("%-14s %s\n", "", dir)
			}
			if u.Error != "" {
				return fmt.Errorf("agent error: %s", u.Error)
			}
		}
	}
	return os.WriteFile(md, []byte(Render(results, all)), 0o644)
}

// repoRoot finds the repository from wherever the command runs: this file
// lives in bench/agent, and bench has the go.mod with the replace.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "cmd", "trilha", "main.go")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("run from inside the trilha repository (cmd/trilha not found above %s)", dir)
		}
		dir = parent
	}
}

func version(repo string) string {
	out, err := exec.Command("git", "-C", repo, "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func agentVersion(agent string) string {
	out, err := exec.Command(agent, "--version").Output()
	if err != nil {
		return agent
	}
	return agent + " " + strings.TrimSpace(string(out))
}

func machine() string {
	return runtime.Version() + ", " + runtime.GOOS + "/" + runtime.GOARCH
}

// failLine picks the line that says what failed: the test name, or a
// compile error with its file, before falling back to the first line.
func failLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "--- FAIL") || strings.Contains(l, ".go:") {
			return l
		}
	}
	return firstLine(s)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
