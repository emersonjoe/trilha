package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha/internal/checkerr"
	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/openapi"
	"github.com/emersonjoe/trilha/internal/scan"
)

// Verifying a project used to be five commands, five outputs to read and five
// rounds of an agent's history resent. trilha check is the single gate: the
// same steps in the order that fails cheapest first, stopping at the first
// real failure, and every problem carries where it is and what resolves it.

// step is one gate, and what became of it.
type step struct {
	Tool   string `json:"tool"`
	Status string `json:"status"` // ok | fixed | failed | skipped | not run
}

// problem is one thing to fix. Fix is the sentence that resolves it; it is
// what saves the round trip that finding out would cost. Code is the stable
// name of this failure — E_DUPLICATE_ROUTE — which is what the JSON form
// carries with the hint and the doc page, so a machine reads the same repair
// a person reads.
type problem struct {
	Tool    string `json:"tool"`
	Code    string `json:"code,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// report is the whole run, and the shape of --json.
type report struct {
	OK       bool      `json:"ok"`
	Steps    []step    `json:"steps"`
	Problems []problem `json:"-"`
}

const (
	statusOK      = "ok"
	statusFixed   = "fixed"
	statusFailed  = "failed"
	statusSkipped = "skipped" // the step does not apply to this project
	statusNotRun  = "not run" // an earlier step failed
)

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, t("flag check json"))
	fix := fs.Bool("fix", false, t("flag check fix"))
	if err := fs.Parse(args); err != nil {
		return usageError{err}
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	r := runCheck(p, *fix)
	if *asJSON {
		b, err := json.MarshalIndent(r.machine(), "", "  ")
		if err != nil {
			return err
		}
		os.Stdout.Write(append(b, '\n'))
	} else {
		fmt.Print(r.text())
	}
	if !r.OK {
		return errors.New(t("check failed"))
	}
	return nil
}

// machine is the shape --json answers in (spec 161): one status, and every
// failure carrying the code that teaches it — the hint from the catalog and
// the page that holds the whole story. Steps stay in the human form; a
// machine that wants progress reads the failures, not the choreography.
func (r report) machine() machineReport {
	out := machineReport{Status: "ok"}
	for _, pb := range r.Problems {
		code := pb.Code
		hint := pb.Fix
		doc := ""
		if d, ok := checkerr.ByCode(code); ok {
			if hint == "" {
				hint = d.Fix
			}
			doc = d.Doc
		}
		out.Status = "fail"
		out.Failures = append(out.Failures, failure{
			Tool: pb.Tool, Code: code, File: pb.File, Line: pb.Line,
			Message: pb.Message, Hint: hint, Doc: doc,
		})
	}
	return out
}

type machineReport struct {
	Status   string    `json:"status"` // "ok" | "fail"
	Failures []failure `json:"failures"`
}

type failure struct {
	Tool    string `json:"tool"`
	Code    string `json:"code"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Doc     string `json:"doc,omitempty"`
}

// runCheck runs the gates in order and stops at the first failure: what comes
// after a broken build says nothing about the project.
func runCheck(p *project, fix bool) report {
	gates := []struct {
		tool string
		run  func(*project, bool) (string, []problem)
	}{
		{"gen", checkStepGen},
		{"gofmt", checkStepGofmt},
		{"vet", checkStepVet},
		{"test", checkStepTest},
		{"audit", checkStepAudit},
		{"openapi", checkStepOpenAPI},
		{"i18n", checkStepI18n},
		{"vuln", checkStepVuln},
		{"surface", checkStepSurface},
	}
	r := report{OK: true}
	failed := false
	for _, g := range gates {
		if failed {
			r.Steps = append(r.Steps, step{g.tool, statusNotRun})
			continue
		}
		status, probs := g.run(p, fix)
		r.Steps = append(r.Steps, step{g.tool, status})
		r.Problems = append(r.Problems, probs...)
		if status == statusFailed {
			failed = true
			r.OK = false
		}
	}
	if r.Problems == nil {
		r.Problems = []problem{}
	}
	return r
}

// text is the human form: one line per step, the problems of the one that
// failed underneath it.
func (r report) text() string {
	mark := map[string]string{statusOK: "✓", statusFixed: "✓", statusFailed: "✗", statusSkipped: "–", statusNotRun: "–"}
	var sb strings.Builder
	for _, s := range r.Steps {
		fmt.Fprintf(&sb, "%s %s", mark[s.Status], s.Tool)
		if s.Status != statusOK {
			fmt.Fprintf(&sb, " (%s)", s.Status)
		}
		sb.WriteString("\n")
		for _, pb := range r.Problems {
			if pb.Tool != s.Tool {
				continue
			}
			fmt.Fprintf(&sb, "    %s\n", pb.where())
			if pb.Fix != "" {
				fmt.Fprintf(&sb, "    → %s\n", pb.Fix)
			}
		}
	}
	if r.OK {
		sb.WriteString(t("check ok") + "\n")
	}
	return sb.String()
}

func (pb problem) where() string {
	switch {
	case pb.File != "" && pb.Line > 0:
		return fmt.Sprintf("%s:%d: %s", pb.File, pb.Line, pb.Message)
	case pb.File != "":
		return pb.File + ": " + pb.Message
	}
	return pb.Message
}

// checkStepGen is the first gate because it is the cheapest and because a
// route missing from trilha_gen.go is a 404 nobody explains.
func checkStepGen(p *project, fix bool) (string, []problem) {
	res, src, err := render(p, "")
	if err != nil {
		var errs scan.Errors
		if errors.As(err, &errs) {
			var out []problem
			for _, e := range errs {
				out = append(out, problem{Tool: "gen", Code: e.Code, File: e.File, Line: e.Line, Message: e.Msg, Fix: e.Fix})
			}
			return statusFailed, out
		}
		return statusFailed, []problem{{Tool: "gen", Message: err.Error(), Fix: t("fix gen")}}
	}
	_ = res
	path := filepath.Join(p.Root, gen.FileName)
	cur, err := os.ReadFile(path)
	if err == nil && string(cur) == string(src) {
		return statusOK, nil
	}
	if fix {
		if err := os.WriteFile(path, src, 0o644); err != nil {
			return statusFailed, []problem{{Tool: "gen", File: gen.FileName, Message: err.Error()}}
		}
		return statusFixed, nil
	}
	msg := t("gen stale")
	code := "E_GEN_STALE"
	if err != nil {
		msg = t("gen missing")
	}
	return statusFailed, []problem{{Tool: "gen", Code: code, File: gen.FileName, Message: msg, Fix: t("fix gen")}}
}

// checkStepGofmt keeps the diff about the change and not about the spacing.
func checkStepGofmt(p *project, fix bool) (string, []problem) {
	out, err := tool(p.Root, "gofmt", "-l", ".")
	if err != nil && out == "" {
		return statusSkipped, nil
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.Contains(filepath.ToSlash(l), "/testdata/") {
			continue
		}
		files = append(files, filepath.ToSlash(l))
	}
	if len(files) == 0 {
		return statusOK, nil
	}
	if fix {
		if _, err := tool(p.Root, append([]string{"gofmt", "-w"}, files...)...); err == nil {
			return statusFixed, nil
		}
	}
	var probs []problem
	for _, f := range files {
		probs = append(probs, problem{Tool: "gofmt", Code: "E_GOFMT", File: f, Message: t("gofmt unformatted"), Fix: t("fix gofmt")})
	}
	return statusFailed, probs
}

func checkStepVet(p *project, _ bool) (string, []problem) {
	out, err := tool(p.Root, "go", "vet", "./...")
	if err == nil {
		return statusOK, nil
	}
	return statusFailed, vetProblems(out)
}

// vetProblems turns vet's output into problems, naming the printf analyzer
// by name and the rest by the family code.
func vetProblems(out string) []problem {
	probs := positions("vet", out, t("fix vet"))
	for i := range probs {
		probs[i].Code = vetCode(probs[i].Message)
	}
	if len(probs) == 0 {
		probs = []problem{{Tool: "vet", Code: "E_VET", Message: firstLines(out, 3), Fix: t("fix vet")}}
	}
	return probs
}

// vetCode names the failure after the analyzer that wrote it: printf is the
// one an agent looks up by name — %d handed a string — and the rest answer
// under the family code.
func vetCode(msg string) string {
	if strings.HasPrefix(msg, "printf:") {
		return "E_VET_PRINTF"
	}
	return "E_VET"
}

// checkStepTest keeps only what says which test failed and where: the rest of
// the output is time and noise, and the reader pays for both.
func checkStepTest(p *project, _ bool) (string, []problem) {
	out, err := tool(p.Root, "go", "test", "./...")
	if err == nil {
		return statusOK, nil
	}
	var probs []problem
	name := ""
	for _, l := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(trimmed, "--- FAIL:"):
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "--- FAIL:"))
			if i := strings.Index(name, " "); i > 0 {
				name = name[:i]
			}
			probs = append(probs, problem{Tool: "test", Code: "E_TEST", Message: name, Fix: t("fix test")})
		case name != "" && strings.HasPrefix(l, " ") && strings.Contains(trimmed, ".go:"):
			file, line, msg := position(trimmed)
			if file == "" {
				continue
			}
			probs[len(probs)-1] = problem{Tool: "test", Code: "E_TEST", File: file, Line: line, Message: name + ": " + msg, Fix: t("fix test")}
		}
	}
	if len(probs) == 0 {
		probs = []problem{{Tool: "test", Code: "E_TEST", Message: firstLines(out, 3), Fix: t("fix test")}}
	}
	return statusFailed, probs
}

// checkStepAudit reuses trilha audit, minus the vulnerability scan: check runs
// on every turn and must not depend on the network.
func checkStepAudit(p *project, _ bool) (string, []problem) {
	var probs []problem
	for _, c := range runAudit(p, false) {
		if c.level == "critical" {
			probs = append(probs, problem{Tool: "audit", Code: "E_AUDIT", Message: c.title, Fix: c.hint})
		}
	}
	if len(probs) == 0 {
		return statusOK, nil
	}
	return statusFailed, probs
}

// checkStepOpenAPI only speaks when the project keeps the document: the title
// and the version come from the file itself, so the comparison is like for
// like. The document lives at the root, or under app/ when a route embeds it
// — mcp.FromRoutes reads it from there — and every copy has to be today's.
func checkStepOpenAPI(p *project, _ bool) (string, []problem) {
	paths := openAPIDocs(p.Root)
	if len(paths) == 0 {
		return statusSkipped, nil
	}
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		return statusSkipped, nil
	}
	var problems []problem
	for _, path := range paths {
		rel, _ := filepath.Rel(p.Root, path)
		rel = filepath.ToSlash(rel)
		fix := t("fix openapi")
		if rel != openapi.FileName {
			fix += " -o " + rel
		}
		cur, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, problem{Tool: "openapi", File: rel, Message: err.Error(), Fix: fix})
			continue
		}
		doc, err := openapi.Generate(p.Root, res, optionsOf(cur))
		if err != nil {
			problems = append(problems, problem{Tool: "openapi", File: rel, Message: err.Error(), Fix: fix})
			continue
		}
		if string(cur) != string(doc) {
			problems = append(problems, problem{Tool: "openapi", File: rel, Message: t("openapi stale"), Fix: fix})
		}
	}
	if len(problems) > 0 {
		return statusFailed, problems
	}
	return statusOK, nil
}

// openAPIDocs is the root document, if any, then every openapi.json under
// app/, sorted.
func openAPIDocs(root string) []string {
	var out []string
	if _, err := os.Stat(filepath.Join(root, openapi.FileName)); err == nil {
		out = append(out, filepath.Join(root, openapi.FileName))
	}
	_ = filepath.WalkDir(filepath.Join(root, "app"), func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == openapi.FileName {
			out = append(out, path)
		}
		return nil
	})
	return out
}

// optionsOf reads back what only the file knows: the title, the version and
// the server the document was written with.
func optionsOf(cur []byte) openapi.Options {
	var doc struct {
		Info struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(cur, &doc); err != nil {
		return openapi.Options{}
	}
	o := openapi.Options{Title: doc.Info.Title, Version: doc.Info.Version}
	if len(doc.Servers) > 0 {
		o.Server = doc.Servers[0].URL
	}
	return o
}

// tool runs a command in the project and returns everything it printed.
func tool(dir string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// positions turns "file:line:col: message" lines into problems, which is how
// every Go tool reports itself.
func positions(toolName, out, fix string) []problem {
	var probs []problem
	for _, l := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		file, line, msg := position(trimmed)
		if file == "" {
			continue
		}
		probs = append(probs, problem{Tool: toolName, File: file, Line: line, Message: msg, Fix: fix})
	}
	return probs
}

// position splits "file.go:12:3: message"; the column is dropped because no
// reader has ever needed it.
func position(s string) (file string, line int, msg string) {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) < 3 || !strings.HasSuffix(parts[0], ".go") {
		return "", 0, ""
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, ""
	}
	rest := parts[2]
	if len(parts) == 4 {
		if _, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
			rest = parts[3]
		} else {
			rest = parts[2] + ":" + parts[3]
		}
	}
	return strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(parts[0])), "./"), n, strings.TrimSpace(rest)
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "; ")
}

// checkStepVuln runs govulncheck, the one gate that needs the network's
// cache, so it only runs when the project asks for it: TRILHA_CHECK_VULN=1.
// Every finding carries the advisory's ID in the code — E_VULN_GO-2026-0123 —
// because the upgrade is the fix and the ID is how the advisory is found.
func checkStepVuln(p *project, _ bool) (string, []problem) {
	if os.Getenv("TRILHA_CHECK_VULN") != "1" {
		return statusSkipped, nil
	}
	out, err := tool(p.Root, "govulncheck", "-show", "verbose", "./...")
	if err != nil && out == "" {
		return statusSkipped, nil // not installed: the gate is opt-in, not required
	}
	probs := vulnProblems(out)
	if len(probs) == 0 {
		return statusOK, nil
	}
	return statusFailed, probs
}

// vulnProblems reads govulncheck's text output: every "Vulnerability" block
// names an advisory ID, which becomes the code, and the module line says what
// to upgrade.
func vulnProblems(out string) []problem {
	var probs []problem
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if m := vulnHead.FindStringSubmatch(l); m != nil {
			probs = append(probs, problem{Tool: "vuln", Code: "E_VULN_" + strings.ToUpper(m[1]), Message: l, Fix: t("fix vuln")})
			continue
		}
		if len(probs) > 0 && strings.Contains(l, "Found in") {
			probs[len(probs)-1].Message += " — " + l
		}
	}
	return probs
}

// vulnHead is the line that starts an advisory; the plain ID shows up again
// in the More info URL, which is not a new advisory.
var vulnHead = regexp.MustCompile(`^Vulnerability #\d+: (GO-\d{4}-\d{4,})`)

// checkStepSurface is the API-surface lock, and it only speaks where a
// surface exists: a project that carries api/current.txt is held to it, the
// same test `make api` answers.
func checkStepSurface(p *project, _ bool) (string, []problem) {
	if _, err := os.Stat(filepath.Join(p.Root, "api", "current.txt")); err != nil {
		return statusSkipped, nil
	}
	out, err := tool(p.Root, "go", "test", "-run", "TestSuperficiePublica", ".")
	if err == nil {
		return statusOK, nil
	}
	return statusFailed, surfaceProblems(out)
}

// surfaceProblems keeps the diff the test prints — the part a reader acts
// on — and names the whole failure E_API_SURFACE.
func surfaceProblems(out string) []problem {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if (strings.HasPrefix(l, "+") || strings.HasPrefix(l, "-")) && strings.Contains(l, "pkg ") {
			lines = append(lines, l)
			if len(lines) == 8 {
				break
			}
		}
	}
	msg := "the surface moved without api/current.txt following"
	if len(lines) > 0 {
		msg += " (diff: " + strings.Join(lines, " ") + ")"
	}
	return []problem{{Tool: "surface", Code: "E_API_SURFACE", Message: msg, Fix: t("fix surface")}}
}
