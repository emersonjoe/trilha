// Package uitest drives a Trilha app in a real browser, for what only a
// browser sees: the focus moving to the field that failed, a fragment swapped
// in place, an upload bar filling, an island mounting, a tooltip opening from
// the keyboard.
//
// What the served HTML already proves — the CSRF token in a form, the nonce on
// a script, a golden of the markup — belongs to trilha.PageSnapshot, which
// needs no browser and runs in every go test. This module is the other half,
// and it is a module of its own so that chromedp never becomes a dependency of
// the framework or of an app.
//
//	func TestLogin(t *testing.T) {
//		uitest.Run(t, "..", func(s *uitest.Session) {
//			s.Navigate("/entrar")
//			s.Fill("#email", "admin@example.com")
//			s.Fill("#password", "a-password")
//			s.Click("button[type=submit]")
//			s.WantURL("/")
//		})
//	}
//
// Run builds the app once per directory, starts the binary on a free port —
// the program that goes to production, not the dev server with its reload —
// and gives the scenario a fresh headless Chrome. Nothing here takes a
// screenshot or sleeps: every step waits for its condition, up to 30 seconds.
// A scenario that fails runs once more from scratch, with the first failure in
// the log; failing twice, it writes report/<scenario>.txt with the step, the
// selector, what was expected, what was there and what to change, and fails
// the test.
//
// Without Chrome the scenarios skip, unless UITEST_REQUIRED=1 is set — which
// is how CI turns a missing browser into a failure. UITEST_CHROME points at a
// browser that is not in the usual places.
package uitest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// T is the part of *testing.T that Run uses.
type T interface {
	Helper()
	Name() string
	Logf(format string, args ...any)
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// Config adjusts a run. The zero value is the default.
type Config struct {
	// Env is added to the app's environment, as "NAME=value": the secret,
	// the administrator, TRILHA_ENV. PORT is set by Run.
	Env []string
	// Timeout is how long one step waits for its condition. Default 30s.
	Timeout time.Duration
	// ReportDir is where a failure report is written. Default "report".
	ReportDir string
}

// Run runs the scenario against the app in appDir. See the package comment.
func Run(t T, appDir string, fn func(s *Session)) { RunWith(t, appDir, Config{}, fn) }

// RunWith is Run with a Config.
func RunWith(t T, appDir string, cfg Config, fn func(s *Session)) {
	t.Helper()
	chrome := RequireBrowser(t)
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.ReportDir == "" {
		cfg.ReportDir = "report"
	}
	bin, err := build(appDir)
	if err != nil {
		t.Fatalf("uitest: building %s: %v", appDir, err)
	}
	var first *Failure
	for attempt := 1; attempt <= 2; attempt++ {
		f := runOnce(t, chrome, bin, appDir, cfg, fn)
		if f == nil {
			if first != nil {
				t.Logf("uitest: passed on the second attempt; the first failed with:\n%s", first.Report())
			}
			return
		}
		f.Scenario, f.Attempt = t.Name(), attempt
		if attempt == 1 {
			first = f
			t.Logf("uitest: attempt 1 failed, running again with a fresh server and browser:\n%s", f.Report())
			continue
		}
		path, werr := f.Write(cfg.ReportDir)
		if werr != nil {
			path = "(not written: " + werr.Error() + ")"
		}
		t.Fatalf("%s\nreport: %s", f.Report(), path)
	}
}

// RequireBrowser returns the Chrome the scenarios will drive, or skips the
// test when there is none — or fails it, with UITEST_REQUIRED=1.
func RequireBrowser(t T) string {
	t.Helper()
	if p := BrowserPath(); p != "" {
		return p
	}
	msg := "uitest: no Chrome or Chromium found (set UITEST_CHROME to its path)"
	if os.Getenv("UITEST_REQUIRED") == "1" {
		t.Fatalf("%s, and UITEST_REQUIRED=1", msg)
	}
	t.Skipf("%s", msg)
	return ""
}

// BrowserPath is the browser Run would use, or "".
func BrowserPath() string {
	if p := os.Getenv("UITEST_CHROME"); p != "" {
		return p
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "headless-shell"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if runtime.GOOS == "darwin" {
		for _, p := range []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

type built struct {
	once sync.Once
	bin  string
	err  error
}

var builds sync.Map // abs dir → *built

// build compiles the app once per process: every scenario starts the same
// binary, and a fresh process gives each one fresh in-memory stores.
func build(appDir string) (string, error) {
	dir, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	v, _ := builds.LoadOrStore(dir, &built{})
	b := v.(*built)
	b.once.Do(func() {
		out, err := os.MkdirTemp("", "uitest-bin-")
		if err != nil {
			b.err = err
			return
		}
		b.bin = filepath.Join(out, "app")
		cmd := exec.Command("go", "build", "-o", b.bin, ".")
		cmd.Dir = dir
		if msg, err := cmd.CombinedOutput(); err != nil {
			b.err = fmt.Errorf("%v\n%s", err, msg)
		}
	})
	return b.bin, b.err
}

// runOnce starts the server and the browser, runs the scenario and turns what
// stopped it into a Failure.
func runOnce(t T, chrome, bin, appDir string, cfg Config, fn func(s *Session)) (fail *Failure) {
	port, err := freePort()
	if err != nil {
		return &Failure{Step: "start", Got: err.Error(), Fix: "free a local port for the app"}
	}
	var log syncBuffer
	cmd := exec.Command(bin)
	cmd.Dir = appDir
	cmd.Env = append(append(os.Environ(), cfg.Env...), "PORT="+port, "ADDR=127.0.0.1:"+port)
	cmd.Stdout, cmd.Stderr = &log, &log
	if err := cmd.Start(); err != nil {
		return &Failure{Step: "start", Got: err.Error(), Fix: "check the app builds and runs with go run ."}
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if fail != nil {
			fail.ServerLog = tail(log.String(), 30)
		}
	}()
	base := "http://127.0.0.1:" + port
	if err := waitUp(base, cfg.Timeout); err != nil {
		return &Failure{Step: "start", Want: "the app answering at " + base, Got: err.Error(),
			Fix: "read the server log below: the app did not start (a missing TRILHA_SECRET in prod, a port in use)"}
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(chrome), chromedp.WindowSize(1280, 900))
	if os.Geteuid() == 0 {
		opts = append(opts, chromedp.NoSandbox)
	}
	actx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	// A Chrome newer than the protocol this chromedp knows sends events it
	// cannot decode — harmless, and noise in every test log.
	bctx, cancelBrowser := chromedp.NewContext(actx, chromedp.WithErrorf(func(string, ...any) {}))
	defer cancelBrowser()

	s := &Session{BaseURL: base, ctx: bctx, timeout: cfg.Timeout}
	chromedp.ListenTarget(bctx, func(ev any) {
		switch ev := ev.(type) {
		case *cdpruntime.EventConsoleAPICalled:
			if ev.Type == cdpruntime.APITypeError || ev.Type == cdpruntime.APITypeWarning {
				var parts []string
				for _, a := range ev.Args {
					parts = append(parts, strings.Trim(string(a.Value), `"`)+a.Description)
				}
				s.console(string(ev.Type) + ": " + strings.Join(parts, " "))
			}
		case *cdpruntime.EventExceptionThrown:
			s.console("exception: " + ev.ExceptionDetails.Error())
		}
	})
	// Focus emulation: a headless tab is never the focused window, and without
	// it focus and focusin do not fire — the keyboard scenarios would test a
	// browser nobody uses. Reduced motion: the kit then swaps without a view
	// transition, whose overlay a headless tab may never finish drawing; a
	// scenario checks where the page ends, not the animation on the way.
	if err := chromedp.Run(bctx, emulation.SetFocusEmulationEnabled(true),
		emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}})); err != nil {
		return &Failure{Step: "browser", Got: err.Error(), Fix: "check Chrome starts headless on this machine (UITEST_CHROME)"}
	}

	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if f, ok := r.(*Failure); ok {
			fail = f
		} else {
			fail = &Failure{Step: s.step, Got: fmt.Sprint(r), Fix: "a panic in the scenario itself"}
		}
		fail.URL = s.url()
		fail.Console = s.Console()
	}()
	fn(s)
	return nil
}

func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

// waitUp waits for the app to answer anything at all: the health probe may be
// closed to strangers, and any status means the server is listening.
func waitUp(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	var last error
	for time.Now().Before(deadline) {
		res, err := client.Get(base + "/_trilha/health/live")
		if err == nil {
			res.Body.Close()
			return nil
		}
		last = err
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("not up after %s: %v", timeout, last)
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *syncBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *syncBuffer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// Session is one browser tab on the app under test. Every method waits for
// its condition and, when it does not come, stops the scenario with a Failure
// that says what to change — there is no error to check.
type Session struct {
	// BaseURL is the app's address, http://127.0.0.1:<port>.
	BaseURL string

	ctx     context.Context
	timeout time.Duration
	step    string

	mu   sync.Mutex
	logs []string
}

func (s *Session) console(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, line)
}

// Console is what the page wrote as an error or warning so far, and the
// exceptions it threw: a refused inline script shows up here.
func (s *Session) Console() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.logs...)
}

// Fail stops the scenario with f. Use it for a check of your own, so the
// report has the same shape as the ones Session writes.
func (s *Session) Fail(f Failure) {
	if f.Step == "" {
		f.Step = s.step
	}
	panic(&f)
}

func (s *Session) run(step, sel, fix string, actions ...chromedp.Action) {
	s.step = step
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	if err := chromedp.Run(ctx, actions...); err != nil {
		got := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			got = "nothing happened in " + s.timeout.String()
		}
		s.Fail(Failure{Step: step, Selector: sel, Want: "the action to complete", Got: got, Fix: fix})
	}
}

// eval runs a JavaScript expression and decodes its value into out.
func (s *Session) eval(js string, out any) error {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	return chromedp.Run(ctx, chromedp.Evaluate(js, out, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
}

// until polls probe until it says ok, or fails the step with what it saw last.
func (s *Session) until(step, sel, want, fix string, probe func() (got string, ok bool)) {
	s.step = step
	deadline := time.Now().Add(s.timeout)
	var got string
	for {
		var ok bool
		got, ok = probe()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.Fail(Failure{Step: step, Selector: sel, Want: want, Got: got, Fix: fix})
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// probeJS evaluates js — which sees the element as el, or null — and returns
// its string answer.
func (s *Session) probeJS(sel, js string) (string, error) {
	var out string
	err := s.eval(`(() => { const el = document.querySelector(`+quote(sel)+`); `+js+` })()`, &out)
	return out, err
}

const describeJS = `const d = (n) => { if (!n || n === document.body) return "<body>"; ` +
	`let s = n.tagName.toLowerCase(); if (n.id) s += "#" + n.id; ` +
	`const nm = n.getAttribute("name"); if (nm) s += "[name=" + nm + "]"; ` +
	`const t = (n.innerText || "").trim().slice(0, 40); if (t) s += " \"" + t + "\""; return s; };`

// Navigate loads path (relative to the app) and waits for the document to
// finish loading.
func (s *Session) Navigate(path string) {
	s.run("Navigate "+path, "", "check the route exists — the server log is below",
		chromedp.Navigate(s.BaseURL+path))
	s.until("Navigate "+path, "", "document.readyState complete", "the page never finished loading", func() (string, bool) {
		var st string
		if err := s.eval(`document.readyState`, &st); err != nil {
			return err.Error(), false
		}
		return st, st == "complete"
	})
}

// WaitVisible waits for an element matching sel to be on the page and shown.
func (s *Session) WaitVisible(sel string) {
	s.until("WaitVisible", sel, "an element matching the selector, visible",
		"check the selector against the served HTML (trilha.CapturePage), or what hides it (hidden, display:none)",
		func() (string, bool) {
			got, err := s.probeJS(sel, `if (!el) return "no element"; const r = el.getBoundingClientRect(); `+
				`const st = getComputedStyle(el); return (r.width > 0 || r.height > 0) && st.visibility !== "hidden" `+
				`&& st.display !== "none" ? "visible" : "present but hidden";`)
			if err != nil {
				return err.Error(), false
			}
			return got, got == "visible"
		})
}

// Click waits for sel to be visible and clicks the middle of it with the
// mouse — after checking nothing covers that point, which is what a person
// would hit instead.
//
// Every action finds its element again with querySelector, at the moment it
// acts: a swap replaces the nodes, and a node remembered from before the swap
// is not on the page anymore.
func (s *Session) Click(sel string) {
	s.WaitVisible(sel)
	s.step = "Click"
	var pt struct {
		X, Y float64
		OK   bool
		Top  string
	}
	if err := s.eval(`(() => { `+describeJS+` const el = document.querySelector(`+quote(sel)+`); `+
		`el.scrollIntoView({block: "center", inline: "center"}); const r = el.getBoundingClientRect(); `+
		`const x = r.left + r.width / 2, y = r.top + r.height / 2; const top = document.elementFromPoint(x, y); `+
		`return {X: x, Y: y, OK: !!top && (top === el || el.contains(top)), Top: d(top)}; })()`, &pt); err != nil {
		s.Fail(Failure{Step: "Click", Selector: sel, Want: "an element to click", Got: err.Error()})
	}
	if !pt.OK {
		s.Fail(Failure{Step: "Click", Selector: sel, Want: "the element on top at its own middle", Got: "covered by " + pt.Top,
			Fix: "something is drawn over it (a dialog, a toast, a sticky header): close it first, or check the z-index"})
	}
	s.run("Click", sel, "the click did not go through", chromedp.MouseClickXY(pt.X, pt.Y))
}

// Fill replaces the value of a field by typing, so the page sees the same
// input events a person would cause.
func (s *Session) Fill(sel, value string) {
	s.WaitVisible(sel)
	s.step = "Fill"
	if got, err := s.probeJS(sel, `if (!("value" in el) || el.disabled || el.readOnly) return "not editable"; `+
		`el.focus(); el.value = ""; el.dispatchEvent(new Event("input", {bubbles: true})); `+
		`return document.activeElement === el ? "" : "did not take the focus";`); err != nil || got != "" {
		if err != nil {
			got = err.Error()
		}
		s.Fail(Failure{Step: "Fill", Selector: sel, Want: "an editable field with the focus", Got: got,
			Fix: "the field is disabled, readonly or not an input"})
	}
	s.run("Fill", sel, "the field did not take the keys: a script moved the focus", chromedp.KeyEvent(value))
}

// Focus moves the focus to sel, as Tab would when it got there.
func (s *Session) Focus(sel string) {
	s.WaitVisible(sel)
	s.step = "Focus"
	if got, err := s.probeJS(sel, `el.focus(); return document.activeElement === el ? "" : "did not take the focus";`); err != nil || got != "" {
		if err != nil {
			got = err.Error()
		}
		s.Fail(Failure{Step: "Focus", Selector: sel, Want: "the focus on the element", Got: got,
			Fix: "the element cannot take focus: it needs to be a link, a control or have tabindex"})
	}
}

var keys = map[string]string{
	"Enter": kb.Enter, "Escape": kb.Escape, "Tab": kb.Tab, "Space": " ", "Backspace": kb.Backspace,
	"ArrowUp": kb.ArrowUp, "ArrowDown": kb.ArrowDown, "ArrowLeft": kb.ArrowLeft, "ArrowRight": kb.ArrowRight,
	"Home": kb.Home, "End": kb.End,
}

// Press sends a key to whatever has the focus: "Enter", "Escape", "Tab",
// "Space", the arrows, or a literal text.
func (s *Session) Press(key string) {
	k, ok := keys[key]
	if !ok {
		k = key
	}
	s.run("Press "+key, "", "the key reached no element: check the focus first with WantFocus", chromedp.KeyEvent(k))
}

// Upload sets the files of a file input, as choosing them in the dialog does.
func (s *Session) Upload(sel string, paths ...string) {
	abs := make([]string, 0, len(paths))
	for _, p := range paths {
		a, err := filepath.Abs(p)
		if err != nil {
			s.Fail(Failure{Step: "Upload", Selector: sel, Got: err.Error()})
		}
		abs = append(abs, a)
	}
	s.run("Upload", sel, "the selector has to be an <input type=file>",
		chromedp.ActionFunc(func(ctx context.Context) error {
			obj, exc, err := cdpruntime.Evaluate("document.querySelector(" + quote(sel) + ")").Do(ctx)
			switch {
			case err != nil:
				return err
			case exc != nil:
				return exc
			case obj.ObjectID == "":
				return errors.New("no element matches")
			}
			return dom.SetFileInputFiles(abs).WithObjectID(obj.ObjectID).Do(ctx)
		}))
}

// Text is the visible text of the first element matching sel.
func (s *Session) Text(sel string) string {
	var got string
	s.until("Text", sel, "an element matching the selector", "check the selector against the served HTML",
		func() (string, bool) {
			var err error
			got, err = s.probeJS(sel, `return el ? "\u0000" + el.innerText : "no element";`)
			if err != nil {
				return err.Error(), false
			}
			return got, strings.HasPrefix(got, "\u0000")
		})
	return strings.TrimPrefix(got, "\u0000")
}

// Attr is the attribute name of the first element matching sel, "" when the
// element does not have it.
func (s *Session) Attr(sel, name string) string {
	var got string
	s.until("Attr", sel, "an element matching the selector", "check the selector against the served HTML",
		func() (string, bool) {
			var err error
			got, err = s.probeJS(sel, `return el ? "\u0000" + (el.getAttribute(`+quote(name)+`) ?? "") : "no element";`)
			if err != nil {
				return err.Error(), false
			}
			return got, strings.HasPrefix(got, "\u0000")
		})
	return strings.TrimPrefix(got, "\u0000")
}

// Focused describes the element that has the focus — tag, id, name and the
// start of its text — or "<body>" when nothing has.
func (s *Session) Focused() string {
	var out string
	if err := s.eval(`(() => { `+describeJS+` return d(document.activeElement); })()`, &out); err != nil {
		s.Fail(Failure{Step: "Focused", Got: err.Error()})
	}
	return out
}

// URL is the path and query the tab is on.
func (s *Session) URL() string {
	u := s.url()
	return strings.TrimPrefix(u, s.BaseURL)
}

func (s *Session) url() string {
	var out string
	if err := s.eval(`location.href`, &out); err != nil {
		return ""
	}
	return out
}

// Eval runs a JavaScript expression in the page — a Promise is awaited — and
// decodes its value into out (nil to ignore it).
func (s *Session) Eval(js string, out any) {
	s.step = "Eval"
	var sink any
	if out == nil {
		out = &sink
	}
	if err := s.eval(js, out); err != nil {
		s.Fail(Failure{Step: "Eval", Want: js, Got: err.Error()})
	}
}

// WantText waits until the text of sel contains want.
func (s *Session) WantText(sel, want string) {
	s.until("WantText", sel, "text containing "+quote(want),
		"the page never showed it: check what the handler renders for this state, and the server log",
		func() (string, bool) {
			got, err := s.probeJS(sel, `return el ? el.innerText : "no element";`)
			if err != nil {
				return err.Error(), false
			}
			return clip(got), strings.Contains(got, want)
		})
}

// WantAttr waits until sel has the attribute name equal to want ("" = only
// present).
func (s *Session) WantAttr(sel, name, want string) {
	w := name + "=" + quote(want)
	if want == "" {
		w = name + " present"
	}
	s.until("WantAttr", sel, w, "the attribute never got there: check the component that sets it, or the script that should",
		func() (string, bool) {
			got, err := s.probeJS(sel, `if (!el) return "no element"; const v = el.getAttribute(`+quote(name)+`); `+
				`return v === null ? "absent" : "\u0000" + v;`)
			if err != nil {
				return err.Error(), false
			}
			if !strings.HasPrefix(got, "\u0000") {
				return got, false
			}
			v := strings.TrimPrefix(got, "\u0000")
			return quote(v), want == "" || v == want
		})
}

// WantFocus waits until the focused element matches sel.
func (s *Session) WantFocus(sel string) {
	s.until("WantFocus", sel, "the focus on an element matching the selector",
		"the kit moves the focus to the first [aria-invalid=true] after a 422 swap (ui.InvalidIf), "+
			"and to the new region after a client navigation: check those marks are on the page",
		func() (string, bool) {
			var out string
			err := s.eval(`(() => { `+describeJS+` const a = document.activeElement; `+
				`return (a && a.matches(`+quote(sel)+`) ? "\u0000" : "") + d(a); })()`, &out)
			if err != nil {
				return err.Error(), false
			}
			return strings.TrimPrefix(out, "\u0000"), strings.HasPrefix(out, "\u0000")
		})
}

// WantURL waits until the tab's path (and query, when want has one) is want.
func (s *Session) WantURL(want string) {
	s.until("WantURL", "", want, "the navigation went elsewhere: check the redirect of the handler and the server log",
		func() (string, bool) {
			var out string
			if err := s.eval(`location.pathname + location.search`, &out); err != nil {
				return err.Error(), false
			}
			if !strings.Contains(want, "?") {
				if i := strings.IndexByte(out, '?'); i >= 0 {
					return out, out[:i] == want
				}
			}
			return out, out == want
		})
}

func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// WaitJS waits until the JavaScript expression is true — for a condition the
// other methods do not name: "no reload happened", "the bar reached the end".
// want says it in words, for the report.
func (s *Session) WaitJS(want, expr string) {
	s.until("WaitJS", "", want+" ("+expr+")", "the page never got there: the report's console and server log say what happened instead",
		func() (string, bool) {
			var out any
			if err := s.eval(`(() => { try { const v = (`+expr+`); return v ? true : String(v); } catch (e) { return "throws: " + e.message; } })()`, &out); err != nil {
				return err.Error(), false
			}
			if out == true {
				return "", true
			}
			return fmt.Sprint(out), false
		})
}
