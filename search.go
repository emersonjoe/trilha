package trilha

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Search is one box over several kinds of thing.
//
// It is the search every internal application has in the top bar: a field, a
// handful of types, results grouped by type, and a click that goes to the
// record. What each application writes instead is one LIKE per type, copied,
// and then finds out that "joao" does not match "João".
//
//	var Busca = trilha.NewSearch(trilha.SearchOpts{Tenant: auth.Tenant}).
//		Kind("pessoa", trilha.KindOpts{Label: "People", Module: "hr"}).
//		Kind("processo", trilha.KindOpts{Label: "Cases"})
//
//	Busca.Put(c, trilha.Doc{Kind: "pessoa", ID: p.ID, Title: p.Name, URL: "/people/" + p.ID})
//	res, err := Busca.Query(c, c.Query("q"), trilha.SearchQuery{Limit: 10})
//
// The screens are ui.SearchBox and ui.SearchResults.
type Search struct {
	store  SearchStore
	order  []string
	kinds  map[string]KindOpts
	tenant func(*Ctx) string
	allow  func(*Ctx, string) bool
}

// Doc is a record as the index sees it: enough to find it, name it and go to
// it, and nothing else.
//
// It is a projection and not the record because a search index that holds the
// whole row is a second copy of the database with its own staleness.
type Doc struct {
	Kind  string
	ID    string
	Title string
	// Body is everything else worth matching — the e-mail, the document number,
	// the description — as one string. It is also where the snippet is cut
	// from, so what goes in is what a person will read back.
	Body string
	// URL is where the record lives. A hit that links nowhere is a hit that
	// makes somebody search a second time, in another box.
	URL string
	// Tenant is the organisation the record belongs to. Empty inherits the one
	// SearchOpts.Tenant answers, which is the whole point: the filter nobody
	// has to remember is the filter nobody forgets.
	Tenant string
}

// KindOpts describes one kind of thing in the index.
type KindOpts struct {
	// Label is what the group is called on the screen. Empty uses the kind's
	// own name, which is fine for a prototype and wrong for a product.
	Label string
	// Module is the area of the application this kind belongs to. With
	// SearchOpts.Allow saying no to it, the kind disappears from the results
	// entirely — count included.
	Module string
}

// SearchOpts configures the index.
type SearchOpts struct {
	// Store is where the documents live. Nil means memory.
	Store SearchStore
	// Tenant is the organisation of the current request. Nil means the
	// application is not multi-tenant, and every document is everybody's.
	//
	// It is a function and not a value because the runtime must not import
	// auth: pass auth.Tenant and the two agree by construction.
	Tenant func(c *Ctx) string
	// Allow answers whether the current user may see a module. Nil allows
	// everything, which is what an application with no policy means.
	Allow func(c *Ctx, module string) bool
}

// SearchQuery narrows one search.
type SearchQuery struct {
	// Kinds restricts the search to these kinds. Empty searches all of them.
	Kinds []string
	// Limit is how many hits per kind. Zero means ten: this is a box, not a
	// listing, and a hundred rows of one type push the other types off the
	// screen.
	Limit int
	// Rerank is the hook for whatever else ranks — a vector index, a model, a
	// hand-written rule. It runs per kind, on the hits that were found.
	Rerank func(query string, hits []Hit) []Hit
}

// Hit is one document that matched.
type Hit struct {
	Doc
	// Score is comparable inside one search and meaningless outside it.
	Score float64
	// Snippet is the piece of the document around the match.
	Snippet Snippet
}

// Snippet is a piece of text cut into parts, with the ones that matched marked.
//
// It is not a string with <mark> in it, and that is deliberate: HTML built by
// the runtime out of a field somebody typed is an injection waiting for the
// first Body with a <script> in it. The screen writes the tag; ui.SearchResults
// already does.
type Snippet []SnippetPart

// SnippetPart is one run of the snippet.
type SnippetPart struct {
	Text  string
	Match bool
}

// String is the snippet as plain text, for a log or a test.
func (s Snippet) String() string {
	var b strings.Builder
	for _, p := range s {
		b.WriteString(p.Text)
	}
	return b.String()
}

// SearchGroup is the hits of one kind, with the label the screen shows.
type SearchGroup struct {
	Kind  string
	Label string
	Hits  []Hit
	// Total is how many matched, which is not len(Hits) when Limit cut.
	Total int
}

// SearchResult is what one search answers.
//
// Groups is a slice and not a map on purpose: a map has no order, and a screen
// whose sections move between one search and the next is a screen people stop
// reading. The order is the order the kinds were declared in.
type SearchResult struct {
	Query  string
	Terms  []string
	Groups []SearchGroup
	Total  int
}

// Group is the hits of one kind, or false when it had none.
func (r SearchResult) Group(kind string) (SearchGroup, bool) {
	for _, g := range r.Groups {
		if g.Kind == kind {
			return g, true
		}
	}
	return SearchGroup{}, false
}

// SearchStore is where the index lives. Memory is the default; a table behind
// the same four methods — FTS5 in SQLite, tsvector in Postgres — is the next
// step, and no screen changes.
//
// The store matches and scores. Everything above it — the grouping, the tenant,
// the module, the snippet — is the same for every store, so two stores cannot
// disagree about what a result looks like.
type SearchStore interface {
	Put(ctx context.Context, docs []Doc) error
	Delete(ctx context.Context, kind, id string) error
	DeleteKind(ctx context.Context, kind string) error
	Search(ctx context.Context, q SearchStoreQuery) ([]Hit, error)
}

// SearchStoreQuery is what a store is asked. The terms are already folded by
// SearchTerms, so a store written for SQL indexes and queries the same words.
type SearchStoreQuery struct {
	Terms  []string
	Kind   string
	Tenant string
	Limit  int
}

// ErrUnknownKind is a document whose kind nobody declared.
var ErrUnknownKind = errors.New("trilha: this kind is not in the index")

// NewSearch builds the index.
func NewSearch(o SearchOpts) *Search {
	s := &Search{store: o.Store, kinds: map[string]KindOpts{}, tenant: o.Tenant, allow: o.Allow}
	if s.store == nil {
		s.store = SearchMemory()
	}
	return s
}

// Kind declares one kind of thing, and answers the index so the declarations
// chain. The order they are declared in is the order the groups come back in.
func (s *Search) Kind(name string, o KindOpts) *Search {
	if name == "" {
		panic("trilha: a search kind needs a name")
	}
	if _, ok := s.kinds[name]; !ok {
		s.order = append(s.order, name)
	}
	if o.Label == "" {
		o.Label = name
	}
	s.kinds[name] = o
	return s
}

// Put writes documents into the index, replacing whatever was there under the
// same kind and id.
//
// A kind nobody declared is refused instead of stored: a typo that indexes in
// silence is a record that never turns up in the search, and nothing points at
// why.
func (s *Search) Put(c *Ctx, docs ...Doc) error {
	if len(docs) == 0 {
		return nil
	}
	tenant := s.tenantOf(c)
	out := make([]Doc, 0, len(docs))
	for _, d := range docs {
		if _, ok := s.kinds[d.Kind]; !ok {
			return NewHint(ErrSearchKind, fmt.Errorf("%w: %q", ErrUnknownKind, d.Kind)).
				Fix(fmt.Sprintf("declare it first: Kind(%q, trilha.KindOpts{Label: …}) — the declared ones are %s",
					d.Kind, s.declared())).
				Doc("/reference/search")
		}
		if d.Tenant == "" {
			d.Tenant = tenant
		}
		out = append(out, d)
	}
	return s.store.Put(ctxOrBackground(c), out)
}

// Delete removes one document.
func (s *Search) Delete(c *Ctx, kind, id string) error {
	return s.store.Delete(ctxOrBackground(c), kind, id)
}

// Reindex replaces every document of one kind.
//
// It is the call behind "the index is stale": the whole kind goes and comes
// back. Ten thousand records belong in a task — see the task package — because
// a request that rebuilds an index is a request that times out.
func (s *Search) Reindex(c *Ctx, kind string, docs []Doc) error {
	if _, ok := s.kinds[kind]; !ok {
		return NewHint(ErrSearchKind, fmt.Errorf("%w: %q", ErrUnknownKind, kind)).
			Fix(fmt.Sprintf("declare it first: Kind(%q, trilha.KindOpts{Label: …}) — the declared ones are %s",
				kind, s.declared())).
			Doc("/reference/search")
	}
	if err := s.store.DeleteKind(ctxOrBackground(c), kind); err != nil {
		return err
	}
	return s.Put(c, docs...)
}

// Query searches every kind the current user may see, and answers them grouped.
//
// An empty query answers an empty result and not everything: a box somebody
// tabbed past must not become a full table scan.
func (s *Search) Query(c *Ctx, q string, sq SearchQuery) (SearchResult, error) {
	terms := SearchTerms(q)
	res := SearchResult{Query: q, Terms: terms}
	if len(terms) == 0 {
		return res, nil
	}
	limit := sq.Limit
	if limit <= 0 {
		limit = 10
	}
	tenant := s.tenantOf(c)
	for _, kind := range s.order {
		if len(sq.Kinds) > 0 && !contains(sq.Kinds, kind) {
			continue
		}
		opts := s.kinds[kind]
		// A denied module leaves no trace: a count saying "3 people" to
		// somebody who may not see people has already told them something.
		if opts.Module != "" && s.allow != nil && !s.allow(c, opts.Module) {
			continue
		}
		hits, err := s.store.Search(ctxOrBackground(c), SearchStoreQuery{
			Terms: terms, Kind: kind, Tenant: tenant, Limit: limit,
		})
		if err != nil {
			return SearchResult{}, err
		}
		if len(hits) == 0 {
			continue
		}
		total := len(hits)
		if sq.Rerank != nil {
			hits = sq.Rerank(q, hits)
		}
		if len(hits) > limit {
			hits = hits[:limit]
		}
		for i := range hits {
			hits[i].Snippet = snippetOf(hits[i].Doc, terms)
		}
		res.Groups = append(res.Groups, SearchGroup{
			Kind: kind, Label: opts.Label, Hits: hits, Total: total,
		})
		res.Total += total
	}
	return res, nil
}

func (s *Search) tenantOf(c *Ctx) string {
	if s.tenant == nil {
		return ""
	}
	return s.tenant(c)
}

func (s *Search) declared() string {
	if len(s.order) == 0 {
		return "none — the index has no kind yet"
	}
	return strings.Join(s.order, ", ")
}

// SearchTerms cuts a query into the words an index matches on: lowercase, with
// the accents taken off, split on everything that is not a letter or a digit.
//
//	trilha.SearchTerms("João da SILVA") // [joao da silva]
//
// It is exported because a store written for SQL has to fold the same way. Two
// tokenizers that disagree are an index that cannot find what it wrote.
func SearchTerms(s string) []string {
	var out []string
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			out = append(out, word.String())
			word.Reset()
		}
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			word.WriteRune(fold(r))
		default:
			flush()
		}
	}
	flush()
	return out
}

// foldGroups is the accent table: the first rune of each group is what the
// rest fold to. It is written this way so it can be read — two parallel strings
// of sixty characters are a table nobody can check.
var foldGroups = []string{
	"aàáâãäåāăą",
	"eèéêëēĕėęě",
	"iìíîïĩīĭįı",
	"oòóôõöøōŏő",
	"uùúûüũūŭůűų",
	"cçćĉċč",
	"nñńņň",
	"yýÿŷ",
	"dđð",
	"sšśş",
	"zžźż",
	"lł", "gğģ", "rřŗ", "tţť",
}

// æ and ß are not in the table, and that is the rule the table
// keeps: one rune folds to one rune. A two-letter expansion would move every
// index after it, and a snippet marks its match by index — the accented word
// has to be findable *and* show up accented.

// foldTable is the same thing as a lookup, built once.
var foldTable = func() map[rune]rune {
	m := map[rune]rune{}
	for _, g := range foldGroups {
		r := []rune(g)
		for _, from := range r[1:] {
			m[from] = r[0]
		}
	}
	return m
}()

// fold lowercases a rune and takes its accent off. A letter the table does not
// know is lowercased and left alone, because a letter nobody transliterated is
// still a letter — and dropping it would make the word unfindable instead of
// merely accented.
func fold(r rune) rune {
	r = unicode.ToLower(r)
	if to, ok := foldTable[r]; ok {
		return to
	}
	return r
}

// snippetLen is how much of a body a snippet shows. It is a screen's worth of
// context and not a paragraph: a snippet long enough to need scrolling is the
// record itself, and the link is right there.
const snippetLen = 160

// snippetOf cuts the window around the first match and marks the terms in it.
func snippetOf(d Doc, terms []string) Snippet {
	body := d.Body
	if body == "" {
		body = d.Title
	}
	runes := []rune(body)
	folded := []rune(foldString(body))
	start := 0
	if i := firstMatch(folded, terms); i > 40 {
		start = i - 40
	}
	end := start + snippetLen
	if end > len(runes) {
		end = len(runes)
	}
	window := runes[start:end]
	windowFolded := folded[start:end]

	var out Snippet
	if start > 0 {
		out = append(out, SnippetPart{Text: "…"})
	}
	i := 0
	for i < len(window) {
		n := matchAt(windowFolded, i, terms)
		if n == 0 {
			j := i
			for j < len(window) && matchAt(windowFolded, j, terms) == 0 {
				j++
			}
			out = append(out, SnippetPart{Text: string(window[i:j])})
			i = j
			continue
		}
		out = append(out, SnippetPart{Text: string(window[i : i+n]), Match: true})
		i += n
	}
	if end < len(runes) {
		out = append(out, SnippetPart{Text: "…"})
	}
	return out
}

// firstMatch is where the first term starts, or -1.
func firstMatch(folded []rune, terms []string) int {
	for i := range folded {
		if matchAt(folded, i, terms) > 0 {
			return i
		}
	}
	return -1
}

// matchAt answers how many runes a term covers at i, and zero when none does.
// A match has to start a word, which is what a prefix search means: "ana" finds
// "Ana Lima" and not "Joana".
func matchAt(folded []rune, i int, terms []string) int {
	if i > 0 && isWordRune(folded[i-1]) {
		return 0
	}
	best := 0
	for _, t := range terms {
		r := []rune(t)
		if len(r) > len(folded)-i {
			continue
		}
		if string(folded[i:i+len(r)]) == t && len(r) > best {
			best = len(r)
		}
	}
	if best == 0 {
		return 0
	}
	// The whole word is marked, not the prefix: half a word in yellow reads
	// like a rendering bug.
	for best < len(folded)-i && isWordRune(folded[i+best]) {
		best++
	}
	return best
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// foldString folds a whole string. It keeps one rune per rune so an index into
// the folded text is an index into the original — which is what lets a snippet
// mark the accented word and show it accented.
func foldString(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(fold(r))
	}
	return b.String()
}

// SearchMemory keeps the index in this process.
//
// It is the default so an application has search before it has a table, and it
// is honest about what it is: every document is scanned, which is right for the
// thousands an internal application has and wrong for the millions it does not.
func SearchMemory() SearchStore { return &searchMemory{docs: map[string][]Doc{}} }

type searchMemory struct {
	mu   sync.RWMutex
	docs map[string][]Doc
}

func (m *searchMemory) Put(_ context.Context, docs []Doc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range docs {
		list := m.docs[d.Kind]
		replaced := false
		for i, old := range list {
			if old.ID == d.ID {
				list[i], replaced = d, true
				break
			}
		}
		if !replaced {
			list = append(list, d)
		}
		m.docs[d.Kind] = list
	}
	return nil
}

func (m *searchMemory) Delete(_ context.Context, kind, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.docs[kind]
	for i, d := range list {
		if d.ID == id {
			m.docs[kind] = append(list[:i:i], list[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *searchMemory) DeleteKind(_ context.Context, kind string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, kind)
	return nil
}

func (m *searchMemory) Search(_ context.Context, q SearchStoreQuery) ([]Hit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var hits []Hit
	for _, d := range m.docs[q.Kind] {
		if q.Tenant != "" && d.Tenant != q.Tenant {
			continue
		}
		score, ok := scoreOf(d, q.Terms)
		if !ok {
			continue
		}
		hits = append(hits, Hit{Doc: d, Score: score})
	}
	// Ties break on the title so two runs of the same search agree.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Title < hits[j].Title
	})
	return hits, nil
}

// scoreOf is the ranking: every term has to appear somewhere, and the title is
// worth more than the body. Two words in a box mean both — a search that
// answers more the more you type is a search people stop typing into.
func scoreOf(d Doc, terms []string) (float64, bool) {
	title := []rune(foldString(d.Title))
	body := []rune(foldString(d.Body))
	var score float64
	for _, t := range terms {
		switch {
		case firstMatch(title, []string{t}) >= 0:
			score += 3
		case firstMatch(body, []string{t}) >= 0:
			score++
		default:
			return 0, false
		}
	}
	return score, true
}
