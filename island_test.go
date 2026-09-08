package trilha

import (
	"bytes"
	"encoding/json"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// islandApp serves one page whose body is what the test asks for.
func islandApp(t *testing.T, logger *slog.Logger, page func(c *Ctx) (h.Node, error)) *App {
	t.Helper()
	if logger == nil {
		logger = quiet()
	}
	a := New(Config{Env: Prod, Logger: logger, Secret: []byte("0123456789abcdef0123456789abcdef")})
	a.Register(Route{Pattern: "/", Kind: KindPage, Page: page})
	return a
}

func islandGet(t *testing.T, a *App) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	return rec
}

// Issue #22: the island carries its own module and its own data, the fallback
// is server-rendered, and the loader is an inline script the default CSP
// accepts because it has the request nonce.
func TestIslandRendersFallbackAndLoader(t *testing.T) {
	a := islandApp(t, nil, func(c *Ctx) (h.Node, error) {
		return h.Div(c.Island("/editor.js", map[string]any{"wpm": 200},
			h.Class("editor"), h.P(h.Text("sem script, o formulário ainda envia")))), nil
	})
	rec := islandGet(t, a)
	body := rec.Body.String()

	for _, want := range []string{
		`data-trilha-island="/editor.js"`,
		`data-trilha-props="{&#34;wpm&#34;:200}"`,
		`class="editor"`,
		"sem script, o formulário ainda envia",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("island markup missing %s in:\n%s", want, body)
		}
	}
	if n := strings.Count(body, `data-trilha-island="`); n != 1 {
		t.Fatalf("data-trilha-island appears %d times, want 1", n)
	}
	nonce := regexp.MustCompile(`nonce-([A-Za-z0-9+/]+)`).FindStringSubmatch(rec.Header().Get("Content-Security-Policy"))
	if nonce == nil {
		t.Fatal("no nonce in the CSP")
	}
	if !strings.Contains(body, `<script nonce="`+nonce[1]+`">`) {
		t.Fatalf("loader script is not carrying the CSP nonce:\n%s", body)
	}
}

// One loader per response, however many islands the page has: the loader
// mounts every one of them, and a second copy would mount them twice.
func TestIslandLoaderOncePerResponse(t *testing.T) {
	a := islandApp(t, nil, func(c *Ctx) (h.Node, error) {
		return h.Div(
			c.Island("/a.js", nil, h.Text("a")),
			c.Island("/b.js", nil, h.Text("b")),
		), nil
	})
	body := islandGet(t, a).Body.String()
	if n := strings.Count(body, islandLoaderMark); n != 1 {
		t.Fatalf("loader appears %d times, want 1:\n%s", n, body)
	}
	if n := strings.Count(body, `data-trilha-island="`); n != 2 {
		t.Fatalf("islands mounted: %d, want 2", n)
	}
	if strings.Contains(body, `data-trilha-props="`) {
		t.Fatal("nil props should not become an attribute")
	}
}

// The props are data the page author did not write: a string that closes the
// script or the attribute must come back as a string, not as markup.
func TestIslandPropsCannotEscape(t *testing.T) {
	hostile := `</script><img src=x onerror=alert(1)>"'`
	a := islandApp(t, nil, func(c *Ctx) (h.Node, error) {
		return h.Div(c.Island("/x.js", map[string]any{"nome": hostile})), nil
	})
	body := islandGet(t, a).Body.String()
	if strings.Contains(body, "<img") || strings.Count(body, "</script>") != 1 {
		t.Fatalf("props escaped into markup:\n%s", body)
	}
	m := regexp.MustCompile(`data-trilha-props="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no props attribute in:\n%s", body)
	}
	var got struct{ Nome string }
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &got); err != nil {
		t.Fatalf("props are not JSON after unescaping: %v", err)
	}
	if got.Nome != hostile {
		t.Fatalf("props round trip = %q, want %q", got.Nome, hostile)
	}
}

// Props that cannot be serialized are a programming mistake, and the page is
// not the place to die for it: the fallback stays, and the log says so once.
func TestIslandBadPropsKeepsFallback(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	a := islandApp(t, logger, func(c *Ctx) (h.Node, error) {
		return h.Div(
			c.Island("/x.js", func() {}, h.Text("conteúdo do servidor")),
			c.Island("/x.js", func() {}, h.Text("de novo")),
		), nil
	})
	body := islandGet(t, a).Body.String()
	if strings.Contains(body, `data-trilha-island="`) {
		t.Fatalf("island was mounted with props that do not serialize:\n%s", body)
	}
	if !strings.Contains(body, "conteúdo do servidor") {
		t.Fatal("fallback content was dropped")
	}
	if n := strings.Count(buf.String(), "/x.js"); n != 1 {
		t.Fatalf("warned %d times, want 1: %s", n, buf.String())
	}
}

// Issue #70: the island reaches the server. The token of the double-submit
// cookie is HttpOnly, so it travels in the element — the same token every form
// of the response already carries.
func TestIslandCarriesTheCSRFToken(t *testing.T) {
	a := islandApp(t, nil, func(c *Ctx) (h.Node, error) {
		return h.Div(c.Island("/editor.js", nil), CSRFInput(c)), nil
	})
	rec := islandGet(t, a)
	body := rec.Body.String()
	m := regexp.MustCompile(`data-trilha-csrf="([^"]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no token on the island:\n%s", body)
	}
	form := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(body)
	if form == nil || form[1] != m[1] {
		t.Fatalf("island token %q, form token %v", m[1], form)
	}
	if ck := rec.Result().Cookies(); len(ck) == 0 || ck[0].Value != html.UnescapeString(m[1]) {
		t.Fatalf("the token in the page is not the one in the cookie: %v", ck)
	}
}

// The mount function receives a third argument, and what it can do with it is
// the contract this test pins: the code lives in the loader, so nobody has to
// download a second module before the island can save.
func TestIslandLoaderCarriesTheChannel(t *testing.T) {
	for _, want := range []string{
		"f(el,p,api(el,ac))",                 // the third argument
		`h["X-CSRF-Token"]=t`,                // the token goes on every write
		`credentials:"same-origin"`,          // and so does the session
		"IslandInvalid",                      // 422 is not an error like the others
		"body.fields",                        // the same fields a form would get
		`res.headers.get("Trilha-Location")`, // a redirect is a redirect
		`headers:{"Trilha-Fragment":target}`, // swap speaks the fragment protocol
		`new CustomEvent("trilha:swap"`,      // and tells the page, like ui.js does
		"new AbortController()",              // signal
		"if(!el.isConnected){ac.abort()",     // aborted when the element is gone
	} {
		if !strings.Contains(islandLoader, want) {
			t.Fatalf("the island loader does not carry %q", want)
		}
	}
	if strings.Contains(islandLoader, "import(") && !strings.Contains(islandLoader, "/*trilha-islands*/") {
		t.Fatal("the loader lost its mark")
	}
}

// The server side of island.post is a route.go like any other: BindJSON reads
// the body, FieldErrors answers 422, and what comes back is the object the
// island turns into IslandInvalid.fields.
func TestIslandPostGetsTheSameFieldsAsAForm(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), Secret: []byte("0123456789abcdef0123456789abcdef")})
	a.Register(Route{Pattern: "/api/save", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			var in struct {
				Title string `json:"title" validate:"required"`
			}
			if err := c.BindJSON(&in); err != nil {
				return err
			}
			return c.JSON(200, map[string]string{"title": in.Title})
		},
	}})

	body := strings.NewReader(`{"title":""}`)
	req := httptest.NewRequest("POST", "/api/save", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(CSRFHeader, "0123456789abcdef0123456789abcdef")
	req.AddCookie(&http.Cookie{Name: CSRFCookie, Value: "0123456789abcdef0123456789abcdef"})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != 422 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var problem struct {
		Detail string            `json:"detail"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Fields["title"] == "" {
		t.Fatalf("no field error: %s", rec.Body)
	}
}
