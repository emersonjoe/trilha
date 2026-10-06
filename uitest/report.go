package uitest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Failure is why a scenario stopped, in the shape an agent can act on: the
// step, the selector, what was expected, what was there, and what to change.
type Failure struct {
	Scenario  string
	Browser   string
	Attempt   int
	Step      string
	Selector  string
	Want      string
	Got       string
	Fix       string
	URL       string
	Console   []string
	ServerLog string
}

// Error makes a Failure usable as an error.
func (f *Failure) Error() string { return f.Report() }

// Report is the failure as text, one field per line, the long parts last.
func (f *Failure) Report() string {
	var b strings.Builder
	line := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%-9s %s\n", k+":", v)
		}
	}
	scenario := f.Scenario
	if f.Attempt > 0 {
		scenario += fmt.Sprintf(" (attempt %d of 2)", f.Attempt)
	}
	line("scenario", scenario)
	line("browser", f.Browser)
	line("step", f.Step)
	line("selector", f.Selector)
	line("expected", f.Want)
	line("got", f.Got)
	line("page", f.URL)
	line("fix", f.Fix)
	if len(f.Console) > 0 {
		b.WriteString("browser console:\n")
		for _, c := range f.Console {
			b.WriteString("  " + c + "\n")
		}
	}
	if f.ServerLog != "" {
		b.WriteString("server log (last lines):\n")
		for _, l := range strings.Split(f.ServerLog, "\n") {
			b.WriteString("  " + l + "\n")
		}
	}
	return b.String()
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// Write puts the report in dir/<scenario>-<browser>.txt (dir/<scenario>.txt
// without a browser) and returns the path.
func (f *Failure) Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := unsafeName.ReplaceAllString(f.Scenario, "_")
	if name == "" {
		name = "scenario"
	}
	if f.Browser != "" {
		name += "-" + unsafeName.ReplaceAllString(f.Browser, "_")
	}
	path := filepath.Join(dir, name+".txt")
	return path, os.WriteFile(path, []byte(f.Report()), 0o644)
}
