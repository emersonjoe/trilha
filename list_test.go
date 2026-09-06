package trilha

import (
	"net/http/httptest"
	"testing"
)

type listagem struct {
	ListParams
	Status string `form:"status"`
}

// bindList runs one GET through a real route and gives back what Bind read.
func bindList(t *testing.T, query string) listagem {
	t.Helper()
	var got listagem
	a := New(Config{Logger: quiet()})
	a.Register(Route{Pattern: "/docs", Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
		if err := c.Bind(&got); err != nil {
			return err
		}
		return c.JSON(200, "ok")
	}}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/docs"+query, nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	return got
}

func TestBindReadsEmbeddedListParams(t *testing.T) {
	q := bindList(t, "?page=3&per_page=50&sort=size&dir=desc&q=nota&status=done")
	if q.Page != 3 || q.PerPage != 50 || q.Sort != "size" || q.Dir != "desc" {
		t.Fatalf("params = %+v", q.ListParams)
	}
	if q.Q != "nota" || q.Status != "done" {
		t.Fatalf("the filters of the screen were lost: %+v", q)
	}
}

func TestListParamsApplyTheLimits(t *testing.T) {
	cases := []struct {
		query   string
		page    int
		perPage int
		dir     string
	}{
		{"", 1, DefaultPerPage, "asc"},
		{"?page=0&per_page=0", 1, DefaultPerPage, "asc"},
		{"?page=-2", 1, DefaultPerPage, "asc"},
		{"?per_page=1000", 1, MaxPerPage, "asc"},
		{"?dir=DESC", 1, DefaultPerPage, "asc"}, // only the exact word turns it around
		{"?dir=desc", 1, DefaultPerPage, "desc"},
	}
	for _, tc := range cases {
		q := bindList(t, tc.query)
		if q.Page != tc.page || q.PerPage != tc.perPage || q.Dir != tc.dir {
			t.Errorf("%q = page %d, per_page %d, dir %q; want %d, %d, %q",
				tc.query, q.Page, q.PerPage, q.Dir, tc.page, tc.perPage, tc.dir)
		}
	}
}

func TestListHrefKeepsTheRestOfTheQuery(t *testing.T) {
	q := bindList(t, "?page=2&q=nota&status=done")
	if got, want := q.Href("page", "3"), "?page=3&q=nota&status=done"; got != want {
		t.Errorf("Href = %q, want %q", got, want)
	}
	if got, want := q.Href("q", ""), "?page=2&status=done"; got != want {
		t.Errorf("an empty value has to remove the parameter: %q, want %q", got, want)
	}
	if got, want := q.Href("sort", "size", "dir", "desc", "page", ""),
		"?dir=desc&q=nota&sort=size&status=done"; got != want {
		t.Errorf("Href = %q, want %q", got, want)
	}
	if got := bindList(t, "").Href(); got != "?" {
		t.Errorf("with nothing in the query Href = %q, want %q", got, "?")
	}
}

func TestListHrefIgnoresTheCSRFTokenAndAnOddPair(t *testing.T) {
	q := bindList(t, "?page=2&"+CSRFField+"=abc")
	if got, want := q.Href("page", "3", "sort"), "?page=3"; got != want {
		t.Errorf("Href = %q, want %q", got, want)
	}
}

func TestListParamsAnswerTheRepository(t *testing.T) {
	q := bindList(t, "?page=3&per_page=25&dir=desc")
	if q.Offset() != 50 || q.Limit() != 25 || q.Asc() {
		t.Errorf("offset %d, limit %d, asc %v", q.Offset(), q.Limit(), q.Asc())
	}
	if got := q.PageHref(1); got != "?dir=desc&per_page=25" {
		t.Errorf("page 1 keeps a page parameter: %q", got)
	}
	for _, tc := range []struct{ total, pages int }{{0, 1}, {1, 1}, {25, 1}, {26, 2}, {75, 3}} {
		if got := q.TotalPages(tc.total); got != tc.pages {
			t.Errorf("TotalPages(%d) = %d, want %d", tc.total, got, tc.pages)
		}
	}
}

func TestRestrictDropsAColumnNobodyDeclared(t *testing.T) {
	q := bindList(t, "?sort=senha&dir=desc")
	if !q.Restrict("name", "size") {
		t.Fatal("Restrict accepted a column that was not declared")
	}
	if q.Sort != "" || q.Dir != "asc" {
		t.Errorf("after Restrict: sort %q, dir %q", q.Sort, q.Dir)
	}
	ok := bindList(t, "?sort=size&dir=desc")
	if ok.Restrict("name", "size") {
		t.Fatal("Restrict dropped a declared column")
	}
	if ok.Sort != "size" || ok.Dir != "desc" {
		t.Errorf("a declared column changed: sort %q, dir %q", ok.Sort, ok.Dir)
	}
	none := bindList(t, "")
	if none.Restrict("name") {
		t.Error("Restrict had something to drop with no sort at all")
	}
}
