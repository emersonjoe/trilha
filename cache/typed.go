package cache

import (
	"context"
	"sync"

	"github.com/emersonjoe/trilha"
)

// Get reads a value and checks its type. A value stored under another type is
// a miss, not a panic: a cache that crashes the app after a deploy that changed
// a struct is worse than a cache that goes to the database again.
func Get[T any](c *Cache, name string) (T, bool) {
	var zero T
	v, ok := c.Get(name)
	if !ok {
		return zero, false
	}
	t, ok := v.(T)
	if !ok {
		return zero, false
	}
	return t, true
}

// call is one fetch in flight. Whoever arrives while it runs waits on it and
// reads the same answer.
type call struct {
	wg  sync.WaitGroup
	val any
	err error
}

// Do returns the cached value or produces it with fn — the whole point of the
// package in one call:
//
//	posts, err := cache.Do(c.Context(), cache, cache.Key{
//		Name: "posts:home",
//		TTL:  5 * time.Minute,
//		Tags: []string{"posts"},
//	}, func(ctx context.Context) ([]Post, error) {
//		return db.Posts(ctx)
//	})
//
// Only one fn runs per name at a time: right after an Invalidate the second
// request waits for the first instead of piling onto the database. An error is
// returned to everyone waiting and cached for nobody — the next request tries
// again.
func Do[T any](ctx context.Context, c *Cache, k Key, fn func(context.Context) (T, error)) (T, error) {
	if v, ok := Get[T](c, k.Name); ok {
		return v, nil
	}
	var zero T

	c.mu.Lock()
	if inflight, ok := c.flight[k.Name]; ok {
		c.mu.Unlock()
		inflight.wg.Wait()
		if inflight.err != nil {
			return zero, inflight.err
		}
		v, ok := inflight.val.(T)
		if !ok {
			return zero, nil
		}
		return v, nil
	}
	cl := &call{}
	cl.wg.Add(1)
	c.flight[k.Name] = cl
	c.mu.Unlock()

	// fn goes to the database. Holding the cache lock here would stop every
	// other key in the app, and a nested Do would deadlock on itself.
	v, err := fn(ctx)
	cl.val, cl.err = v, err

	c.mu.Lock()
	delete(c.flight, k.Name)
	c.mu.Unlock()
	cl.wg.Done()

	if err != nil {
		return zero, err
	}
	c.Set(k, v)
	return v, nil
}

// onceKey is where the per-request answers live inside the Ctx.
const onceKey = "trilha.cache.once"

// onceMu guards only the creation of that map. The Ctx belongs to one request
// but not to one goroutine, and two Once calls racing to be the first would
// otherwise write the Ctx map at the same time.
var onceMu sync.Mutex

// Once answers a question once per request. It is not the cache: nothing here
// survives the response, and that is the point — the current user's data must
// not be served to the next one.
//
//	user, err := cache.Once(c, "user", func() (*User, error) {
//		return db.User(c.Context(), id)
//	})
//
// Use it for what the layout, the page and three components all need to know
// and nobody wants to thread through as an argument. The error is remembered
// too: a failed question is answered once, not on every component that asks.
func Once[T any](c *trilha.Ctx, name string, fn func() (T, error)) (T, error) {
	onceMu.Lock()
	m, _ := c.Get(onceKey).(*sync.Map)
	if m == nil {
		m = &sync.Map{}
		c.Set(onceKey, m)
	}
	onceMu.Unlock()

	if v, ok := m.Load(name); ok {
		cl := v.(*call)
		cl.wg.Wait()
		return typed[T](cl)
	}
	cl := &call{}
	cl.wg.Add(1)
	if v, loaded := m.LoadOrStore(name, cl); loaded {
		other := v.(*call)
		other.wg.Wait()
		return typed[T](other)
	}
	cl.val, cl.err = fn()
	cl.wg.Done()
	return typed[T](cl)
}

func typed[T any](cl *call) (T, error) {
	var zero T
	if cl.err != nil {
		return zero, cl.err
	}
	v, ok := cl.val.(T)
	if !ok {
		return zero, nil
	}
	return v, nil
}
