// Package precommit holds the size limits a commit to a Trilha project
// passes: no file an agent writes over 5,000 tokens, no source file over 500
// lines. Small files are what an agent reads whole, reviews whole and changes
// without loading the rest of the project into its context.
package precommit

import (
	"bytes"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/tokbudget"
)

// The limits, and the two kinds of violation.
const (
	MaxTokens = 5000
	MaxLines  = 500
	Tokens    = "tokens"
	Lines     = "lines"
)

// Violation is one file over one limit.
type Violation struct {
	Path  string
	Kind  string // Tokens or Lines
	Count int
	Limit int
}

// Check applies the limits to files (path → content). generated, when not
// nil, marks more files as not written by an agent (.gitattributes).
func Check(files map[string][]byte, generated func(string) bool) []Violation {
	var out []Violation
	for p, b := range files {
		if notAuthored(p, b) || (generated != nil && generated(p)) {
			continue
		}
		if n := tokbudget.Estimate(string(b)); n > MaxTokens {
			out = append(out, Violation{p, Tokens, n, MaxTokens})
		}
		if isSource(p) {
			if n := countLines(b); n > MaxLines {
				out = append(out, Violation{p, Lines, n, MaxLines})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// lockfiles are written by package managers, never by hand.
var lockfiles = map[string]bool{
	"go.sum": true, "package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "bun.lockb": true, "Cargo.lock": true, "poetry.lock": true,
	"Pipfile.lock": true, "composer.lock": true, "Gemfile.lock": true, "uv.lock": true,
}

// generatedHeader is the Go convention for generated code, in any comment
// syntax, and the stamps of the files the framework writes (ui kit, agents).
var generatedHeader = regexp.MustCompile(`(?m)^(//|#|/\*|<!--|--)\s*(Code generated .*DO NOT EDIT|trilha (ui|agents|hooks) [0-9a-f]{16})`)

// notAuthored says the file is not one an agent writes: a lockfile, a golden,
// vendored code, a binary, or a file that declares itself generated.
func notAuthored(p string, b []byte) bool {
	if lockfiles[path.Base(p)] || strings.HasSuffix(p, ".golden") {
		return true
	}
	for _, dir := range strings.Split(path.Dir(p), "/") {
		if dir == "vendor" || dir == "node_modules" {
			return true
		}
	}
	head := b[:min(len(b), 8000)]
	return bytes.IndexByte(head, 0) >= 0 || generatedHeader.Match(head[:min(len(head), 1024)])
}

// sourceExt are the languages the line limit applies to. CSS and Markdown
// are not in it: one is style, the other documentation.
var sourceExt = map[string]bool{
	".go": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true,
	".sql": true, ".sh": true, ".py": true, ".rb": true, ".rs": true, ".java": true, ".kt": true,
	".swift": true, ".c": true, ".h": true, ".cc": true, ".cpp": true, ".php": true, ".cs": true,
}

// testDirs hold tests and their fixtures, exempt from the line limit.
var testDirs = map[string]bool{"test": true, "tests": true, "__tests__": true, "testdata": true, "e2e": true}

func isSource(p string) bool {
	base := path.Base(p)
	if !sourceExt[path.Ext(base)] || strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_") {
		return false
	}
	for _, dir := range strings.Split(path.Dir(p), "/") {
		if testDirs[dir] {
			return false
		}
	}
	return true
}

func countLines(b []byte) int {
	n := bytes.Count(b, []byte{'\n'})
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}

// Staged checks the files staged in the repository at root, as they are in
// the index — what the commit will carry, not the working tree.
func Staged(root string) ([]Violation, error) {
	return inIndex(root, "diff", "--cached", "--name-only", "-z", "--diff-filter=ACMR")
}

// Tracked checks every file in the index: the size of a project before it
// turns the hook on.
func Tracked(root string) ([]Violation, error) {
	return inIndex(root, "ls-files", "-z", "--cached")
}

// inIndex checks the files a git command lists, read from the index.
func inIndex(root string, list ...string) ([]Violation, error) {
	out, err := git(root, list...)
	if err != nil {
		return nil, err
	}
	names := splitZ(out)
	files := make(map[string][]byte, len(names))
	for _, n := range names {
		b, err := git(root, "show", ":"+n)
		if err != nil {
			return nil, err
		}
		files[n] = b
	}
	marked, err := markedGenerated(root, names)
	if err != nil {
		return nil, err
	}
	return Check(files, func(p string) bool { return marked[p] }), nil
}

// markedGenerated reads linguist-generated and linguist-vendored from the
// .gitattributes in the index: the way a project marks code it does not write.
func markedGenerated(root string, names []string) (map[string]bool, error) {
	marked := map[string]bool{}
	if len(names) == 0 {
		return marked, nil
	}
	args := append([]string{"check-attr", "-z", "--cached", "linguist-generated", "linguist-vendored", "--"}, names...)
	out, err := git(root, args...)
	if err != nil {
		return nil, err
	}
	f := splitZ(out)
	for i := 0; i+2 < len(f); i += 3 { // path, attribute, value
		if v := f[i+2]; v == "set" || v == "true" {
			marked[f[i]] = true
		}
	}
	return marked, nil
}

func git(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	return cmd.Output()
}

func splitZ(b []byte) []string {
	var out []string
	for _, s := range strings.Split(string(b), "\x00") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
