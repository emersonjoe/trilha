package trilha

import (
	"net/http"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

var modified = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// cacheApp answers with the etag and date the test asks for through headers, so
// one app serves every case.
func cacheApp(t *testing.T) *App {
	t.Helper()
	a := New(Config{Env: Prod, Logger: quiet()})
	a.Register(Route{Pattern: "/post", Page: func(c *Ctx) (h.Node, error) {
		if c.ETag("v1") {
			return nil, nil
		}
		return h.Div(h.Text("corpo")), nil
	}})
	a.Register(Route{Pattern: "/data", Page: func(c *Ctx) (h.Node, error) {
		if c.LastModified(modified) {
			return nil, nil
		}
		return h.Div(h.Text("corpo")), nil
	}})
	a.Register(Route{Pattern: "/ambos", Page: func(c *Ctx) (h.Node, error) {
		if c.ETag("v1") {
			return nil, nil
		}
		if c.LastModified(modified) {
			return nil, nil
		}
		return h.Div(h.Text("corpo")), nil
	}})
	a.Register(Route{Pattern: "/vazio", Page: func(c *Ctx) (h.Node, error) {
		if c.ETag("") || c.LastModified(time.Time{}) {
			t.Error("nothing declared, nothing to match")
		}
		c.CacheControl("private, no-cache")
		return h.Div(h.Text("corpo")), nil
	}})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{
		"POST": func(c *Ctx) error {
			if c.ETag("v1") {
				t.Error("a POST is not a revalidation")
			}
			return c.JSON(201, map[string]string{"ok": "1"})
		},
	}})
	return a
}

func TestETagAnswers304(t *testing.T) {
	a := cacheApp(t)

	rec := get(t, a, "GET", "/post", "", nil)
	if rec.Code != 200 || rec.Header().Get("ETag") != `"v1"` {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("ETag"))
	}

	for _, inm := range []string{`"v1"`, `"v0", "v1"`, `W/"v1"`, "*"} {
		rec := get(t, a, "GET", "/post", "", map[string]string{"If-None-Match": inm})
		if rec.Code != http.StatusNotModified {
			t.Fatalf("If-None-Match %s: %d", inm, rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("304 with a body: %q", rec.Body.String())
		}
		// A 304 has to repeat the validator and the Vary, or the next
		// request is answered from the wrong entry.
		if rec.Header().Get("ETag") != `"v1"` {
			t.Fatalf("304 without ETag: %q", rec.Header().Get("ETag"))
		}
		if rec.Header().Get("Vary") != fragmentHeader {
			t.Fatalf("304 without Vary: %q", rec.Header().Get("Vary"))
		}
	}

	if rec := get(t, a, "GET", "/post", "", map[string]string{"If-None-Match": `"v0"`}); rec.Code != 200 || rec.Body.Len() == 0 {
		t.Fatalf("another version: %d %d bytes", rec.Code, rec.Body.Len())
	}
	if rec := get(t, a, "POST", "/api/x", "", map[string]string{"If-None-Match": `"v1"`}); rec.Code != 201 {
		t.Fatalf("POST: %d", rec.Code)
	}
	if rec := get(t, a, "GET", "/vazio", "", map[string]string{"If-None-Match": "*"}); rec.Code != 200 {
		t.Fatalf("nothing declared: %d", rec.Code)
	} else if rec.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatal(rec.Header().Get("Cache-Control"))
	}
}

func TestLastModifiedAnswers304(t *testing.T) {
	a := cacheApp(t)

	rec := get(t, a, "GET", "/data", "", nil)
	if rec.Code != 200 || rec.Header().Get("Last-Modified") != modified.Format(http.TimeFormat) {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Last-Modified"))
	}

	same := get(t, a, "GET", "/data", "", map[string]string{"If-Modified-Since": modified.Format(http.TimeFormat)})
	if same.Code != http.StatusNotModified || same.Body.Len() != 0 {
		t.Fatalf("same date: %d", same.Code)
	}
	later := get(t, a, "GET", "/data", "", map[string]string{"If-Modified-Since": modified.Add(time.Hour).Format(http.TimeFormat)})
	if later.Code != http.StatusNotModified {
		t.Fatalf("later date: %d", later.Code)
	}
	before := get(t, a, "GET", "/data", "", map[string]string{"If-Modified-Since": modified.Add(-time.Hour).Format(http.TimeFormat)})
	if before.Code != 200 || before.Body.Len() == 0 {
		t.Fatalf("earlier date: %d", before.Code)
	}
	// Lixo no cabeçalho é ausência de pergunta, não erro.
	if rec := get(t, a, "GET", "/data", "", map[string]string{"If-Modified-Since": "ontem"}); rec.Code != 200 {
		t.Fatalf("unparseable date: %d", rec.Code)
	}
}

// RFC 9110: quando os dois validadores estão na mesa, quem decide é o ETag.
func TestETagWinsOverTheDate(t *testing.T) {
	a := cacheApp(t)
	rec := get(t, a, "GET", "/ambos", "", map[string]string{
		"If-None-Match":     `"v0"`,
		"If-Modified-Since": modified.Format(http.TimeFormat),
	})
	if rec.Code != 200 || rec.Body.Len() == 0 {
		t.Fatalf("a data não pode salvar uma etiqueta velha: %d", rec.Code)
	}
	// O cabeçalho continua sendo escrito — ele é informação; o que ele não
	// faz é decidir por cima do ETag.
	if rec.Header().Get("Last-Modified") != modified.Format(http.TimeFormat) {
		t.Fatal(rec.Header().Get("Last-Modified"))
	}
}
