// Package itens is the in-memory store behind the fixture's item pages, and
// the counters a scenario reads to know how many requests the server saw.
package itens

import (
	"strings"
	"sync"
)

var (
	mu    sync.Mutex
	nomes = []string{"alpha", "beta"}
	views int
)

// Add stores one item.
func Add(nome string) {
	mu.Lock()
	defer mu.Unlock()
	nomes = append(nomes, nome)
}

// Find is the items whose name contains q.
func Find(q string) []string {
	mu.Lock()
	defer mu.Unlock()
	var out []string
	for _, n := range nomes {
		if strings.Contains(n, q) {
			out = append(out, n)
		}
	}
	return out
}

// View counts one full render of the list and returns the count so far.
func View() int {
	mu.Lock()
	defer mu.Unlock()
	views++
	return views
}
