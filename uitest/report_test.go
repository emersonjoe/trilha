package uitest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/uitest"
)

// The report is what an agent reads instead of a screenshot: one field per
// line, the step, the selector, what was expected, what was there and what to
// change, and then the browser console and the server log.
func TestFailureReport(t *testing.T) {
	f := &uitest.Failure{
		Scenario: "TestX/sub case", Attempt: 2, Step: "WantFocus", Selector: "#email",
		Want: "the focus on an element matching the selector", Got: "<body>",
		Fix: "mark the field with ui.InvalidIf", URL: "http://127.0.0.1:1/p",
		Console: []string{"error: boom"}, ServerLog: "line 1\nline 2",
	}
	got := f.Report()
	for _, want := range []string{
		"scenario: TestX/sub case (attempt 2 of 2)\n", "step:     WantFocus\n", "selector: #email\n",
		"expected: the focus on", "got:      <body>\n", "page:     http://127.0.0.1:1/p\n",
		"fix:      mark the field", "browser console:\n  error: boom\n", "server log (last lines):\n  line 1\n  line 2\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains((&uitest.Failure{Step: "x"}).Report(), "selector:") {
		t.Error("an empty field is printed")
	}
	path, err := f.Write(t.TempDir())
	if err != nil || filepath.Base(path) != "TestX_sub_case.txt" {
		t.Fatalf("path %q, err %v", path, err)
	}
	if b, _ := os.ReadFile(path); string(b) != got {
		t.Fatalf("file:\n%s", b)
	}
}

// fakeT stands in for *testing.T, so a scenario can fail on purpose and the
// test can look at what the failure left.
type fakeT struct {
	name  string
	fatal string
	logs  []string
}

func (f *fakeT) Helper()                      {}
func (f *fakeT) Name() string                 { return f.name }
func (f *fakeT) Logf(format string, a ...any) { f.logs = append(f.logs, fmt.Sprintf(format, a...)) }
func (f *fakeT) Skipf(format string, a ...any) {
	f.fatal = "skip: " + fmt.Sprintf(format, a...)
	runtime.Goexit()
}
func (f *fakeT) Fatalf(format string, a ...any) {
	f.fatal = fmt.Sprintf(format, a...)
	runtime.Goexit()
}

// A scenario that fails twice leaves report/<scenario>.txt with the step,
// the selector, both values and the fix — and says where in the failure.
func TestFailureLeavesAReport(t *testing.T) {
	dir := app(t)
	reports := t.TempDir()
	ft := &fakeT{name: "TestOnPurpose"}
	done := make(chan struct{})
	go func() {
		defer close(done)
		uitest.RunWith(ft, dir, uitest.Config{Env: env, Timeout: 2 * time.Second, ReportDir: reports}, func(s *uitest.Session) {
			s.Navigate("/padroes/formulario")
			s.WantFocus("#email")
		})
	}()
	<-done
	path := filepath.Join(reports, "TestOnPurpose.txt")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no report (%v); the run said: %s", err, ft.fatal)
	}
	report := string(b)
	for _, want := range []string{"scenario: TestOnPurpose (attempt 2 of 2)", "step:     WantFocus", "selector: #email",
		"expected: the focus on an element matching the selector", "got:      <body>", "fix:      ", "/padroes/formulario",
		"server log (last lines):"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
	if !strings.Contains(ft.fatal, "report: "+path) {
		t.Errorf("the failure does not say where the report is: %s", ft.fatal)
	}
	if len(ft.logs) == 0 || !strings.Contains(ft.logs[0], "attempt 1 failed") {
		t.Errorf("the first attempt was not logged before the retry: %v", ft.logs)
	}
}

// No browser is a skip, and a failure only when the run says a browser is
// required — which is how CI keeps a missing Chrome from passing in silence.
func TestNoBrowserSkipsUnlessRequired(t *testing.T) {
	t.Setenv("UITEST_CHROME", filepath.Join(t.TempDir(), "no-chrome"))
	try := func(required string) string {
		t.Setenv("UITEST_REQUIRED", required)
		ft := &fakeT{name: "TestNoBrowser"}
		done := make(chan struct{})
		go func() {
			defer close(done)
			uitest.RequireBrowser(ft)
		}()
		<-done
		return ft.fatal
	}
	if got := try(""); !strings.HasPrefix(got, "skip: ") || !strings.Contains(got, "UITEST_CHROME") {
		t.Errorf("without a browser: %q, want a skip that names UITEST_CHROME", got)
	}
	if got := try("1"); strings.HasPrefix(got, "skip: ") || !strings.Contains(got, "UITEST_REQUIRED=1") {
		t.Errorf("required and missing: %q, want a failure", got)
	}
}
