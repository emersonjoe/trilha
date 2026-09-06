package cache

import (
	"strconv"
	"testing"
	"time"
)

func TestTTLExpires(t *testing.T) {
	c := New(Options{})
	c.Set(Key{Name: "a", TTL: 30 * time.Millisecond}, 1)
	if v, ok := c.Get("a"); !ok || v.(int) != 1 {
		t.Fatalf("Get = %v, %v", v, ok)
	}
	// TTL <= 0 means no expiry: tags are the way out, not the clock.
	c.Set(Key{Name: "forever"}, 2)
	time.Sleep(50 * time.Millisecond)
	if _, ok := c.Get("a"); ok {
		t.Fatal("expired entry must be a miss")
	}
	if _, ok := c.Get("forever"); !ok {
		t.Fatal("TTL <= 0 must not expire")
	}
	if s := c.Stats(); s.Hits != 2 || s.Misses != 1 {
		t.Fatalf("stats = %+v", s)
	}
}

func TestInvalidateByTag(t *testing.T) {
	c := New(Options{})
	c.Set(Key{Name: "lista", Tags: []string{"posts"}}, "l")
	c.Set(Key{Name: "post:1", Tags: []string{"posts", "post:1"}}, "p")
	c.Set(Key{Name: "menu", Tags: []string{"nav"}}, "m")
	if n := c.Invalidate("desconhecida"); n != 0 {
		t.Fatalf("unknown tag dropped %d entries", n)
	}
	if n := c.Invalidate("posts"); n != 2 {
		t.Fatalf("Invalidate = %d, want 2", n)
	}
	if _, ok := c.Get("lista"); ok {
		t.Fatal("tagged entry survived")
	}
	if _, ok := c.Get("menu"); !ok {
		t.Fatal("untagged entry must stay")
	}
	// The second tag of a dropped entry must not keep pointing at it.
	if n := c.Invalidate("post:1"); n != 0 {
		t.Fatalf("orphan tag dropped %d entries", n)
	}
}

func TestRewriteReplacesTags(t *testing.T) {
	c := New(Options{})
	c.Set(Key{Name: "x", Tags: []string{"velha"}}, 1)
	c.Set(Key{Name: "x", Tags: []string{"nova"}}, 2)
	if n := c.Invalidate("velha"); n != 0 {
		t.Fatalf("old tag still drops the entry (%d)", n)
	}
	if v, _ := c.Get("x"); v.(int) != 2 {
		t.Fatalf("Get = %v", v)
	}
	if n := c.Invalidate("nova"); n != 1 {
		t.Fatalf("new tag = %d", n)
	}
}

func TestMaxEntriesEvictsLeastRecentlyUsed(t *testing.T) {
	c := New(Options{MaxEntries: 3})
	for i := 0; i < 3; i++ {
		c.Set(Key{Name: strconv.Itoa(i)}, i)
	}
	c.Get("0") // 1 is now the oldest use
	c.Set(Key{Name: "3"}, 3)
	if c.Len() != 3 {
		t.Fatalf("Len = %d, want 3", c.Len())
	}
	if _, ok := c.Get("1"); ok {
		t.Fatal("least recently used must be evicted")
	}
	if _, ok := c.Get("0"); !ok {
		t.Fatal("recently used must survive")
	}
	if s := c.Stats(); s.Evictions != 1 {
		t.Fatalf("stats = %+v", s)
	}
}

func TestDeleteAndClear(t *testing.T) {
	c := New(Options{})
	c.Set(Key{Name: "a", Tags: []string{"t"}}, 1)
	c.Set(Key{Name: "b", Tags: []string{"t"}}, 2)
	if n := c.Delete("a", "nao-existe"); n != 1 {
		t.Fatalf("Delete = %d", n)
	}
	if n := c.Invalidate("t"); n != 1 {
		t.Fatalf("deleted key still in the tag (%d)", n)
	}
	c.Set(Key{Name: "c", Tags: []string{"u"}}, 3)
	c.Clear()
	if c.Len() != 0 || c.tagCount() != 0 {
		t.Fatalf("Clear left %d entries and %d tags", c.Len(), c.tagCount())
	}
}
