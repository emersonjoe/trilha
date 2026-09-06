---
title: Data and cache
description: Where the data comes from, how long an answer is worth keeping, and what knocks it down — with cache.Do, tags and cache.Once.
---

Trilha has no ORM, no repository and no opinion about your database: a page calls your
code, your code calls whatever you use. What the framework does bring is the part that is
always written by hand and always written wrong — keeping an answer for a while, and
throwing it away when it stops being true.

```go
import "github.com/emersonjoe/trilha/cache"
```

## The cache is yours, not the framework's

There is no `app.Cache()`. You create it, you say how big it gets, and you keep it where
the code that fills it lives — usually the package that queries the database:

```go
// internal/events/events.go
var Cache *cache.Cache

// app/setup.go
func Setup(a *trilha.App) error {
	events.Cache = cache.New(cache.Options{
		Name:       "events",
		MaxEntries: 500,
		Metrics:    a.Metrics(),
	})
	return nil
}
```

`MaxEntries` has a default (10 000) and no way to say "no limit". A cache without a
ceiling is a memory leak that takes a week to show up: every key someone can invent — a
search term, a filter in the query string — becomes an entry that never leaves. When the
ceiling is reached the least recently used entry is evicted.

## `Do`: the value, or the way to get it

`cache.Do` is the whole package in one call. It returns what is stored, or runs your
function and stores what it returns:

```go
func Upcoming(ctx context.Context) ([]Event, error) {
	return cache.Do(ctx, Cache, cache.Key{
		Name: "events:upcoming",
		TTL:  5 * time.Minute,
		Tags: []string{"events"},
	}, func(ctx context.Context) ([]Event, error) {
		return db.Upcoming(ctx)
	})
}
```

| `Key` field | What it is |
|---|---|
| `Name` | the address of the value; equal names are the same entry |
| `TTL` | how long it is worth; `0` (or less) means no expiry |
| `Tags` | labels for invalidating in bulk later |

The name is a decision, not a detail: everything that changes the answer belongs in it.
A list that depends on the page and the logged-in user is `posts:page:2:user:42`, not
`posts` — a cache key that forgets the user is how one person's data is served to
another.

An error is returned to the caller and cached for nobody. The next request tries again.

### One fetch at a time

The moment a hot key expires, every request that wanted it arrives at the same instant
and every one of them goes to the database. `Do` does not let that happen: the first
caller runs the function, the others wait for it and read the same answer. It is one
fetch per key, however many requests are queued behind it.

## What knocks it down

Time is the weak way to invalidate — five minutes of a wrong list is five minutes of
someone reading a post that was already deleted. The strong way is to say so:

```go
func Create(ctx context.Context, e Event) error {
	if err := db.Insert(ctx, e); err != nil {
		return err
	}
	Cache.Invalidate("events")
	return nil
}
```

`Invalidate` drops every entry carrying that tag, whatever its name, and returns how many
it dropped. `Delete(names...)` drops by name, and `Clear()` empties everything.

Put the call next to the write, never next to the read. A cache is invalidated by
whoever changed the data — the code doing the reading has no way of knowing that
something changed.

## `Once` is not the cache

The layout wants to know who is logged in. The header wants the same. Two components
inside the page want it too. `cache.Once` answers the question once per request:

```go
func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	user, err := cache.Once(c, "user", func() (*users.User, error) {
		return users.Find(c.Context(), auth.From(c).Subject)
	})
	…
}
```

Nothing stored here survives the response, and that is the whole point. It takes no
`*Cache`, no TTL and no tag, because there is nothing to expire: the value dies with the
request that created it. Reach for `Once` when the alternative is threading a value
through six function signatures, and for `Do` when the answer is the same for everyone.

Do not swap them. A user's name in `Do` under the name `"user"` is that user's name
served to the next person who opens the page.

## Seeing it work

With `Options.Metrics`, four series appear in `/metrics`, labelled by the cache's name:

```
trilha_cache_hits_total{cache="events"} 1043
trilha_cache_misses_total{cache="events"} 61
trilha_cache_evictions_total{cache="events"} 0
trilha_cache_entries{cache="events"} 61
```

Hits over hits plus misses is the hit ratio — under 50 % the TTL is too short or the key
carries something it should not. Evictions climbing means the ceiling is too low: the
cache is throwing away what it was about to be asked for.

## The cache the browser keeps

The cache above saves the server a trip to the database. This one saves the network a whole
response: the browser already has the page and asks only whether it changed.

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	p, ok := trilha.Use[*posts.Store](c).Get(c.Param("slug"))
	if !ok {
		return nil, trilha.ErrNotFound
	}
	c.CacheControl("private, no-cache")
	if c.ETag(p.Updated.UTC().Format(time.RFC3339Nano)) {
		return nil, nil // the copy in the browser is current: 304, no body
	}
	c.SetTitle(p.Title)
	return view(p), nil
}
```

`ETag` writes the tag and reports whether the request already carried it. When it says yes the
`304` is already written, so return `nil, nil` — a body there would be thrown away. `LastModified`
is the same deal for a date, and `CacheControl` writes the header as you typed it. `no-cache` does
not mean "do not store"; it means "store it, but ask me before reusing it", which is exactly what
makes the `304` happen.

The tag is a version of the data, not a hash of the page — and Trilha will not compute one for
you. Every response carries a fresh CSP nonce, so a hash of the HTML would never match twice.
Anything that moves when the data moves works: `updated_at`, a revision number, the ids of what
was rendered.

> A tag that forgets who is reading is the same bug as a cache key that forgets the user. If the
> page changes with the visitor, put that in the tag or do not send one.

Files under `static/` already do this on their own: the fingerprint in `?v=` is their ETag, so the
second visit costs a `304` and no bytes.

## Challenge

The event detail page calls the database on every visit. Cache it for an hour, with a tag
that lets `Save` drop just that one event and another that drops the whole section.

:::solution
Tags are a list, so an entry can belong to more than one group:

```go
func Find(ctx context.Context, slug string) (Event, error) {
	return cache.Do(ctx, Cache, cache.Key{
		Name: "event:" + slug,
		TTL:  time.Hour,
		Tags: []string{"events", "event:" + slug},
	}, func(ctx context.Context) (Event, error) {
		return db.Find(ctx, slug)
	})
}

func Save(ctx context.Context, e Event) error {
	if err := db.Save(ctx, e); err != nil {
		return err
	}
	// The event changed: its own page, and every list it appears in.
	Cache.Invalidate("event:"+e.Slug, "events")
	return nil
}
```
:::
