package cache

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

const tok = "0123456789abcdef0123456789abcdef"

// FR-009: a cache nobody can see is a cache nobody can size. The four series
// carry the cache name as a label, so two caches in one app stay apart.
func TestMetricsExposesTheFourSeries(t *testing.T) {
	app := trilha.New(trilha.Config{
		Env:           trilha.Prod,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Observability: trilha.Observability{Metrics: "/_trilha/metrics", Token: tok},
	})
	c := New(Options{Name: "posts", MaxEntries: 2, Metrics: app.Metrics()})

	c.Get("nada")                              // miss
	c.Set(Key{Name: "a", TTL: time.Minute}, 1) // entry
	c.Get("a")                                 // hit
	c.Set(Key{Name: "b"}, 2)                   //
	c.Set(Key{Name: "c"}, 3)                   // evicts "a" or "b"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/_trilha/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("scrape: %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`trilha_cache_hits_total{cache="posts"} 1`,
		`trilha_cache_misses_total{cache="posts"} 1`,
		`trilha_cache_evictions_total{cache="posts"} 1`,
		`trilha_cache_entries{cache="posts"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in\n%s", want, body)
		}
	}
}

// The framework speaks English everywhere else; /metrics is read by people too.
func TestHelpTextIsEnglish(t *testing.T) {
	app := trilha.New(trilha.Config{
		Env:           trilha.Prod,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Observability: trilha.Observability{Metrics: "/_trilha/metrics", Token: tok},
	})
	New(Options{Name: "posts", Metrics: app.Metrics()})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/_trilha/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	app.Handler().ServeHTTP(rec, req)
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "# HELP ") {
			continue
		}
		for _, pt := range []string{"ções", "Duração", "Pânicos", "instante", "rota"} {
			if strings.Contains(line, pt) {
				t.Fatalf("HELP still in Portuguese: %s", line)
			}
		}
	}
}
