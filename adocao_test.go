package trilha

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type ctxKey struct{}

// #9: a middleware puts a value in the request context; stdlib-style code reads it.
func TestSetContextAndRequest(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/x",
		Middlewares: []MiddlewareFunc{func(c *Ctx, next Next) error {
			c.SetContext(context.WithValue(c.Context(), ctxKey{}, c.Nonce()))
			return next()
		}},
		Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
			v, _ := c.Request().Context().Value(ctxKey{}).(string)
			if v == "" || v != c.Nonce() {
				return Errorf(500, "context value lost")
			}
			r := c.Request().Clone(c.Context())
			r.URL.Path = "/rewritten"
			c.SetRequest(r)
			return c.Text(200, c.Request().URL.Path)
		}},
	})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 200 || rec.Body.String() != "/rewritten" {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

// #7: NoTimeout maps to 0; zero keeps the default.
func TestNoTimeout(t *testing.T) {
	if or(NoTimeout, 30*time.Second) != 0 || or(0, 30*time.Second) != 30*time.Second || or(time.Second, time.Minute) != time.Second {
		t.Fatal("or() mapping")
	}
}

// #8: static cache control and per-file headers.
func TestStaticHeaders(t *testing.T) {
	pub := fstest.MapFS{"app.css": {Data: []byte("body{}")}, "robots.txt": {Data: []byte("User-agent: *")}}
	a := New(Config{Logger: quiet(), Env: Prod, Public: pub, StaticCacheControl: "public, max-age=31536000, immutable",
		StaticHeaders: func(name string, h http.Header) {
			if name == "robots.txt" {
				h.Set("Cache-Control", "no-store")
			}
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
		}})
	get := func(p string) http.Header {
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != 200 {
			t.Fatal(p, rec.Code)
		}
		return rec.Header()
	}
	if h := get("/app.css"); h.Get("Cache-Control") != "public, max-age=31536000, immutable" || h.Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Fatal(h)
	}
	if h := get("/robots.txt"); h.Get("Cache-Control") != "no-store" {
		t.Fatal(h)
	}
	a.Config().Env = Dev
	if h := get("/app.css"); h.Get("Cache-Control") != "no-cache" {
		t.Fatal("dev must not cache:", h)
	}
}

// #6: derived fields changed in Setup apply once serving starts.
func TestConfigReappliedAfterSetup(t *testing.T) {
	a := New(Config{Logger: quiet(), Env: Prod})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		c.App().Logger().Info("hello from handler")
		if err := c.SetSigned("s", "v", time.Hour); err != nil {
			return err
		}
		return c.Text(200, "ok")
	}}})
	// "Setup": swap logger, enable rate limit, set a secret.
	var buf bytes.Buffer
	cfg := a.Config()
	cfg.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	cfg.RateLimit = RateLimit{RPS: 1, Burst: 1}
	cfg.Secret = []byte(strings.Repeat("k", MinSecretLen))
	h := a.Handler() // reapplies
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || len(rec.Result().Cookies()) != 1 {
		t.Fatal("signed cookie needs the secret set in Setup:", rec.Code, rec.Result().Cookies())
	}
	if !strings.Contains(buf.String(), `"msg":"hello from handler"`) {
		t.Fatal("logger set in Setup not used:", buf.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 429 {
		t.Fatal("rate limit set in Setup not applied:", rec.Code)
	}
}
