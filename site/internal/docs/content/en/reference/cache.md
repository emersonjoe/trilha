---
title: cache
description: Options, Key, Cache, Do, Get and Once — the API of the cache package, with the defaults and what each field changes.
---

`import "github.com/emersonjoe/trilha/cache"` — in-memory cache with expiry, tags and
bulk invalidation, plus a per-request memo. The package imports the runtime; the runtime
does not import it, so an app that never mentions it carries nothing of it.

## Creating

```go
func New(o Options) *Cache
```

| `Options` field | Default | What it does |
|---|---|---|
| `Name string` | `"cache"` | label on the metric series; give each cache its own |
| `MaxEntries int` | `10000` | ceiling; the least recently used entry is evicted |
| `Metrics *trilha.Metrics` | `nil` | registry to publish into, usually `a.Metrics()` |

There is no `Close`: nothing runs in the background. An expired entry is removed when it
is read or when the ceiling pushes it out, so a cache nobody touches costs nothing but
the memory it already holds.

## Keys

```go
type Key struct {
	Name string
	TTL  time.Duration
	Tags []string
}
```

`Name` is the address: equal names are the same entry, and everything that changes the
answer belongs in it. `TTL` of zero or less means no expiry. `Tags` group entries for
`Invalidate`; rewriting an entry replaces its tags rather than adding to them.

## Reading and writing

```go
func (c *Cache) Set(k Key, v any)
func (c *Cache) Get(name string) (any, bool)
func (c *Cache) Delete(names ...string) int
func (c *Cache) Invalidate(tags ...string) int
func (c *Cache) Clear()
func (c *Cache) Len() int
func (c *Cache) Stats() Stats
```

`Delete` and `Invalidate` return how many entries they removed. `Stats` carries `Hits`,
`Misses`, `Evictions` and `Entries` — the same four numbers the metrics publish, for a
health page or a test.

Every method is safe from any goroutine.

## Typed access

Go does not allow type parameters on methods, so the typed half of the package is
package-level functions:

```go
func Get[T any](c *Cache, name string) (T, bool)
func Do[T any](ctx context.Context, c *Cache, k Key, fn func(context.Context) (T, error)) (T, error)
```

`Get[T]` returns the value only when the stored type matches; a value written under
another type is a miss, not a panic — a deploy that changes a struct must not crash the
app.

`Do` returns the cached value or produces it with `fn`, storing the result under `k`. An
error is returned to the caller and cached for nobody. Only one `fn` runs per name at a
time: whoever arrives while a fetch is in flight waits for it and reads the same answer,
so the first request after an `Invalidate` does not become a stampede. The cache lock is
not held while `fn` runs, so a `Do` inside a `Do` is fine.

## Per request

```go
func Once[T any](c *trilha.Ctx, name string, fn func() (T, error)) (T, error)
```

Answers a question once per request and forgets it with the response. It is not a cache
and takes no `*Cache`: use it for what a layout, a page and three components all need to
know — the logged-in user, above all — instead of threading the value through every
signature. The error is remembered too, so a failed lookup is attempted once.

Storing a value that belongs to one user in the `*Cache` under a fixed name serves that
value to the next visitor; `Once` is the one that cannot.

## Metrics

With `Options.Metrics` set, four series appear in the exposition, labelled `cache` with
the value of `Options.Name`:

| Series | Type | Meaning |
|---|---|---|
| `trilha_cache_hits_total` | counter | reads answered from memory |
| `trilha_cache_misses_total` | counter | reads that found nothing or found it expired |
| `trilha_cache_evictions_total` | counter | entries dropped by the ceiling |
| `trilha_cache_entries` | gauge | entries held right now |

Evictions climbing steadily means `MaxEntries` is too low for the key space in use.
