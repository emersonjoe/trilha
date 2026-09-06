// Package posts is an in-memory post store for the example app.
package posts

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/cache"
)

// Post is a blog post.
type Post struct {
	Slug    string    `json:"slug"`
	Title   string    `json:"title"`
	Body    string    `json:"body"`
	Created time.Time `json:"created"`
}

var (
	mu    sync.RWMutex
	store = map[string]Post{}
)

// Published counts posts created since the process started. Setup points it
// at the app registry; nil (this package under its own test) counts nothing.
var Published *trilha.Counter

// Cache holds the lists this package computes. Setup points it at a cache of
// the app's; nil (this package under its own test) means every call reads the
// store, which is the behaviour the cache is supposed to be invisible against.
var Cache *cache.Cache

// listKey is the whole cache policy of this example in one place: one name,
// five minutes, one tag. The tag is what Create and Delete pull.
var listKey = cache.Key{Name: "posts:all", TTL: 5 * time.Minute, Tags: []string{"posts"}}

// Cached returns the same thing as All, once per window. Here the store is a
// map and the saving is nothing; in an app this is the query every visit to
// the list makes.
func Cached(ctx context.Context) ([]Post, error) {
	if Cache == nil {
		return All(), nil
	}
	return cache.Do(ctx, Cache, listKey, func(context.Context) ([]Post, error) {
		return All(), nil
	})
}

// invalidate drops what stopped being true. It lives next to the writes, not
// next to the reads: a cache is invalidated by whoever changed the data.
func invalidate() {
	if Cache != nil {
		Cache.Invalidate("posts")
	}
}

// Seed loads the initial posts.
func Seed() {
	mu.Lock()
	defer mu.Unlock()
	store = map[string]Post{}
	for i, p := range []Post{
		{Slug: "ola-trilha", Title: "Olá, Trilha", Body: "Roteamento por arquivos em Go."},
		{Slug: "layouts", Title: "Layouts aninhados", Body: "Cada pasta pode ter o seu layout.go."},
	} {
		p.Created = time.Date(2026, 9, 1+i, 0, 0, 0, 0, time.UTC)
		store[p.Slug] = p
	}
	invalidate()
}

// All returns posts, newest first.
func All() []Post {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Post, 0, len(store))
	for _, p := range store {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

// Get returns one post.
func Get(slug string) (Post, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := store[slug]
	return p, ok
}

// Create stores a post; the slug is derived from the title.
func Create(title, body string) Post {
	p := Post{Slug: Slugify(title), Title: title, Body: body, Created: time.Now().UTC()}
	mu.Lock()
	store[p.Slug] = p
	mu.Unlock()
	invalidate()
	if Published != nil {
		Published.Inc()
	}
	return p
}

// Count returns how many posts are loaded (used by the readiness check).
func Count() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(store)
}

// Delete removes a post; reports whether it existed.
func Delete(slug string) bool {
	mu.Lock()
	defer mu.Unlock()
	_, ok := store[slug]
	delete(store, slug)
	invalidate()
	return ok
}

// Slugify turns a title into a URL slug.
func Slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
