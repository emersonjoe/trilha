package trilha

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

type hookKey struct{}

// The hook of Config.OnRequest sees every request that reached a route: the
// route template, the status that was written, and one call of next.
func TestOnRequestHookSeesPatternAndStatus(t *testing.T) {
	var calls int
	var pattern, value string
	var status int
	cfg := Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cfg.OnRequest = func(c *Ctx, next func() int) {
		calls++
		pattern = c.Pattern()
		// A value put in the context here is what a tracing module does with
		// a span; it has to reach the handler.
		c.SetContext(context.WithValue(c.Context(), hookKey{}, "span"))
		status = next()
	}
	a := New(cfg)
	a.Register(Route{Pattern: "/p/{id}", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		v, _ := c.Context().Value(hookKey{}).(string)
		value = v
		return c.Text(201, "ok")
	}}})
	rec := get(t, a, "GET", "/p/7", "", nil)
	if rec.Code != 201 || calls != 1 || status != 201 {
		t.Fatalf("code=%d calls=%d status=%d", rec.Code, calls, status)
	}
	if pattern != "/p/{id}" {
		t.Fatalf("the hook must see the route template, not the concrete path: %q", pattern)
	}
	if value != "span" {
		t.Fatalf("the context set by the hook must reach the handler: %q", value)
	}
}

// A panic in the handler is still a 500 for the hook: the recover is inside
// next, so the hook is never unwound.
func TestOnRequestHookSeesPanicAsFiveHundred(t *testing.T) {
	var status int
	cfg := Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cfg.OnRequest = func(c *Ctx, next func() int) { status = next() }
	a := New(cfg)
	a.Register(Route{Pattern: "/boom", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		panic("no")
	}}})
	rec := get(t, a, "GET", "/boom", "", nil)
	if rec.Code != 500 || status != 500 {
		t.Fatalf("code=%d status=%d", rec.Code, status)
	}
}

// A hook that forgets to call next does not get to swallow the request.
func TestOnRequestHookThatSkipsNextStillAnswers(t *testing.T) {
	cfg := Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cfg.OnRequest = func(c *Ctx, next func() int) {}
	a := New(cfg)
	a.Register(Route{Pattern: "/x", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		return c.Text(200, "ok")
	}}})
	if rec := get(t, a, "GET", "/x", "", nil); rec.Code != 200 || rec.Body.String() != "ok" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
}

// A second call of next reports the status instead of running the handler
// twice: one request, one answer.
func TestOnRequestHookCallingNextTwiceRunsTheHandlerOnce(t *testing.T) {
	var runs int
	var first, second int
	cfg := Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cfg.OnRequest = func(c *Ctx, next func() int) { first, second = next(), next() }
	a := New(cfg)
	a.Register(Route{Pattern: "/x", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		runs++
		return c.Text(202, "ok")
	}}})
	get(t, a, "GET", "/x", "", nil)
	if runs != 1 || first != 202 || second != 202 {
		t.Fatalf("runs=%d first=%d second=%d", runs, first, second)
	}
}
