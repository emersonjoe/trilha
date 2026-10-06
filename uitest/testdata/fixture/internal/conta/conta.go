// Package conta counts, per address, the GETs a route served and how many of
// them were prefetches — what a prefetch scenario asks the server.
package conta

import (
	"fmt"
	"sync"

	"github.com/emersonjoe/trilha"
)

var (
	mu  sync.Mutex
	got = map[string][2]int{}
)

// Hit records the request of c under its path and query.
func Hit(c *trilha.Ctx) {
	mu.Lock()
	defer mu.Unlock()
	n := got[c.Request().URL.RequestURI()]
	if c.IsPrefetch() {
		n[1]++
	} else {
		n[0]++
	}
	got[c.Request().URL.RequestURI()] = n
}

// Of is "gets N prefetches M" for an address.
func Of(uri string) string {
	mu.Lock()
	defer mu.Unlock()
	n := got[uri]
	return fmt.Sprintf("gets %d prefetches %d", n[0], n[1])
}
