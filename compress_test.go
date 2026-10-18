package trilha

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/emersonjoe/trilha/h"
)

func gzipApp(env Env) (*App, []byte) {
	css := []byte(strings.Repeat(".ui-button{color:var(--ui-primary)}\n", 200))
	return New(Config{Env: env, Logger: quiet(), Public: fstest.MapFS{
		"ui.css":   &fstest.MapFile{Data: css},
		"tiny.js":  &fstest.MapFile{Data: []byte("ok()")},
		"foto.png": &fstest.MapFile{Data: bytes.Repeat([]byte{0x89}, 4096)},
	}}), css
}

func getWith(a *App, target string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

// Spec 177: a text asset goes compressed to whoever accepts gzip, with the
// same bytes once unpacked, the content type of the file and an ETag of its
// own — one strong tag for two different bodies breaks RFC 9110 §8.8.3.
func TestStaticGzip(t *testing.T) {
	for _, env := range []Env{Prod, Dev} {
		a, css := gzipApp(env)
		plain := getWith(a, "/ui.css", nil)
		zipped := getWith(a, "/ui.css", map[string]string{"Accept-Encoding": "br, gzip;q=0.8"})

		if plain.Header().Get("Content-Encoding") != "" || !bytes.Equal(plain.Body.Bytes(), css) {
			t.Fatalf("%v: without Accept-Encoding the file goes as it is", env)
		}
		if zipped.Code != 200 || zipped.Header().Get("Content-Encoding") != "gzip" {
			t.Fatalf("%v: %d, Content-Encoding %q", env, zipped.Code, zipped.Header().Get("Content-Encoding"))
		}
		size := zipped.Body.Len()
		zr, err := gzip.NewReader(bytes.NewReader(zipped.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(zr)
		if !bytes.Equal(got, css) {
			t.Fatalf("%v: unpacked body differs from the file", env)
		}
		if size == 0 || size > len(css)/4 {
			t.Fatalf("%v: gzip body is %d bytes for %d of CSS", env, size, len(css))
		}
		if ct := zipped.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
			t.Fatalf("%v: Content-Type %q", env, ct)
		}
		for _, rec := range []*httptest.ResponseRecorder{plain, zipped} {
			if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Accept-Encoding") {
				t.Fatalf("%v: Vary %q: a shared cache would hand gzip to whoever cannot read it", env, rec.Header().Values("Vary"))
			}
		}
		pt, zt := plain.Header().Get("ETag"), zipped.Header().Get("ETag")
		if pt == "" || zt == "" || pt == zt {
			t.Fatalf("%v: ETag identity %q, gzip %q", env, pt, zt)
		}

		// Revalidating the compressed copy answers 304 with no body.
		again := getWith(a, "/ui.css", map[string]string{"Accept-Encoding": "gzip", "If-None-Match": zt})
		if again.Code != 304 || again.Body.Len() != 0 {
			t.Fatalf("%v: revalidation %d, %d bytes", env, again.Code, again.Body.Len())
		}
	}
}

// What gzip does not help or cannot do safely goes as it is: a file too small
// to gain, an image already compressed, a Range request (the offsets are of
// the identity body), and a client that refuses gzip with q=0.
func TestStaticGzipLeavesAlone(t *testing.T) {
	a, _ := gzipApp(Prod)
	cases := map[string]*httptest.ResponseRecorder{
		"tiny":   getWith(a, "/tiny.js", map[string]string{"Accept-Encoding": "gzip"}),
		"png":    getWith(a, "/foto.png", map[string]string{"Accept-Encoding": "gzip"}),
		"range":  getWith(a, "/ui.css", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9"}),
		"q=0":    getWith(a, "/ui.css", map[string]string{"Accept-Encoding": "gzip;q=0, identity"}),
		"head-q": getWith(a, "/ui.css", map[string]string{"Accept-Encoding": "*;q=0, identity"}),
	}
	for name, rec := range cases {
		if ce := rec.Header().Get("Content-Encoding"); ce != "" {
			t.Errorf("%s: Content-Encoding %q", name, ce)
		}
	}
	if cases["range"].Code != 206 || cases["range"].Body.Len() != 10 {
		t.Errorf("range: %d, %d bytes", cases["range"].Code, cases["range"].Body.Len())
	}
}

// Pages are not compressed: they carry the CSRF token next to text the
// visitor controls, the shape BREACH needs.
func TestPageIsNotCompressed(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet()})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		return h.P(h.Text(strings.Repeat("texto ", 500))), nil
	}})
	rec := getWith(a, "/", map[string]string{"Accept-Encoding": "gzip"})
	if rec.Code != 200 || rec.Header().Get("Content-Encoding") != "" {
		t.Fatalf("%d, Content-Encoding %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
}

// Spec 177: Vary accumulates. Setting it wiped the "Origin" CORS added, and a
// shared cache could then hand one origin's Access-Control-Allow-Origin to
// another.
func TestVaryKeepsWhatWasThere(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), CORS: CORS{Origins: []string{"https://app.exemplo.com"}}})
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		c.ETag("v1")
		return h.P(h.Text("oi")), nil
	}})
	for _, inm := range []string{"", `"v1"`} {
		hdr := map[string]string{"Origin": "https://app.exemplo.com"}
		if inm != "" {
			hdr["If-None-Match"] = inm
		}
		rec := getWith(a, "/", hdr)
		vary := strings.Join(rec.Header().Values("Vary"), ",")
		if !strings.Contains(vary, "Origin") || !strings.Contains(vary, fragmentHeader) {
			t.Fatalf("status %d: Vary %q, want Origin and %s", rec.Code, vary, fragmentHeader)
		}
		if strings.Count(vary, fragmentHeader) != 1 {
			t.Fatalf("Vary repeats: %q", vary)
		}
	}
}
