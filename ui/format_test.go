package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// inApp renders through a real app, which is where the language and the zone
// come from — and the reason these take a Ctx at all.
func inApp(t *testing.T, cfg trilha.Config, build func(*trilha.Ctx) h.Node) string {
	t.Helper()
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	a := trilha.New(cfg)
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, build(c))
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	return out
}

var moment = time.Date(2026, 9, 8, 15, 4, 5, 0, time.UTC)

// The two languages, side by side: this is the whole feature, and a golden is
// the honest way to pin it.
func TestFormattersInBothLanguages(t *testing.T) {
	for _, c := range []struct {
		lang, zone string
		date       string
		bytes      string
		number     string
	}{
		{"", "", "Sep 8, 2026 3:04 PM", "1.4 MB", "12,345.68"},
		{"pt-BR", "America/Sao_Paulo", "08/09/2026 12:04", "1,4 MB", "12.345,68"},
	} {
		cfg := trilha.Config{Locale: c.lang, TimeZone: c.zone}
		got := inApp(t, cfg, func(ctx *trilha.Ctx) h.Node {
			return h.Div(
				Date(ctx, moment),
				Bytes(ctx, 1_400_000),
				Number(ctx, 12345.678, Decimals(2)),
			)
		})
		for _, want := range []string{c.date, c.bytes, c.number} {
			if !strings.Contains(got, want) {
				t.Errorf("locale %q: falta %q em %s", c.lang, want, got)
			}
		}
		// The machine-readable value is the instant, never the local text.
		if !strings.Contains(got, `datetime="2026-09-08T15:04:05Z"`) {
			t.Errorf("locale %q: o datetime não é o instante: %s", c.lang, got)
		}
	}
}

// What the beginner gets wrong, and what this exists to stop: a zero date
// printing as the first day of year one.
func TestFormattersShowADashInsteadOfNonsense(t *testing.T) {
	var nilTime *time.Time
	got := inApp(t, trilha.Config{}, func(ctx *trilha.Ctx) h.Node {
		return h.Div(
			Date(ctx, time.Time{}), Date(ctx, nilTime),
			Bytes(ctx, 0), Duration(ctx, 0), Number(ctx, "não é número"),
		)
	})
	if strings.Contains(got, "0001") || strings.Contains(got, "1970") {
		t.Fatalf("uma data zero virou uma data: %s", got)
	}
	if n := strings.Count(got, "—"); n != 5 {
		t.Fatalf("esperava cinco travessões, veio %d: %s", n, got)
	}
}

// A zone the machine does not carry has to fall back loudly. Falling back in
// silence would shift every timestamp on the screen and nothing would look
// broken.
func TestUnknownTimeZoneFallsBackToUTC(t *testing.T) {
	var logged strings.Builder
	a := trilha.New(trilha.Config{TimeZone: "Mars/Olympus",
		Logger: slog.New(slog.NewTextHandler(&logged, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, Date(c, moment))
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if !strings.Contains(out, "3:04 PM") {
		t.Fatalf("não caiu para UTC: %s", out)
	}
	if !strings.Contains(logged.String(), "Mars/Olympus") {
		t.Fatalf("caiu para UTC em silêncio: %s", logged.String())
	}
}

func TestRelativeKeepsTheAbsoluteWithin(t *testing.T) {
	got := inApp(t, trilha.Config{Locale: "pt-BR"}, func(ctx *trilha.Ctx) h.Node {
		return Date(ctx, time.Now().Add(-3*time.Minute), Relative())
	})
	if !strings.Contains(got, "há 3 min") {
		t.Errorf("relativo = %s", got)
	}
	// The absolute stays in the title: the relative is a summary, not the fact.
	if !strings.Contains(got, "title=") || !strings.Contains(got, "datetime=") {
		t.Errorf("o relativo apagou o absoluto: %s", got)
	}
}

func TestDurationReadsLikeAPersonSaysIt(t *testing.T) {
	got := inApp(t, trilha.Config{}, func(ctx *trilha.Ctx) h.Node {
		return h.Div(
			Duration(ctx, 340*time.Millisecond),
			Duration(ctx, 133*time.Second),
			Duration(ctx, 65*time.Minute),
		)
	})
	for _, want := range []string{"340 ms", "2 min 13 s", "1 h 5 min"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q em %s", want, got)
		}
	}
}
