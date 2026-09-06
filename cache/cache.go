// Package cache is an in-memory cache with TTL, tags and explicit
// invalidation. It answers three questions in one place: where the value comes
// from, how long it is good for, and what drops it.
//
// The cache is safe for concurrent use. It never starts a goroutine: an
// expired entry leaves when it is read or when the cap needs the room.
package cache

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emersonjoe/trilha"
)

// defaultMaxEntries caps a cache nobody sized. A cache without a cap is a leak
// with a nice name (NIST SP 800-53 SC-5).
const defaultMaxEntries = 10000

// Options configures a Cache. The zero value is valid.
type Options struct {
	// Name labels the metrics of this cache (default "default").
	Name string
	// MaxEntries is the entry cap; past it, the least recently used one is
	// evicted (default 10000).
	MaxEntries int
	// Metrics registers hits, misses, evictions and size. Nil records
	// nothing but Stats.
	Metrics *trilha.Metrics
}

// Key is one write: the name to read it back by, how long it is good for, and
// the tags that can drop it. TTL <= 0 means no expiry — the entry lives until
// a tag, a Delete or the cap takes it.
type Key struct {
	Name string
	TTL  time.Duration
	Tags []string
}

// Stats are the counters of one cache.
type Stats struct {
	Hits, Misses, Evictions int64
	Entries                 int
}

type entry struct {
	name    string
	value   any
	expires time.Time // zero means no expiry
	tags    []string
}

// Cache is an in-memory cache. Create it with New.
type Cache struct {
	max int

	mu    sync.Mutex
	items map[string]*list.Element // name -> element holding *entry
	lru   *list.List               // front = most recently used
	tags  map[string]map[string]struct{}

	flight map[string]*call // in-flight Do calls, one per name

	hits, misses, evictions atomic.Int64

	mHits, mMisses, mEvict *trilha.Counter
	mEntries               *trilha.Gauge
}

// New returns a cache ready for concurrent use.
func New(o Options) *Cache {
	c := &Cache{
		max:    o.MaxEntries,
		items:  map[string]*list.Element{},
		lru:    list.New(),
		tags:   map[string]map[string]struct{}{},
		flight: map[string]*call{},
	}
	if c.max <= 0 {
		c.max = defaultMaxEntries
	}
	if o.Metrics != nil {
		name := o.Name
		if name == "" {
			name = "default"
		}
		c.mHits = o.Metrics.Counter("trilha_cache_hits_total", "Cache reads answered from memory.", "cache").With(name)
		c.mMisses = o.Metrics.Counter("trilha_cache_misses_total", "Cache reads with nothing to answer with.", "cache").With(name)
		c.mEvict = o.Metrics.Counter("trilha_cache_evictions_total", "Entries dropped to stay under the cap.", "cache").With(name)
		c.mEntries = o.Metrics.Gauge("trilha_cache_entries", "Entries held right now.", "cache").With(name)
	}
	return c
}

// Set writes a value. Writing over a name replaces its tags too.
func (c *Cache) Set(k Key, v any) {
	if k.Name == "" {
		return
	}
	e := &entry{name: k.Name, value: v, tags: k.Tags}
	if k.TTL > 0 {
		e.expires = time.Now().Add(k.TTL)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.drop(k.Name)
	el := c.lru.PushFront(e)
	c.items[k.Name] = el
	for _, t := range k.Tags {
		if c.tags[t] == nil {
			c.tags[t] = map[string]struct{}{}
		}
		c.tags[t][k.Name] = struct{}{}
	}
	for c.lru.Len() > c.max {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.drop(oldest.Value.(*entry).name)
		c.evictions.Add(1)
		if c.mEvict != nil {
			c.mEvict.Inc()
		}
	}
	c.size()
}

// Get reads a value. An expired entry is a miss, and leaves on the way out.
func (c *Cache) Get(name string) (any, bool) {
	c.mu.Lock()
	el, ok := c.items[name]
	if !ok {
		c.mu.Unlock()
		c.miss()
		return nil, false
	}
	e := el.Value.(*entry)
	if !e.expires.IsZero() && time.Now().After(e.expires) {
		c.drop(name)
		c.size()
		c.mu.Unlock()
		c.miss()
		return nil, false
	}
	c.lru.MoveToFront(el)
	v := e.value
	c.mu.Unlock()
	c.hits.Add(1)
	if c.mHits != nil {
		c.mHits.Inc()
	}
	return v, true
}

// Delete removes entries by name and reports how many were there.
func (c *Cache) Delete(names ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, name := range names {
		if _, ok := c.items[name]; ok {
			c.drop(name)
			n++
		}
	}
	c.size()
	return n
}

// Invalidate removes every entry carrying any of the tags and reports how many
// fell. This is the explicit way out: write to the database, drop the tag.
func (c *Cache) Invalidate(tags ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range tags {
		for name := range c.tags[t] {
			if _, ok := c.items[name]; ok {
				c.drop(name)
				n++
			}
		}
		delete(c.tags, t)
	}
	c.size()
	return n
}

// Clear empties the cache, tags included.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = map[string]*list.Element{}
	c.lru = list.New()
	c.tags = map[string]map[string]struct{}{}
	c.size()
}

// Len is how many entries are held (expired ones that nobody read still count).
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

// Stats reports the counters. They only ever grow, except Entries.
func (c *Cache) Stats() Stats {
	return Stats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Entries:   c.Len(),
	}
}

// drop removes one entry from the three structures at once. Every removal goes
// through here: an entry that leaves the map while a tag still points at it is
// the bug this package is most likely to have.
func (c *Cache) drop(name string) {
	el, ok := c.items[name]
	if !ok {
		return
	}
	for _, t := range el.Value.(*entry).tags {
		if set := c.tags[t]; set != nil {
			delete(set, name)
			if len(set) == 0 {
				delete(c.tags, t)
			}
		}
	}
	c.lru.Remove(el)
	delete(c.items, name)
}

// size publishes the current size. Call it holding the lock.
func (c *Cache) size() {
	if c.mEntries != nil {
		c.mEntries.Set(float64(c.lru.Len()))
	}
}

func (c *Cache) miss() {
	c.misses.Add(1)
	if c.mMisses != nil {
		c.mMisses.Inc()
	}
}

// tagCount is how many tags point at something (tests).
func (c *Cache) tagCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tags)
}
