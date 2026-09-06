package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

// Module is the import path of every copy: the example moves out of the
// repository's module (which still contains it, and Go refuses the same
// package in two modules) into one of its own, as a user's project is.
func Module(sc Scenario) string { return "example.com/" + filepath.Base(sc.Example) }

// Build copies the scenario's example into dir as a module of its own,
// pointing at the repo, and applies the scenario's preparation. Imports of
// the example's old path are rewritten to the new module on the way.
func Build(repo string, sc Scenario, dir string, agents bool) error {
	src := filepath.Join(repo, filepath.FromSlash(sc.Example))
	old := "github.com/emersonjoe/trilha/" + sc.Example
	if err := copyTree(src, dir, old, Module(sc)); err != nil {
		return err
	}
	gomod := fmt.Sprintf("module %s\n\ngo 1.22\n\nrequire github.com/emersonjoe/trilha v0.0.0\n\nreplace github.com/emersonjoe/trilha => %s\n", Module(sc), repo)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		return err
	}
	// The AGENTS.md of spec 044, exactly as `trilha new --agents` writes it:
	// this is the one variable the ruler is measuring here.
	if agents {
		if _, err := scaffold.WriteAgents(dir, scaffold.Data{Name: filepath.Base(sc.Example)}, false); err != nil {
			return err
		}
	}
	if sc.Prepare != nil {
		return sc.Prepare(dir)
	}
	return nil
}

// BuildCLI compiles the trilha command of the repo into bin, so the agent's
// `trilha gen` is the one under test.
func BuildCLI(repo, bin string) error {
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	out, err := command(repo, "go", "build", "-o", filepath.Join(bin, "trilha"), "./cmd/trilha").CombinedOutput()
	if err != nil {
		return fmt.Errorf("build trilha: %v\n%s", err, out)
	}
	return nil
}

// Verify copies the hidden tests in and runs vet, test and the scenario's
// check. The output of the first failure comes back, trimmed to its tail:
// that is what a person reads to see why a run did not pass.
func Verify(ctx context.Context, dir string, sc Scenario) (bool, string) {
	for rel, src := range sc.Tests {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return false, err.Error()
		}
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(src, "MODULE", Module(sc))), 0o644); err != nil {
			return false, err.Error()
		}
	}
	for _, args := range [][]string{{"go", "vet", "./..."}, {"go", "test", "-count=1", "./..."}} {
		c := exec.CommandContext(ctx, args[0], args[1:]...)
		c.Dir = dir
		c.Env = append(os.Environ(), "TRILHA_LANG=en")
		if out, err := c.CombinedOutput(); err != nil {
			return false, tail(strings.Join(args, " ")+": "+err.Error()+"\n"+string(out), 4000)
		}
	}
	if sc.Check != nil {
		if err := sc.Check(dir); err != nil {
			return false, err.Error()
		}
	}
	return true, ""
}

// BrokenRuler reports whether a Verify failure on an *untouched* fixture is
// the harness and not the feature. The hidden tests talk to the app over HTTP
// (trilha.TestRequest) and name no symbol the agent has to write, so on a
// fixture nobody edited `go vet` has nothing to complain about: a failure
// there means the fixture stopped compiling with the hidden test beside it,
// and a run on it would measure the agent fixing the ruler. After the agent
// has written code, a vet failure is the agent's own and this says nothing.
func BrokenRuler(why string) bool { return strings.HasPrefix(why, "go vet ./...: ") }

// Vet is the sanity check of a fixture before any agent touches it.
func Vet(dir string) error {
	out, err := command(dir, "go", "vet", "./...").CombinedOutput()
	if err != nil {
		return fmt.Errorf("go vet on the untouched fixture: %v\n%s", err, out)
	}
	return nil
}

func command(dir, name string, args ...string) *exec.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = dir
	c.Cancel = func() error { cancel(); return c.Process.Kill() }
	return c
}

// Workspace copies the framework repository into work/trilha and makes the
// copy read-only. The fixture's replace points there, so the agent can read
// the framework the way it would read a module cache, and cannot change it:
// what the ruler measures is the framework as committed, not as patched by
// the agent to make a test pass.
func Workspace(repo, work string) (string, error) {
	dst := filepath.Join(work, "trilha")
	skip := map[string]bool{".git": true, "bench": true, ".trilha": true, "bin": true}
	err := filepath.WalkDir(repo, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repo, path)
		if d.IsDir() && rel != "." && skip[d.Name()] {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o444)
	})
	if err != nil {
		return "", err
	}
	return dst, lock(dst, true)
}

// lock makes every directory under dir read-only (or writable again, for
// the cleanup): a read-only file in a writable directory can still be
// replaced, which is what the agent would do.
func lock(dir string, ro bool) error {
	var dirs []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, path)
		}
		return err
	})
	if err != nil {
		return err
	}
	mode := os.FileMode(0o555)
	if !ro {
		mode = 0o755
	}
	// Deepest first when locking does not matter; unlocking must open the
	// parent before walking into the child, which WalkDir's order gives.
	for _, d := range dirs {
		if err := os.Chmod(d, mode); err != nil {
			return err
		}
	}
	return nil
}

// Unlock makes the workspace removable again.
func Unlock(work string) { _ = lock(filepath.Join(work, "trilha"), false) }

// copyTree copies src into dst, skipping what belongs to the repo and not to
// the project (the dev cache, compiled binaries) and rewriting the module
// path inside Go files.
func copyTree(src, dst, oldModule, newModule string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if d.IsDir() && (d.Name() == ".trilha" || d.Name() == "bin") && rel != "." {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".go") {
			b = bytes.ReplaceAll(b, []byte(oldModule), []byte(newModule))
		}
		return os.WriteFile(target, b, 0o644)
	})
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// summary is the first line of the agent's last message: enough to tell a
// "done" from a "could not" in the table's notes.
func summary(raw []byte) string {
	var r claudeResult
	if err := jsonUnmarshal(raw, &r); err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(r.Result), "\n")
	if len(line) > 160 {
		line = line[:160] + "…"
	}
	return line
}

func jsonUnmarshal(b []byte, v any) error {
	return json.Unmarshal(bytes.TrimSpace(b), v)
}
