package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// A fn that panics must not leave its name in flight: the recover at the
// edge of the request saves the process, and every later Do on that key
// would otherwise wait forever.
func TestDoPanicReleasesTheKey(t *testing.T) {
	c := New(Options{})
	k := Key{Name: "n"}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic must reach the caller")
			}
		}()
		Do(context.Background(), c, k, func(context.Context) (int, error) { panic("boom") })
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if v, err := Do(context.Background(), c, k, func(context.Context) (int, error) { return 7, nil }); err != nil || v != 7 {
			t.Errorf("Do after panic = %v, %v", v, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Do after a panicking fn hangs: the key stayed in flight")
	}
}

// Whoever waits on a fn that panics gets an error, not a hang and not a zero
// value dressed as success.
func TestDoPanicIsAnErrorForWaiters(t *testing.T) {
	c := New(Options{})
	k := Key{Name: "n"}
	started, release := make(chan struct{}), make(chan struct{})
	go func() {
		defer func() { recover() }()
		Do(context.Background(), c, k, func(context.Context) (int, error) {
			close(started)
			<-release
			panic("boom")
		})
	}()
	<-started
	errc := make(chan error, 1)
	go func() {
		_, err := Do(context.Background(), c, k, func(context.Context) (int, error) { return 1, nil })
		errc <- err
	}()
	waitFlightWaiter(t, c)
	close(release)
	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "panic") {
			t.Fatalf("waiter err = %v, want an error naming the panic", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter hangs on a fn that panicked")
	}
}

// An Invalidate that lands while fn is reading the old data must win: caching
// what fn returns would bring back the value the write just dropped, for the
// whole TTL.
func TestDoInvalidateDuringFlightIsNotCached(t *testing.T) {
	for name, drop := range map[string]func(*Cache){
		"Invalidate": func(c *Cache) { c.Invalidate("posts") },
		"Delete":     func(c *Cache) { c.Delete("n") },
		"Clear":      func(c *Cache) { c.Clear() },
	} {
		t.Run(name, func(t *testing.T) {
			c := New(Options{})
			k := Key{Name: "n", TTL: time.Hour, Tags: []string{"posts"}}
			v, err := Do(context.Background(), c, k, func(context.Context) (string, error) {
				drop(c) // the write happens after the read started
				return "stale", nil
			})
			if err != nil || v != "stale" {
				t.Fatalf("Do = %q, %v", v, err)
			}
			if got, ok := Get[string](c, "n"); ok {
				t.Fatalf("the value read before the %s was cached: %q", name, got)
			}
		})
	}
}

// The leader's request going away (the client closed the tab) must not fail
// the others waiting on the same key: they ask again with their own context.
func TestDoLeaderCancelDoesNotFailWaiters(t *testing.T) {
	c := New(Options{})
	k := Key{Name: "n"}
	leaderCtx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go Do(leaderCtx, c, k, func(ctx context.Context) (int, error) {
		close(started)
		<-ctx.Done()
		return 0, ctx.Err()
	})
	<-started
	res := make(chan error, 1)
	go func() {
		v, err := Do(context.Background(), c, k, func(context.Context) (int, error) { return 9, nil })
		if err == nil && v != 9 {
			err = errors.New("wrong value")
		}
		res <- err
	}()
	waitFlightWaiter(t, c)
	cancel()
	select {
	case err := <-res:
		if err != nil {
			t.Fatalf("a waiter with a live context got %v from the leader's cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiter hangs")
	}
}

// A waiter whose own request ends stops waiting.
func TestDoWaiterHonoursItsContext(t *testing.T) {
	c := New(Options{})
	k := Key{Name: "n"}
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	go Do(context.Background(), c, k, func(context.Context) (int, error) {
		close(started)
		<-release
		return 1, nil
	})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Do(ctx, c, k, func(context.Context) (int, error) { return 2, nil }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the waiter's own deadline", err)
	}
}

// Once has the same flight: a panic in fn must not hang the next component
// of the same request that asks.
func TestOncePanicReleasesTheName(t *testing.T) {
	app := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		func() {
			defer func() { recover() }()
			Once(c, "u", func() (int, error) { panic("boom") })
		}()
		done := make(chan error, 1)
		go func() {
			_, err := Once(c, "u", func() (int, error) { return 1, nil })
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "panic") {
				t.Errorf("Once after panic err = %v, want the remembered panic", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Once after a panicking fn hangs")
		}
		return h.Div(), nil
	}})
	app.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

// waitFlightWaiter gives the waiter goroutine time to reach the in-flight
// call; there is no hook to observe it, and the assertions do not depend on
// the timing, only the coverage of the waiting branch does.
func waitFlightWaiter(t *testing.T, c *Cache) {
	t.Helper()
	time.Sleep(20 * time.Millisecond)
}
