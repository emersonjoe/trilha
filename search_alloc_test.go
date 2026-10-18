package trilha

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func searchCorpus(n int) SearchStore {
	m := SearchMemory()
	body := strings.Repeat("Relatório de vendas do trimestre com números da região sul ", 20)
	docs := make([]Doc, n)
	for i := range docs {
		docs[i] = Doc{Kind: "pedido", ID: fmt.Sprint(i), Title: fmt.Sprintf("Pedido %d de Ana Lima", i), Body: body}
	}
	m.Put(context.Background(), docs)
	return m
}

// Spec 177: the memory index folds a document once, when it is put, and not
// on every search: folding the title and the body of every document, rune by
// rune through the accent table, was what a search spent its time on. A
// search that matches nothing allocates nothing per document.
func TestSearchMemoryAllocationsPerDoc(t *testing.T) {
	const n = 200
	m := searchCorpus(n)
	q := SearchStoreQuery{Kind: "pedido", Terms: []string{"regiao", "inexistente"}}
	allocs := testing.AllocsPerRun(5, func() {
		if _, err := m.Search(context.Background(), q); err != nil {
			t.Fatal(err)
		}
	})
	if perDoc := allocs / n; perDoc >= 1 {
		t.Fatalf("%.0f allocations per search, %.1f per document", allocs, perDoc)
	}
}

func BenchmarkSearchMemory(b *testing.B) {
	m := searchCorpus(1000)
	q := SearchStoreQuery{Kind: "pedido", Terms: []string{"regiao", "sul"}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.Search(context.Background(), q)
	}
}

// hasWordPrefix answers what firstMatch answers for one term, on the same
// folded text: a term that starts a word, never one inside a word.
func TestHasWordPrefixAgreesWithFirstMatch(t *testing.T) {
	texts := []string{"", "Ana Lima", "Joana", "joão-ana", "São Paulo, SÃO", "x ana", "anana ana",
		"pedido nº 42 (urgente)", "çãé ÇÃÉ", "ana\nlima", "aaa", "ünïcödé ü"}
	terms := []string{"ana", "lima", "sao", "joao", "42", "urg", "a", "cae", "unicode", "u", "ana lima", ""}
	for _, text := range texts {
		folded := foldString(text)
		for _, term := range terms {
			want := firstMatch([]rune(folded), []string{term}) >= 0
			if got := hasWordPrefix(folded, term); got != want {
				t.Errorf("hasWordPrefix(%q, %q) = %v, firstMatch says %v", text, term, got, want)
			}
		}
	}
}

// Putting a document whose id is already there replaces it, also after a
// delete moved the others; the position index must follow both.
func TestSearchMemoryPutReplacesAfterDelete(t *testing.T) {
	ctx := context.Background()
	m := SearchMemory()
	m.Put(ctx, []Doc{{Kind: "k", ID: "a", Title: "alfa"}, {Kind: "k", ID: "b", Title: "beta"}, {Kind: "k", ID: "c", Title: "gama"}})
	m.Delete(ctx, "k", "a")
	m.Put(ctx, []Doc{{Kind: "k", ID: "c", Title: "delta"}, {Kind: "k", ID: "a", Title: "alfa de novo"}})
	for term, want := range map[string]int{"gama": 0, "delta": 1, "beta": 1, "alfa": 1} {
		hits, _ := m.Search(ctx, SearchStoreQuery{Kind: "k", Terms: []string{term}})
		if len(hits) != want {
			t.Errorf("%q: %d hits, want %d", term, len(hits), want)
		}
	}
	m.DeleteKind(ctx, "k")
	m.Put(ctx, []Doc{{Kind: "k", ID: "b", Title: "beta"}})
	if hits, _ := m.Search(ctx, SearchStoreQuery{Kind: "k", Terms: []string{"beta"}}); len(hits) != 1 {
		t.Errorf("after DeleteKind: %d hits for beta", len(hits))
	}
}

func BenchmarkSearchMemoryReindex(b *testing.B) {
	docs := make([]Doc, 20000)
	for i := range docs {
		docs[i] = Doc{Kind: "pedido", ID: fmt.Sprint(i), Title: "Pedido", Body: "corpo"}
	}
	for i := 0; i < b.N; i++ {
		m := SearchMemory()
		m.Put(context.Background(), docs)
	}
}
