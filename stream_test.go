package trilha

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStream(t *testing.T) {
	app := New(Config{Logger: quiet()})
	app.Register(Route{Pattern: "/sse", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		s := c.Stream()
		if err := s.Send("delta", "olá\nmundo"); err != nil {
			return err
		}
		if err := s.JSON("", map[string]int{"n": 1}); err != nil {
			return err
		}
		if err := s.Comment("ping"); err != nil {
			return err
		}
		return s.Send("done", "")
	}}})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/sse", nil))
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatal(rec.Code, rec.Header())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
		t.Fatal(cc)
	}
	want := "event: delta\ndata: olá\ndata: mundo\n\ndata: {\"n\":1}\n\n: ping\n\nevent: done\ndata: \n\n"
	if rec.Body.String() != want {
		t.Fatalf("got %q", rec.Body.String())
	}
	if !rec.Flushed {
		t.Fatal("stream must flush")
	}
}
