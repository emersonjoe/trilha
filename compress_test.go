package trilha

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/emersonjoe/trilha/h"
)

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("not gzip: %v (%q)", err, b[:min(len(b), 40)])
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

var compressCSS = strings.Repeat(".ui-card{display:grid;gap:.5rem;padding:1rem}\n", 200)

func compressApp(cfg Config) *App {
	cfg.Logger = quiet()
	if cfg.Env == "" {
		cfg.Env = Prod
	}
	cfg.Public = fstest.MapFS{
		"ui.css":   {Data: []byte(compressCSS)},
		"logo.png": {Data: bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 1000)},
		"tiny.js":  {Data: []byte("x=1")},
	}
	a := New(cfg)
	big := strings.Repeat("linha da tabela ", 400)
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		return h.Div(h.Text(big)), nil
	}})
	a.Register(Route{Pattern: "/curta", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		return c.Text(200, "ok")
	}}})
	a.Register(Route{Pattern: "/json", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		return c.JSON(200, map[string]string{"x": big})
	}}})
	a.Register(Route{Pattern: "/ja", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		c.Header("Content-Encoding", "br")
		c.Header("Content-Type", "text/plain")
		_, err := c.Writer().Write([]byte(big))
		return err
	}}})
	a.Register(Route{Pattern: "/sse", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		s := c.Stream()
		if err := s.Send("um", big); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond)
		return s.Send("dois", "fim")
	}}})
	return a
}

var gz = map[string]string{"Accept-Encoding": "gzip, deflate, br"}

// SC-001 — the SSR page, JSON and the kit's CSS go out compressed, and the
// body that comes out of gunzip is the one that went in.
func TestCompressionTextResponses(t *testing.T) {
	a := compressApp(Config{})
	plain := get(t, a, "GET", "/", "", nil)
	if plain.Header().Get("Content-Encoding") != "" {
		t.Fatal("a client that did not ask must get plain bytes")
	}
	for _, p := range []string{"/", "/json", "/ui.css"} {
		rec := get(t, a, "GET", p, "", gz)
		if rec.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%s: %v", p, rec.Header())
		}
		if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Accept-Encoding") {
			t.Fatalf("%s: Vary %v", p, rec.Header().Values("Vary"))
		}
		if rec.Header().Get("Content-Length") != "" && p != "/ui.css" {
			t.Fatalf("%s: stale Content-Length %s", p, rec.Header().Get("Content-Length"))
		}
		body := gunzip(t, rec.Body.Bytes())
		if p == "/" && body != plain.Body.String() {
			t.Fatalf("decompressed page differs:\n%s\n---\n%s", body[:80], plain.Body.String()[:80])
		}
		if p == "/ui.css" && body != compressCSS {
			t.Fatal("css differs")
		}
		if rec.Body.Len()*3 > len(body) {
			t.Fatalf("%s: %d → %d is not compression", p, len(body), rec.Body.Len())
		}
	}
	// The page keeps the fragment's Vary next to the encoding's.
	vary := strings.Join(get(t, a, "GET", "/", "", gz).Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Trilha-Fragment") {
		t.Fatal(vary)
	}
}

// SC-002 — what is never compressed.
func TestCompressionSkips(t *testing.T) {
	a := compressApp(Config{})
	cases := map[string]struct {
		method, path string
		hdr          map[string]string
	}{
		"short body":           {"GET", "/curta", gz},
		"png":                  {"GET", "/logo.png", gz},
		"tiny static":          {"GET", "/tiny.js", gz},
		"already encoded":      {"GET", "/ja", gz},
		"head":                 {"HEAD", "/", gz},
		"range":                {"GET", "/ui.css", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9"}},
		"gzip refused":         {"GET", "/", map[string]string{"Accept-Encoding": "gzip;q=0, *"}},
		"identity only":        {"GET", "/", map[string]string{"Accept-Encoding": "identity"}},
		"no header":            {"GET", "/", nil},
		"star with q=0 refuse": {"GET", "/", map[string]string{"Accept-Encoding": "*;q=0"}},
	}
	for name, tc := range cases {
		rec := get(t, a, tc.method, tc.path, "", tc.hdr)
		ce := rec.Header().Get("Content-Encoding")
		if name == "already encoded" {
			if ce != "br" {
				t.Fatalf("%s: the handler's encoding was replaced: %q", name, ce)
			}
			continue
		}
		if ce != "" {
			t.Fatalf("%s: compressed (%s)", name, ce)
		}
	}
	if get(t, a, "GET", "/", "", map[string]string{"Accept-Encoding": "*"}).Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("* accepts gzip")
	}
}

// SC-003 — the event stream is never compressed and each event reaches the
// client when it is sent, not when the handler returns.
func TestCompressionLeavesEventStreamLive(t *testing.T) {
	srv := httptest.NewServer(compressApp(Config{}).Handler())
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/sse", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("Content-Encoding") != "" {
		t.Fatal("SSE compressed")
	}
	start := time.Now()
	line, err := bufio.NewReader(res.Body).ReadString('\n')
	if err != nil || line != "event: um\n" {
		t.Fatalf("%q %v", line, err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("the first event waited for the handler to end")
	}
}

// SC-004 — a versioned static file is compressed once, answered with a weak
// validator, and the 304 still works on it.
func TestCompressionStaticCacheAndETag(t *testing.T) {
	a := compressApp(Config{})
	first := get(t, a, "GET", "/ui.css", "", gz)
	etag := first.Header().Get("ETag")
	if !strings.HasPrefix(etag, `W/"`) {
		t.Fatalf("ETag %q", etag)
	}
	if plainTag := get(t, a, "GET", "/ui.css", "", nil).Header().Get("ETag"); plainTag != strings.TrimPrefix(etag, "W/") {
		t.Fatalf("plain %q gzip %q", plainTag, etag)
	}
	second := get(t, a, "GET", "/ui.css", "", gz)
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("the cached bytes changed")
	}
	if a.staticGzip.total == 0 {
		t.Fatal("not cached")
	}
	nm := get(t, a, "GET", "/ui.css", "", map[string]string{"Accept-Encoding": "gzip", "If-None-Match": etag})
	if nm.Code != http.StatusNotModified || nm.Body.Len() != 0 {
		t.Fatalf("%d %d", nm.Code, nm.Body.Len())
	}
}

// SC-005 — Disabled is for the app behind a proxy that already compresses.
func TestCompressionDisabled(t *testing.T) {
	a := compressApp(Config{Compression: Compression{Disabled: true}})
	for _, p := range []string{"/", "/ui.css"} {
		if ce := get(t, a, "GET", p, "", gz).Header().Get("Content-Encoding"); ce != "" {
			t.Fatalf("%s: %s", p, ce)
		}
	}
}

// SC-006 — BREACH: with the page compressed, the token in it changes every
// response, and every one of them still passes the check.
func TestCSRFTokenMaskedPerResponse(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet()})
	a.Register(Route{Pattern: "/form", Page: func(c *Ctx) (h.Node, error) {
		return h.Form(h.Method("post"), CSRFInput(c)), nil
	}, Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return c.Text(200, "ok") }}})
	cookie := "segredo-do-cookie-com-mais-de-32-bytes-ok"
	tokens := map[string]bool{}
	for range 3 {
		body := get(t, a, "GET", "/form", "", map[string]string{"Cookie": CSRFCookie + "=" + cookie}).Body.String()
		i := strings.Index(body, `value="`)
		tok := body[i+7 : i+7+strings.Index(body[i+7:], `"`)]
		if strings.Contains(tok, cookie) || unmaskCSRF(tok) != cookie {
			t.Fatalf("token %q does not mask the cookie", tok)
		}
		tokens[tok] = true
		rec := get(t, a, "POST", "/form", "", map[string]string{
			"Cookie": CSRFCookie + "=" + cookie, CSRFHeader: tok})
		if rec.Code != 200 {
			t.Fatalf("masked token refused: %d", rec.Code)
		}
	}
	if len(tokens) != 3 {
		t.Fatal("the same token was written twice")
	}
	// The bare value still passes (test clients, apps that read the cookie).
	if rec := get(t, a, "POST", "/form", "", map[string]string{
		"Cookie": CSRFCookie + "=" + cookie, CSRFHeader: cookie}); rec.Code != 200 {
		t.Fatalf("bare token refused: %d", rec.Code)
	}
	for _, bad := range []string{"m.", "m.!!!", "m.YWJj", maskCSRF("outro-segredo-qualquer-de-32-bytes!!")} {
		if rec := get(t, a, "POST", "/form", "", map[string]string{
			"Cookie": CSRFCookie + "=" + cookie, CSRFHeader: bad}); rec.Code != http.StatusForbidden {
			t.Fatalf("%q accepted: %d", bad, rec.Code)
		}
	}
}
