package trilha

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A probe is the question "would this caller reach the handler?": the chain
// runs, the handler does not, and neither the access log nor the metrics
// hear about it.
func TestProbeRunsTheChainAndNotTheHandler(t *testing.T) {
	var logs bytes.Buffer
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	ran := false
	guard := func(c *Ctx, next Next) error {
		if c.Request().Header.Get("Authorization") != "Bearer ok" {
			return &HTTPError{Code: http.StatusUnauthorized, Message: "a key is required"}
		}
		if !c.Probing() {
			t.Error("the middleware should see that it is being probed")
		}
		return next()
	}
	a.Register(Route{Pattern: "/api/items", Kind: KindAPI, Middlewares: []MiddlewareFunc{guard},
		Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { ran = true; return c.Text(200, "x") }}})

	req := httptest.NewRequest("GET", "/api/items", nil)
	if a.Probe(req) {
		t.Fatal("without the key the probe should say no")
	}
	req.Header.Set("Authorization", "Bearer ok")
	if !a.Probe(req) {
		t.Fatal("with the key the probe should say yes")
	}
	if ran {
		t.Fatal("the handler ran during a probe")
	}
	if a.Probe(httptest.NewRequest("GET", "/nowhere", nil)) {
		t.Fatal("an unknown route is not reachable")
	}
	if a.Probe(httptest.NewRequest("POST", "/api/items", nil)) {
		t.Fatal("a method the route does not answer is not reachable")
	}
	if strings.Contains(logs.String(), "/api/items") {
		t.Fatalf("a probe must not reach the access log:\n%s", logs.String())
	}
	// Outside a probe nobody is probing.
	a.Register(Route{Pattern: "/plain", Kind: KindAPI, Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		if c.Probing() {
			t.Error("a real request is not a probe")
		}
		return c.Text(200, "x")
	}}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/plain", nil))
}

// A request built by a bridge — an MCP tool call, a job — says how it
// arrived, and the actor recognised inside carries that, not the mechanism
// that recognised it.
func TestWithViaOverridesTheActorsVia(t *testing.T) {
	s := &sink{}
	a := auditApp(t, s, nil, func(c *Ctx) error {
		if c.Param("id") == "42" && c.Actor().Via != "mcp" {
			t.Errorf("Via inside the route = %q", c.Actor().Via)
		}
		c.Audit("documento.excluiu", c.Param("id"))
		return c.Text(200, "ok")
	}, func(c *Ctx, next Next) error {
		c.SetActor(Actor{Subject: "key:k1", Via: "api_key"})
		return next()
	})
	req := WithVia(httptest.NewRequest("DELETE", "/docs/42", nil), "mcp")
	a.Handler().ServeHTTP(httptest.NewRecorder(), req)
	if len(s.recs) != 1 || s.recs[0].Actor.Via != "mcp" || s.recs[0].Actor.Subject != "key:k1" {
		t.Fatalf("records = %+v", s.recs)
	}
	// Without the mark, the mechanism's own word stands.
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("DELETE", "/docs/43", nil))
	if len(s.recs) != 2 || s.recs[1].Actor.Via != "api_key" {
		t.Fatalf("records = %+v", s.recs)
	}
}

func TestRouteReturnsTheRegisteredRoute(t *testing.T) {
	a := New(Config{})
	a.Register(Route{Pattern: "/api/items/", Kind: KindAPI, Methods: map[string]HandlerFunc{"GET": nil, "POST": nil}})
	r, ok := a.Route("/api/items")
	if !ok || r.Kind != KindAPI || len(r.Methods) != 2 {
		t.Fatalf("route = %+v, %v", r, ok)
	}
	if _, ok := a.Route("/nope"); ok {
		t.Fatal("unknown pattern found")
	}
}

// A subtree limit is a middleware like any other, and the probe that lists
// tools for a caller must not spend the caller's tokens.
func TestProbeTakesNoTokenFromASubtreeLimit(t *testing.T) {
	a := New(Config{Env: Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	a.Register(Route{Pattern: "/api/items", Kind: KindAPI, Middlewares: []MiddlewareFunc{Limit(0, 1)},
		Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error { return c.Text(200, "x") }}})
	for i := 0; i < 5; i++ {
		if !a.Probe(httptest.NewRequest("GET", "/api/items", nil)) {
			t.Fatalf("probe %d was refused", i+1)
		}
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/items", nil))
	if rec.Code != 200 {
		t.Fatalf("the real call after the probes should pass: %d", rec.Code)
	}
}
