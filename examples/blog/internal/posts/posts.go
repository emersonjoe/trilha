// Package posts is an in-memory post store for the example app.
package posts

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
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
