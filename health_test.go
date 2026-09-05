package trilha

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func obsApp(t *testing.T, env Env, o Observability) *App {
	t.Helper()
	a := New(Config{Env: env, Logger: quiet(), Observability: o})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.Text(200, "ok") }}})
	return a
}

// US1: live never runs checks; ready reflects them.
func TestLiveIsAlwaysUpAndReadyFollowsChecks(t *testing.T) {
	ran := 0
	a := obsApp(t, Prod, Observability{})
	a.Check("banco", func(context.Context) error { ran++; return errors.New("conexão recusada") })

	rec := get(t, a, "GET", "/_trilha/health/live", "", nil)
	if rec.Code != 200 || ran != 0 {
		t.Fatalf("live: %d, verificações rodadas %d", rec.Code, ran)
	}
	if ct := rec.Header().Get("Content-Type"); ct != healthMediaType {
		t.Fatal(ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatal(cc)
	}
	if rec.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatal("health must not be indexed")
	}

	rec = get(t, a, "GET", "/_trilha/health/ready", "", nil)
	if rec.Code != 503 || ran != 1 {
		t.Fatalf("ready: %d, rodadas %d", rec.Code, ran)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("a failing readiness must tell the balancer when to retry")
	}
}

// US2: an anonymous client learns pass/fail and nothing else.
func TestAnonymousHealthLeaksNothing(t *testing.T) {
	a := obsApp(t, Prod, Observability{})
	a.Check("postgres-financeiro", func(context.Context) error { return errors.New("dial tcp 10.0.0.7:5432: refused") })
	rec := get(t, a, "GET", "/_trilha/health", "", nil)
	body := rec.Body.String()
	if rec.Code != 503 {
		t.Fatal(rec.Code)
	}
	if strings.Contains(body, "postgres") || strings.Contains(body, "10.0.0.7") || strings.Contains(body, "refused") {
		t.Fatal("detail leaked to an anonymous client: " + body)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got["status"] != "fail" || len(got) != 1 {
		t.Fatalf("%v", got)
	}
}

func TestDetailWithTokenAndTrustedNetwork(t *testing.T) {
	const tok = obsTok
	a := obsApp(t, Prod, Observability{Token: tok})
	a.Check("banco", func(context.Context) error { return errors.New("conexão recusada") })

	if rec := get(t, a, "GET", "/_trilha/health", "", map[string]string{"Authorization": "Bearer errado"}); strings.Contains(rec.Body.String(), "banco") {
		t.Fatal("wrong token must not open the detail")
	}
	rec := get(t, a, "GET", "/_trilha/health", "", map[string]string{"Authorization": "Bearer " + tok})
	body := rec.Body.String()
	if rec.Code != 503 || !strings.Contains(body, "banco") || !strings.Contains(body, "conexão recusada") {
		t.Fatalf("%d %s", rec.Code, body)
	}

	// httptest's RemoteAddr is 192.0.2.1: a trusted network dispenses the token.
	b := obsApp(t, Prod, Observability{Token: tok, Trusted: []string{"192.0.2.0/24"}})
	b.Check("banco", func(context.Context) error { return nil })
	if rec := get(t, b, "GET", "/_trilha/health", "", nil); !strings.Contains(rec.Body.String(), "banco") {
		t.Fatal("trusted network should see the detail: " + rec.Body.String())
	}

	// Dev is open, and Details=Off closes it even for the right token.
	d := obsApp(t, Dev, Observability{})
	d.Check("banco", func(context.Context) error { return nil })
	if rec := get(t, d, "GET", "/_trilha/health", "", nil); !strings.Contains(rec.Body.String(), "banco") {
		t.Fatal("dev should show the detail")
	}
	off := obsApp(t, Prod, Observability{Token: tok, Details: Off})
	off.Check("banco", func(context.Context) error { return nil })
	if rec := get(t, off, "GET", "/_trilha/health", "", map[string]string{"Authorization": "Bearer " + tok}); strings.Contains(rec.Body.String(), "banco") {
		t.Fatal("Details=off must win over the token")
	}
}

// FR-004: a slow dependency cannot hold the probe open, and repeated probes
// do not repeat the work.
func TestCheckTimeoutAndCache(t *testing.T) {
	a := obsApp(t, Prod, Observability{Timeout: 20 * time.Millisecond, CacheFor: time.Minute, Token: "x"})
	a.Check("lento", func(ctx context.Context) error {
		select {
		case <-time.After(2 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	start := time.Now()
	rec := get(t, a, "GET", "/_trilha/health/ready", "", nil)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("probe waited %s", d)
	}
	if rec.Code != 503 {
		t.Fatal(rec.Code)
	}
	calls := 0
	b := obsApp(t, Prod, Observability{CacheFor: time.Minute})
	b.Check("conta", func(context.Context) error { calls++; return nil })
	for i := 0; i < 5; i++ {
		get(t, b, "GET", "/_trilha/health/ready", "", nil)
	}
	if calls != 1 {
		t.Fatalf("cached result should run the check once, ran %d", calls)
	}
	c := obsApp(t, Prod, Observability{CacheFor: NoTimeout})
	c.Check("conta", func(context.Context) error { calls++; return nil })
	get(t, c, "GET", "/_trilha/health/ready", "", nil)
	get(t, c, "GET", "/_trilha/health/ready", "", nil)
	if calls != 3 {
		t.Fatalf("CacheFor=NoTimeout disables the cache, got %d", calls)
	}
}

func TestHealthReportIsAvailableInGo(t *testing.T) {
	a := obsApp(t, Prod, Observability{CacheFor: NoTimeout})
	a.Check("ok", func(context.Context) error { return nil })
	a.Check("ruim", func(context.Context) error { return errors.New("x") })
	rep := a.HealthReport(context.Background())
	if rep.Status != "fail" || len(rep.Checks) != 2 {
		t.Fatalf("%+v", rep)
	}
	if rep.Checks[0].Name != "ok" || rep.Checks[0].Status != "pass" || rep.Checks[1].Error == "" {
		t.Fatalf("%+v", rep.Checks)
	}
}

func TestHealthCanBeDisabled(t *testing.T) {
	a := obsApp(t, Prod, Observability{Health: Off})
	if rec := get(t, a, "GET", "/_trilha/health", "", nil); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
}

func TestPanicInCheckBecomesFail(t *testing.T) {
	a := obsApp(t, Prod, Observability{Token: obsTok})
	a.Check("explode", func(context.Context) error { panic("boom") })
	rec := get(t, a, "GET", "/_trilha/health/ready", "", authed())
	if rec.Code != 503 || !strings.Contains(rec.Body.String(), "panic in check") {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
