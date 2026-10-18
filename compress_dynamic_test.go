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
	"time"

	"github.com/emersonjoe/trilha/h"
)

func dynamicApp(cfg Config) *App {
	cfg.Logger = quiet()
	a := New(cfg)
	big := strings.Repeat("palavra ", 400)
	a.Register(Route{Pattern: "/pagina", Page: func(c *Ctx) (h.Node, error) {
		c.ETag("v7")
		return h.Form(h.Method("post"), CSRFInput(c), h.P(h.Text(big))), nil
	}})
	a.Register(Route{Pattern: "/curta", Page: func(c *Ctx) (h.Node, error) { return h.P(h.Text("oi")), nil }})
	a.Register(Route{Pattern: "/api/lista", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		return c.JSON(200, map[string]string{"texto": big})
	}}})
	a.Register(Route{Pattern: "/foto", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		c.Writer().Header().Set("Content-Type", "image/png")
		_, err := c.Writer().Write(bytes.Repeat([]byte{0x89}, 4096))
		return err
	}}})
	a.Register(Route{Pattern: "/eventos", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		s := c.Stream()
		for i := 0; i < 50; i++ {
			if err := s.Send("tick", big[:40]); err != nil {
				return err
			}
		}
		return nil
	}}})
	return a
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// #305: pages and API answers go gzip-compressed to the client that accepts
// it, now that the CSRF token in them is masked per response (#308).
func TestDynamicResponsesCompressed(t *testing.T) {
	a := dynamicApp(Config{Env: Prod})
	for _, path := range []string{"/pagina", "/api/lista"} {
		plain := getWith(a, path, nil)
		zipped := getWith(a, path, map[string]string{"Accept-Encoding": "gzip"})
		if ce := zipped.Header().Get("Content-Encoding"); ce != "gzip" {
			t.Fatalf("%s: Content-Encoding %q", path, ce)
		}
		if plain.Header().Get("Content-Encoding") != "" {
			t.Fatalf("%s: compressed for a client that did not ask", path)
		}
		if zipped.Header().Get("Content-Length") != "" {
			t.Fatalf("%s: Content-Length of the identity body on a gzip one", path)
		}
		if !strings.Contains(strings.Join(zipped.Header().Values("Vary"), ","), "Accept-Encoding") {
			t.Fatalf("%s: Vary %q", path, zipped.Header().Values("Vary"))
		}
		got := gunzip(t, zipped.Body.Bytes())
		want := plain.Body.String()
		if path == "/pagina" { // the masked token differs per response; the rest is the page
			got, want = csrfValue.ReplaceAllString(got, "TOKEN"), csrfValue.ReplaceAllString(want, "TOKEN")
		}
		if got != want {
			t.Fatalf("%s: unpacked body differs", path)
		}
		if zipped.Body.Len()*3 > plain.Body.Len() {
			t.Fatalf("%s: %d bytes compressed for %d", path, zipped.Body.Len(), plain.Body.Len())
		}
	}
}

// A compressed body is another body: its ETag turns weak, and the browser
// that revalidates with it still gets a 304 (RFC 9110 §8.8.3).
func TestDynamicCompressedETagIsWeak(t *testing.T) {
	a := dynamicApp(Config{Env: Prod})
	zipped := getWith(a, "/pagina", map[string]string{"Accept-Encoding": "gzip"})
	tag := zipped.Header().Get("ETag")
	if tag != `W/"v7"` {
		t.Fatalf("ETag %q, want W/\"v7\"", tag)
	}
	again := getWith(a, "/pagina", map[string]string{"Accept-Encoding": "gzip", "If-None-Match": tag})
	if again.Code != 304 || again.Body.Len() != 0 || again.Header().Get("Content-Encoding") != "" {
		t.Fatalf("revalidation: %d, %d bytes, %q", again.Code, again.Body.Len(), again.Header().Get("Content-Encoding"))
	}
}

// What gzip cannot help or must not touch goes as it is: a short page, an
// image, a stream of events, a HEAD, and an app that turned it off.
func TestDynamicCompressionLeavesAlone(t *testing.T) {
	a := dynamicApp(Config{Env: Prod})
	gz := map[string]string{"Accept-Encoding": "gzip"}
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"short page": getWith(a, "/curta", gz),
		"image":      getWith(a, "/foto", gz),
		"events":     getWith(a, "/eventos", gz),
		"off":        getWith(dynamicApp(Config{Env: Prod, Compress: Off}), "/pagina", gz),
	} {
		if ce := rec.Header().Get("Content-Encoding"); ce != "" {
			t.Errorf("%s: Content-Encoding %q", name, ce)
		}
		if rec.Code != 200 || rec.Body.Len() == 0 {
			t.Errorf("%s: %d, %d bytes", name, rec.Code, rec.Body.Len())
		}
	}
	req := httptest.NewRequest("HEAD", "/pagina", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("HEAD: Content-Encoding %q", rec.Header().Get("Content-Encoding"))
	}
}

// A flush reaches the client compressed: a page that streams its slow part
// (ui.Defer, a progressive render) is not held until the end.
func TestDynamicCompressionFlushes(t *testing.T) {
	release := make(chan struct{})
	a := New(Config{Env: Prod, Logger: quiet()})
	a.Register(Route{Pattern: "/lento", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		c.Writer().Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(c.Writer(), "<p>first</p>")
		http.NewResponseController(c.Writer()).Flush()
		<-release
		_, err := io.WriteString(c.Writer(), "<p>second</p>")
		return err
	}}})
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	defer close(release)
	req, _ := http.NewRequest("GET", srv.URL+"/lento", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding %q", res.Header.Get("Content-Encoding"))
	}
	done := make(chan string, 1)
	go func() {
		zr, err := gzip.NewReader(res.Body)
		if err != nil {
			done <- err.Error()
			return
		}
		line, _ := bufio.NewReader(zr).ReadString('>')
		done <- line
	}()
	select {
	case got := <-done:
		if got != "<p>" {
			t.Fatalf("first bytes %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the flushed part did not reach the client before the handler finished")
	}
}
