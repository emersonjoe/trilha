package trilha

import (
	"bytes"
	"encoding/json"
	"html"
	"log/slog"
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
