package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func TestTypedGetWrongTypeIsAMiss(t *testing.T) {
	c := New(Options{})
	c.Set(Key{Name: "n"}, 42)
	if v, ok := Get[int](c, "n"); !ok || v != 42 {
		t.Fatalf("Get[int] = %v, %v", v, ok)
	}
	if v, ok := Get[string](c, "n"); ok || v != "" {
		t.Fatalf("wrong type must be a miss, got %q, %v", v, ok)
	}
	if v, ok := Get[int](c, "nada"); ok || v != 0 {
		t.Fatalf("absent = %v, %v", v, ok)
	}
}

func TestDoCachesAndDoesNotCacheErrors(t *testing.T) {
	c := New(Options{})
	var calls atomic.Int64
	fetch := func(context.Context) (int, error) { calls.Add(1); return 7, nil }
	k := Key{Name: "n", TTL: time.Minute, Tags: []string{"t"}}
	for i := 0; i < 3; i++ {
		if v, err := Do(context.Background(), c, k, fetch); err != nil || v != 7 {
			t.Fatalf("Do = %v, %v", v, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("fetched %d times, want 1", calls.Load())
	}
	c.Invalidate("t")
	if _, err := Do(context.Background(), c, k, fetch); err != nil || calls.Load() != 2 {
		t.Fatalf("after Invalidate: %d calls, err %v", calls.Load(), err)
	}

	boom := errors.New("boom")
	var tries atomic.Int64
	fail := func(context.Context) (int, error) { tries.Add(1); return 0, boom }
	for i := 0; i < 2; i++ {
		if _, err := Do(context.Background(), c, Key{Name: "ruim"}, fail); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	}
	if tries.Load() != 2 {
		t.Fatal("an error must not be cached")
	}
}

// FR-007: one flight per key. Without this, the first request after an
// Invalidate is a stampede on the database.
func TestDoHasOneFlightPerKey(t *testing.T) {
	c := New(Options{})
	var calls atomic.Int64
	release := make(chan struct{})
	slow := func(context.Context) (int, error) {
		calls.Add(1)
		<-release
		return 1, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if v, err := Do(context.Background(), c, Key{Name: "n", TTL: time.Minute}, slow); err != nil || v != 1 {
				t.Errorf("Do = %v, %v", v, err)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Fatalf("fetched %d times, want 1", n)
	}
}

// A Do inside a Do is ordinary (a page caches a list, an item caches itself):
// the lock must not be held while fn runs.
func TestDoNests(t *testing.T) {
	c := New(Options{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		v, err := Do(context.Background(), c, Key{Name: "fora"}, func(ctx context.Context) (int, error) {
			inner, err := Do(ctx, c, Key{Name: "dentro"}, func(context.Context) (int, error) { return 2, nil })
			return inner + 1, err
		})
		if err != nil || v != 3 {
			t.Errorf("Do = %v, %v", v, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("nested Do deadlocked")
	}
}

// FR-008: Once is per request, and that is the whole point — the value must
// not survive into the next one.
func TestOnceIsPerRequest(t *testing.T) {
	var calls atomic.Int64
	app := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		for i := 0; i < 2; i++ {
			v, err := Once(c, "u", func() (string, error) { calls.Add(1); return "ana", nil })
			if err != nil || v != "ana" {
				t.Errorf("Once = %q, %v", v, err)
			}
		}
		// A different name is a different question.
		if _, err := Once(c, "outro", func() (string, error) { calls.Add(1); return "x", nil }); err != nil {
			t.Error(err)
		}
		return h.Div(), nil
	}})
	for i := 0; i < 2; i++ {
		app.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}
	if n := calls.Load(); n != 4 {
		t.Fatalf("%d fetches, want 4 (two per request)", n)
	}
}

func TestOnceKeepsTheError(t *testing.T) {
	boom := errors.New("boom")
	app := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var calls int
	app.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		for i := 0; i < 2; i++ {
			if _, err := Once(c, "u", func() (int, error) { calls++; return 0, boom }); !errors.Is(err, boom) {
				t.Errorf("err = %v", err)
			}
		}
		return h.Div(), nil
	}})
	app.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if calls != 1 {
		t.Fatalf("%d calls: the answer to the question is the error too", calls)
	}
}
