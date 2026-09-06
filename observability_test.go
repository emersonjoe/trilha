package trilha

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// FR-005: probes are never rate limited; the app routes still are.
func TestProbesSkipTheRateLimiter(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), RateLimit: RateLimit{RPS: 1, Burst: 1},
		Observability: Observability{Metrics: "/_trilha/metrics"}})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.Text(200, "ok") }}})
	for i := 0; i < 20; i++ {
		if rec := get(t, a, "GET", "/_trilha/health/live", "", nil); rec.Code != 200 {
			t.Fatalf("probe %d got %d", i, rec.Code)
		}
	}
	get(t, a, "GET", "/", "", nil)
	if rec := get(t, a, "GET", "/", "", nil); rec.Code != 429 {
		t.Fatalf("app routes must stay limited: %d", rec.Code)
	}
}

// FR-006: probes must not drown the audit log.
func TestProbesAreLoggedAtDebug(t *testing.T) {
	var buf strings.Builder
	lg := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	a := New(Config{Env: Prod, Logger: lg, Secret: []byte(obsTok),
		Observability: Observability{Metrics: "/_trilha/metrics", Token: obsTok}})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.Text(200, "ok") }}})
	get(t, a, "GET", "/_trilha/health/live", "", nil)
	get(t, a, "GET", "/_trilha/health/ready", "", nil)
	get(t, a, "GET", "/_trilha/metrics", "", authed())
	if strings.Contains(buf.String(), "msg=request") {
		t.Fatalf("probes logged as requests: %s", buf.String())
	}
	get(t, a, "GET", "/", "", nil)
	if !strings.Contains(buf.String(), "msg=request") || !strings.Contains(buf.String(), `path=/ `) {
		t.Fatalf("normal requests must still be logged: %s", buf.String())
	}
}

// US5: W3C Trace Context propagation and a request-scoped logger.
func TestTraceContextAndRequestLogger(t *testing.T) {
	var buf strings.Builder
	lg := slog.New(slog.NewTextHandler(&buf, nil))
	a := New(Config{Env: Prod, Logger: lg})
	a.Register(Route{Pattern: "/x", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		c.Log().Info("dentro")
		return c.Text(200, c.TraceID())
	}}})
	const tp = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	rec := get(t, a, "GET", "/x", "", map[string]string{"traceparent": tp})
	if rec.Body.String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatal(rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
	out := buf.String()
	if strings.Count(out, "trace_id=4bf92f3577b34da6a3ce929d0e0e4736") != 2 {
		t.Fatalf("trace id must reach the handler log and the request log:\n%s", out)
	}
	if !strings.Contains(out, "msg=dentro") || !strings.Contains(out, "request_id=") {
		t.Fatal(out)
	}
	// A malformed traceparent is ignored, never propagated.
	buf.Reset()
	rec = get(t, a, "GET", "/x", "", map[string]string{"traceparent": "lixo"})
	if rec.Body.String() != "" || strings.Contains(buf.String(), "trace_id") {
		t.Fatalf("%q %s", rec.Body.String(), buf.String())
	}
}

func TestObservabilityTokenIsRejectedWhenTooShort(t *testing.T) {
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Observability: Observability{Metrics: "/m", Token: "curto"}})
	if rec := get(t, a, "GET", "/m", "", map[string]string{"Authorization": "Bearer curto"}); rec.Code != 401 {
		t.Fatalf("a short token must not authorize anything: %d", rec.Code)
	}
}

func TestObservabilityFromEnv(t *testing.T) {
	t.Setenv("TRILHA_OBS_TOKEN", "0123456789abcdef0123456789abcdef")
	t.Setenv("TRILHA_METRICS", "/metricas")
	cfg := ConfigFromEnv()
	if cfg.Observability.Token == "" || cfg.Observability.Metrics != "/metricas" {
		t.Fatalf("%+v", cfg.Observability)
	}
}

func TestHealthEndpointsDoNotSetCookiesOrCSRF(t *testing.T) {
	a := obsApp(t, Prod, Observability{})
	a.Check("x", func(context.Context) error { return nil })
	rec := get(t, a, "GET", "/_trilha/health", "", nil)
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("a probe must not create session state")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("hardening headers still apply")
	}
}

// Spec 055 (#42): the access record carries both the concrete path and the
// route template. An app with an id in the URL writes one path per record and
// one route per screen; whoever investigates a case wants the first, whoever
// aggregates wants the second, and rebuilding the second from the first with a
// regular expression outside the app is the cardinality problem this exists to
// avoid. What the fallback answered has no template, and says so with an empty
// field rather than by inventing one out of user input.
func TestAccessLogCarriesPathAndRoute(t *testing.T) {
	var buf strings.Builder
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(&buf, nil))})
	a.Register(Route{Pattern: "/blog/{slug}", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		return c.Text(200, c.Pattern())
	}}})
	rec := get(t, a, "GET", "/blog/ola-mundo", "", nil)
	if rec.Body.String() != "/blog/{slug}" {
		t.Fatalf("c.Pattern() = %q", rec.Body.String())
	}
	line := buf.String()
	if !strings.Contains(line, "path=/blog/ola-mundo") || !strings.Contains(line, "route=/blog/{slug}") {
		t.Fatalf("access record without both fields: %s", line)
	}
	buf.Reset()
	if rec := get(t, a, "GET", "/nao-existe", "", nil); rec.Code != 404 {
		t.Fatalf("expected the fallback: %d", rec.Code)
	}
	if strings.Contains(buf.String(), "route=/") {
		t.Fatalf("the fallback has no template to report: %s", buf.String())
	}
}

// Spec 055 (#42): the template is there from the first middleware, which is
// what a bridge to code that already exists needs — it runs before the handler
// and used to have to be told its own route as a string.
func TestPatternIsReadableFromMiddleware(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet()})
	var seen string
	a.Register(Route{
		Pattern:     "/v/{viagemId}/orcamento",
		Middlewares: []MiddlewareFunc{func(c *Ctx, next Next) error { seen = c.Pattern(); return next() }},
		Methods:     map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.Text(200, "ok") }},
	})
	get(t, a, "GET", "/v/cmtkldayg000527g3e2mkw51p/orcamento", "", nil)
	if seen != "/v/{viagemId}/orcamento" {
		t.Fatalf("middleware saw %q", seen)
	}
}
