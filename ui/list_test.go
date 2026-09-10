package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

type doc struct {
	ID, Name string
	Size     int
}

var docs = []doc{{"1", "nota.pdf", 12}, {"2", "recibo.pdf", 340}}

var docCols = Columns[doc]{
	{Key: "name", Label: "File", Sort: true, Cell: func(d doc) h.Node { return h.Text(d.Name) }},
	{Key: "size", Label: "Size", Sort: true, Num: true, Cell: func(d doc) h.Node { return h.Text("x") }},
	{Key: "kind", Label: "Kind", Cell: func(d doc) h.Node { return Badge(h.Text("pdf")) }},
}

// listing renders one DataTable through a real request, so ListParams comes
// from the query the way an app reads it.
func listing(t *testing.T, query string, rows []doc, tune func(*ListState)) string {
	t.Helper()
	a := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/docs", Page: func(c *trilha.Ctx) (h.Node, error) {
		var q struct {
			trilha.ListParams
			Status string `form:"status"`
		}
		if err := c.Bind(&q); err != nil {
			return nil, err
		}
		st := ListState{Params: q.ListParams, Total: len(rows)}
		if tune != nil {
			tune(&st)
		}
		out = render(t, DataTable(c, docCols, rows, st))
		return h.Div(), nil
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/docs"+query, nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	return out
}

func has(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %q in %s", w, got)
		}
	}
}

func TestDataTableOrdersWithRealLinks(t *testing.T) {
	got := listing(t, "?sort=name&dir=asc&q=nota&status=done", docs, nil)
	has(t, got,
		// the column in use says so, and turning it around is the same link
		`<th scope="col" aria-sort="ascending">`,
		`href="?dir=desc&amp;q=nota&amp;sort=name&amp;status=done"`,
		// another sortable column starts ascending and keeps the filters
		`href="?dir=asc&amp;q=nota&amp;sort=size&amp;status=done"`,
	)
	if n := strings.Count(got, "aria-sort"); n != 1 {
		t.Fatalf("aria-sort appears %d times: %s", n, got)
	}
	if strings.Contains(got, "page=") {
		t.Fatalf("ordering has to go back to the first page: %s", got)
	}
	if strings.Contains(got, `<th scope="col" class="ui-num">Kind`) {
		t.Fatal("a column that is not numeric was aligned as one")
	}
	// a column nobody declared sortable is not a link
	has(t, got, `<th scope="col">Kind</th>`)
}

func TestDataTableIgnoresAColumnNobodyDeclared(t *testing.T) {
	got := listing(t, "?sort=password&dir=desc", docs, nil)
	if strings.Contains(got, "aria-sort") {
		t.Fatalf("ordered by an undeclared column: %s", got)
	}
	has(t, got, `href="?dir=asc&amp;sort=name"`)
}

func TestDataTableSwapsOnlyItsOwnFragment(t *testing.T) {
	plain := listing(t, "?per_page=1", docs, nil)
	if strings.Contains(plain, "trilha-target") {
		t.Fatalf("no id, no fragment: %s", plain)
	}
	got := listing(t, "?per_page=1", docs, func(st *ListState) {
		st.ID = "list"
		st.Search = "Search"
	})
	has(t, got,
		`<div class="ui-list" id="list">`,
		`<form class="ui-list-filters" method="get" data-trilha-target="list">`,
		`<input class="ui-input" type="search" name="q"`,
		`class="ui-sort" href="?dir=asc&amp;per_page=1&amp;sort=name" data-trilha-target="list"`,
		`class="ui-pagination"`,
	)
	// the pagination of the fragment swaps the fragment too
	if n := strings.Count(got, `data-trilha-target="list"`); n < 5 {
		t.Fatalf("only %d links swap the fragment: %s", n, got)
	}
	// what the form does not carry as a field travels hidden, or the search
	// would throw the page size away
	has(t, got, `<input type="hidden" name="per_page" value="1">`)
}

func TestDataTableCountsAndPaginates(t *testing.T) {
	one := listing(t, "", docs, nil)
	has(t, one, "2 results")
	if strings.Contains(one, "ui-pagination") {
		t.Fatalf("one page needs no pagination: %s", one)
	}
	many := listing(t, "?per_page=1", docs, func(st *ListState) { st.Total = 9 })
	has(t, many, "9 results", `class="ui-pagination"`, `href="?page=2&amp;per_page=1"`)
}

func TestDataTableEmptyStateFillsTheTable(t *testing.T) {
	got := listing(t, "", nil, func(st *ListState) { st.Caption = "Documents" })
	has(t, got, `<caption>Documents</caption>`, `colspan="3"`, "Nothing here yet", "0 results")
	if strings.Contains(got, "<tbody><tr><td>") {
		t.Fatalf("a row was rendered with no rows: %s", got)
	}
	mine := listing(t, "", nil, func(st *ListState) { st.Empty = h.P(h.Text("No document yet")) })
	has(t, mine, "No document yet")
}

// Spec 065 (#97): a list that is empty and a list that was filtered down to
// empty are two different screens. Saying "nothing here" to somebody who just
// searched for "xyz" tells them the application is empty, when what happened is
// that their term matched nothing — and the way out is one link away.
func TestDataTableTellsEmptyFromFilteredToEmpty(t *testing.T) {
	empty := listing(t, "", nil, nil)
	has(t, empty, "Nothing here yet")
	if strings.Contains(empty, "Clear the search") {
		t.Fatalf("an empty list offered to clear a search nobody made: %s", empty)
	}

	filtered := listing(t, "?q=xyz&page=2", nil, func(st *ListState) { st.Search = "Search" })
	has(t, filtered, "No results for", "xyz", "Clear the search")
	// The way out drops the term and goes back to the first page: keeping
	// page=2 would clear the search into another empty screen.
	if !strings.Contains(filtered, `href="?"`) && !strings.Contains(filtered, `<a href=""`) {
		t.Fatalf("the way out keeps the query it was supposed to clear: %s", filtered)
	}
	if strings.Contains(filtered, "q=xyz&amp;page=2\" class=\"ui-btn") {
		t.Fatalf("the clear link kept the term: %s", filtered)
	}
}

func TestDataTableLinksTheRowAndSelectsIt(t *testing.T) {
	got := listing(t, "", docs, func(st *ListState) {
		st.RowHref = func(i int) string { return "/docs/" + docs[i].ID }
		st.Select = &ListSelect{
			Name:  "id",
			Value: func(i int) string { return docs[i].ID },
			Bar:   Button(h.Text("Delete")),
		}
	})
	has(t, got,
		`<tr class="ui-row-linked">`,
		`<a class="ui-row-link" href="/docs/1">nota.pdf</a>`,
		`<a class="ui-row-link" href="/docs/2">recibo.pdf</a>`,
		`<form class="ui-list-form" method="post">`,
		`<div class="ui-list-bulk">`,
		`<input type="checkbox" class="ui-checkbox" name="id" value="2">`,
		`<th class="ui-list-check" scope="col"><span class="ui-sr">Select all</span></th>`,
	)
	if !strings.Contains(got, "_csrf") {
		t.Fatalf("the bulk form went out without the token: %s", got)
	}
}

func TestPollAndOnAreJustAttributes(t *testing.T) {
	if got, want := render(t, h.Div(h.ID("s"), Poll("6s", "/docs/42/status"))),
		`<div id="s" data-trilha-poll="6s" data-trilha-src="/docs/42/status"></div>`; got != want {
		t.Errorf("Poll = %s, want %s", got, want)
	}
	if got, want := render(t, h.Div(h.ID("s"), Poll("30s", ""))),
		`<div id="s" data-trilha-poll="30s"></div>`; got != want {
		t.Errorf("Poll without a source = %s, want %s", got, want)
	}
	if got, want := render(t, h.Div(h.ID("s"), On("doc:42", "/docs/42/status"))),
		`<div id="s" data-trilha-on="doc:42" data-trilha-src="/docs/42/status"></div>`; got != want {
		t.Errorf("On = %s, want %s", got, want)
	}
	if got, want := render(t, h.Body(Live("/events"))), `<body data-trilha-live="/events"></body>`; got != want {
		t.Errorf("Live = %s, want %s", got, want)
	}
}

func TestLiveScriptIsOptIn(t *testing.T) {
	a := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var head, script string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		head, script = render(t, Head(c)), render(t, LiveScript(c))
		return h.Div(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if strings.Contains(head, "ui.live.js") {
		t.Fatalf("Head loads the file of who does not use it: %s", head)
	}
	if want := `<script src="/ui.live.js" defer></script>`; script != want {
		t.Fatalf("LiveScript = %s, want %s", script, want)
	}
}

// The JS is the other half of Poll, Live and On: without the pair of names it
// looks for, Go writes attributes nobody reads.
func TestUiLiveJSKnowsTheAttributes(t *testing.T) {
	js := string(Asset("ui.live.js"))
	for _, want := range []string{"data-trilha-poll", "data-trilha-src", "data-trilha-on", "data-trilha-live",
		"Trilha-Fragment", "Trilha-Poll", "Retry-After", "EventSource", "visibilitychange", "trilha:hydrate"} {
		if !strings.Contains(js, want) {
			t.Errorf("ui.live.js says nothing about %q", want)
		}
	}
	if !strings.Contains(string(Asset("ui.js")), "trilha:hydrate") {
		t.Error("ui.js does not hydrate what ui.live.js swaps")
	}
	var found bool
	for _, f := range Files {
		found = found || f == "ui.live.js"
	}
	if !found {
		t.Errorf("ui.live.js is not written to public/: %v", Files)
	}
	if len(js) > 6<<10 {
		t.Errorf("ui.live.js is %d bytes", len(js))
	}
}

// #118 — o fragmento que volta com a página inteira é um 200: nada erra, e só
// o navegador pode contar. O script conta, e só quando há para quem: o
// window.__trilha é escrito pelo script de dev, e em produção ele não existe.
func TestUiLiveJSContaOFragmentoQueVeioPaginaInteira(t *testing.T) {
	js := string(Asset("ui.live.js"))
	for _, want := range []string{"window.__trilha", "fragment-html"} {
		if !strings.Contains(js, want) {
			t.Fatalf("ui.live.js não fala %q", want)
		}
	}
}
