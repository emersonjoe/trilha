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

// Store holds the posts of one app. It is a value, not package state: Setup
// creates it, trilha.Provide puts it in the app, and a test suite that boots a
// server per test gives each one its own — which package state cannot do.
type Store struct {
	mu    sync.RWMutex
	store map[string]Post

	// Published counts posts created since the process started. Setup points
	// it at the app registry; nil (this package under its own test) counts
	// nothing.
	Published *trilha.Counter
	// Cache holds the lists this store computes. Setup points it at a cache
	// of the app's; nil means every call reads the map, which is the
	// behaviour the cache is supposed to be invisible against.
	Cache *cache.Cache
}

// New returns an empty store, ready for Seed.
func New() *Store { return &Store{store: map[string]Post{}} }

// listKey is the whole cache policy of this example in one place: one name,
// five minutes, one tag. The tag is what Create and Delete pull.
var listKey = cache.Key{Name: "posts:all", TTL: 5 * time.Minute, Tags: []string{"posts"}}

// Cached returns the same thing as All, once per window. Here the store is a
// map and the saving is nothing; in an app this is the query every visit to
// the list makes.
func (s *Store) Cached(ctx context.Context) ([]Post, error) {
	if s.Cache == nil {
		return s.All(), nil
	}
	return cache.Do(ctx, s.Cache, listKey, func(context.Context) ([]Post, error) {
		return s.All(), nil
	})
}

// invalidate drops what stopped being true. It lives next to the writes, not
// next to the reads: a cache is invalidated by whoever changed the data.
func (s *Store) invalidate() {
	if s.Cache != nil {
		s.Cache.Invalidate("posts")
	}
}

// Seed loads the initial posts.
func (s *Store) Seed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = map[string]Post{}
	for i, p := range []Post{
		{Slug: "ola-trilha", Title: "Olá, Trilha", Body: "Roteamento por arquivos em Go."},
		{Slug: "layouts", Title: "Layouts aninhados", Body: "Cada pasta pode ter o seu layout.go."},
	} {
		p.Created = time.Date(2026, 9, 1+i, 0, 0, 0, 0, time.UTC)
		s.store[p.Slug] = p
	}
	s.invalidate()
}

// All returns posts, newest first.
func (s *Store) All() []Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Post, 0, len(s.store))
	for _, p := range s.store {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out
}

// Get returns one post.
func (s *Store) Get(slug string) (Post, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.store[slug]
	return p, ok
}

// Create stores a post; the slug is derived from the title.
func (s *Store) Create(title, body string) Post {
	p := Post{Slug: Slugify(title), Title: title, Body: body, Created: time.Now().UTC()}
	s.mu.Lock()
	s.store[p.Slug] = p
	s.mu.Unlock()
	s.invalidate()
	if s.Published != nil {
		s.Published.Inc()
	}
	return p
}

// Count returns how many posts are loaded (used by the readiness check).
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.store)
}

// Delete removes a post; reports whether it existed.
func (s *Store) Delete(slug string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.store[slug]
	delete(s.store, slug)
	s.invalidate()
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
