package trilha

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// snapApp serves one page with every volatile value in it: the nonce on an
// inline script, the CSRF token in a form, the request id and a timestamp.
func snapApp(page func(c *Ctx) (h.Node, error)) *App {
	a := New(Config{Env: Prod, Secret: []byte("a-test-secret-with-more-than-32-bytes!!"), Logger: silentLogger()})
	a.Register(Route{Pattern: "/", Page: page})
	return a
}

func volatilePage(c *Ctx) (h.Node, error) {
	return h.Div(
		h.Script(NonceAttr(c), h.Raw("console.log(1)")),
		h.Form(h.Method("post"), CSRFInput(c),
			h.Input(h.Name("email"), h.ID("email"), h.Attr("aria-invalid", "true"), h.Attr("aria-describedby", "email-err")),
			h.Input(h.Name("nome"), h.ID("nome"), h.Attr("aria-invalid", "true")),
		),
		h.P(h.Attr("data-request-id", c.RequestID()), h.Text(time.Now().UTC().Format(time.RFC3339Nano))),
		h.P(h.Text("2026-01-02")),
	), nil
}

func TestPageSnapshotIsStable(t *testing.T) {
	a := snapApp(volatilePage)
	one := CapturePage(t, a, "GET", "/")
	two := CapturePage(t, a, "GET", "/")
	if one.String() != two.String() {
		t.Fatalf("two snapshots of the same page differ:\n%s\n---\n%s", one, two)
	}
	for _, want := range []string{`nonce="{{NONCE}}"`, `value="{{CSRF}}"`, `data-request-id="{{RID}}"`,
		"{{DATE}}", "2026-01-02", "'nonce-{{NONCE}}'", "X-Request-Id: {{RID}}", CSRFCookie + "={{CSRF}}"} {
		if !strings.Contains(one.String(), want) {
			t.Errorf("snapshot lacks %q:\n%s", want, one)
		}
	}
	if !strings.HasPrefix(one.String(), "200 OK\n") {
		t.Errorf("snapshot does not start with the status: %.40q", one.String())
	}
}

// Behind a login: the snapshot of a TestClient response takes the cookies the
// client sent too, not only the ones the response set.
func TestResponseSnapshotUsesSentCookies(t *testing.T) {
	a := snapApp(volatilePage)
	c := NewTestClient(t, a)
	c.Get("/")
	snap := c.Get("/").Snapshot()
	if strings.Contains(snap.Body, c.jar[CSRFCookie]) {
		t.Fatalf("the CSRF token from the jar is still in the body")
	}
	if !strings.Contains(snap.Body, `value="{{CSRF}}"`) {
		t.Fatalf("body:\n%s", snap.Body)
	}
}

func TestMatchGolden(t *testing.T) {
	snap := CapturePage(t, snapApp(volatilePage), "GET", "/")
	path := filepath.Join(t.TempDir(), "sub", "page.golden")
	if err := snap.MatchGolden(path, false); err == nil || !strings.Contains(err.Error(), "-update") {
		t.Fatalf("missing golden: %v", err)
	}
	if err := snap.MatchGolden(path, true); err != nil {
		t.Fatal(err)
	}
	if err := snap.MatchGolden(path, false); err != nil {
		t.Fatal(err)
	}
	snap.Body = strings.Replace(snap.Body, "2026-01-02", "2026-01-03", 1)
	err := snap.MatchGolden(path, false)
	if err == nil || !strings.Contains(err.Error(), "2026-01-03") || !strings.Contains(err.Error(), "line") {
		t.Fatalf("changed body: %v", err)
	}
	if b, _ := os.ReadFile(path); strings.Contains(string(b), "2026-01-03") {
		t.Fatal("a failed match rewrote the golden")
	}
}

func TestHasCSRFToken(t *testing.T) {
	good := CapturePage(t, snapApp(volatilePage), "GET", "/")
	if err := good.HasCSRFToken(); err != nil {
		t.Fatal(err)
	}
	bad := CapturePage(t, snapApp(func(c *Ctx) (h.Node, error) {
		return h.Div(
			h.Form(h.Method("get"), h.Input(h.Name("q"))),
			h.Form(h.Method("POST"), h.Action("/sair"), h.Button(h.Text("Sair"))),
		), nil
	}), "GET", "/")
	err := bad.HasCSRFToken()
	if err == nil || !strings.Contains(err.Error(), `action="/sair"`) || !strings.Contains(err.Error(), "CSRFInput") {
		t.Fatalf("form without token: %v", err)
	}
}

func TestHasAria(t *testing.T) {
	s := CapturePage(t, snapApp(volatilePage), "GET", "/")
	if err := s.HasAria("input#email", "aria-describedby"); err != nil {
		t.Fatal(err)
	}
	err := s.HasAria("input[aria-invalid=true]", "aria-describedby")
	if err == nil || !strings.Contains(err.Error(), "1 of 2") || !strings.Contains(err.Error(), `name="nome"`) {
		t.Fatalf("one field without it: %v", err)
	}
	if err := s.HasAria("button.nada", "aria-label"); err == nil || !strings.Contains(err.Error(), "no element") {
		t.Fatalf("nothing matches: %v", err)
	}
	if err := s.HasAria("form input", "aria-label"); err == nil || !strings.Contains(err.Error(), "understood") {
		t.Fatalf("descendant selector: %v", err)
	}
}

func TestFocusedOnError(t *testing.T) {
	s := CapturePage(t, snapApp(volatilePage), "GET", "/")
	if err := s.FocusedOnError("#email"); err != nil {
		t.Fatal(err)
	}
	err := s.FocusedOnError("input[name=nome]")
	if err == nil || !strings.Contains(err.Error(), `id="email"`) {
		t.Fatalf("focus elsewhere: %v", err)
	}
	none := CapturePage(t, snapApp(func(c *Ctx) (h.Node, error) { return h.Input(h.ID("x")), nil }), "GET", "/")
	if err := none.FocusedOnError("#x"); err == nil || !strings.Contains(err.Error(), "InvalidIf") {
		t.Fatalf("nothing invalid: %v", err)
	}
}

func TestHasCSPNonce(t *testing.T) {
	s := CapturePage(t, snapApp(volatilePage), "GET", "/")
	if err := s.HasCSPNonce(); err != nil {
		t.Fatal(err)
	}
	bad := CapturePage(t, snapApp(func(c *Ctx) (h.Node, error) {
		return h.Div(
			h.Script(h.Src("/app.js")),
			h.Script(h.Type("application/json"), h.Raw(`{}`)),
			h.Script(h.Raw("alert(1)")),
		), nil
	}), "GET", "/")
	err := bad.HasCSPNonce()
	if err == nil || !strings.Contains(err.Error(), "E_CSP_NONCE") || !strings.Contains(err.Error(), "1 inline") {
		t.Fatalf("script without nonce: %v", err)
	}
}

func TestHasSafeCookies(t *testing.T) {
	s := CapturePage(t, snapApp(volatilePage), "GET", "/")
	if err := s.HasSafeCookies(); err != nil {
		t.Fatal(err)
	}
	a := New(Config{Env: Prod, Secret: []byte("a-test-secret-with-more-than-32-bytes!!"), Logger: silentLogger()})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		http.SetCookie(c.Writer(), &http.Cookie{Name: "pref", Value: "dark-mode-on", Path: "/"})
		http.SetCookie(c.Writer(), &http.Cookie{Name: "old", Value: "", MaxAge: -1})
		return h.Text("ok"), nil
	}})
	err := CapturePage(t, a, "GET", "/").HasSafeCookies()
	if err == nil || !strings.Contains(err.Error(), "pref without HttpOnly and SameSite") || strings.Contains(err.Error(), "old") {
		t.Fatalf("unsafe cookie: %v", err)
	}
}

func TestHasNoSecret(t *testing.T) {
	s := CapturePage(t, snapApp(func(c *Ctx) (h.Node, error) {
		return h.P(h.Text("key: sk_live_abcdef123456")), nil
	}), "GET", "/")
	if err := s.HasNoSecret("", "whsec_other"); err != nil {
		t.Fatal(err)
	}
	if err := s.HasNoSecret("sk_live_abcdef123456"); err == nil || !strings.Contains(err.Error(), "never the value") {
		t.Fatalf("secret on the page: %v", err)
	}
}

func TestParseElementsSkipsScriptBodies(t *testing.T) {
	els := parseElements(`<!-- <form method=post> --><script>if (a<b) "<form method=post>"</script>` +
		`<input name='x' value="a &amp; b" disabled data-y=z>`)
	var tags []string
	for _, e := range els {
		tags = append(tags, e.tag)
	}
	if strings.Join(tags, ",") != "script,/script,input" {
		t.Fatalf("tags = %v", tags)
	}
	in := els[2]
	if in.attr("value") != "a & b" || in.attr("data-y") != "z" || in.attr("name") != "x" {
		t.Fatalf("attrs = %v", in.attrs)
	}
	if _, ok := in.attrs["disabled"]; !ok {
		t.Fatalf("boolean attr lost: %v", in.attrs)
	}
}
