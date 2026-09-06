package trilha

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// TestPollHeaderTalksBackToTheClock covers the only thing the server says to a
// polling fragment: come back later, come back sooner, do not come back.
func TestPollHeaderTalksBackToTheClock(t *testing.T) {
	cases := []struct {
		name string
		do   func(*Ctx)
		want string
	}{
		{"stop", func(c *Ctx) { c.PollStop() }, "stop"},
		{"slower", func(c *Ctx) { c.PollEvery(30 * time.Second) }, "30s"},
		{"a minute", func(c *Ctx) { c.PollEvery(time.Minute) }, "60s"},
		{"never under a second", func(c *Ctx) { c.PollEvery(10 * time.Millisecond) }, "1s"},
		{"quiet", func(c *Ctx) {}, ""},
	}
	for _, tc := range cases {
		a := New(Config{Logger: quiet()})
		a.Register(Route{Pattern: "/status", Page: func(c *Ctx) (h.Node, error) {
			tc.do(c)
			return h.Div(h.ID("status"), h.Text("running")), nil
		}})
		req := httptest.NewRequest("GET", "/status", nil)
		req.Header.Set("Trilha-Fragment", "status")
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		if got := rec.Header().Get("Trilha-Poll"); got != tc.want {
			t.Errorf("%s: Trilha-Poll = %q, want %q", tc.name, got, tc.want)
		}
		// The fragment is still a fragment: the answer is the piece, and the
		// cache is told what the piece depends on.
		if body := rec.Body.String(); strings.Contains(body, "<html") {
			t.Errorf("%s: whole page in a fragment answer: %s", tc.name, body)
		}
		if v := rec.Header().Get("Vary"); !strings.Contains(v, "Trilha-Fragment") {
			t.Errorf("%s: Vary = %q", tc.name, v)
		}
	}
}

func TestStreamNotifySendsANameAndNoData(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/events", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		return c.Stream().Notify("doc:42")
	}}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/events", nil))
	if got, want := rec.Body.String(), "event: doc:42\ndata: \n\n"; got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}
