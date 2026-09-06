package trilha

import (
	"net/url"
	"strconv"
)

// DefaultPerPage and MaxPerPage bound the page size a listing reads from the
// query: an absent or impossible per_page becomes the default, and a client
// asking for more than the maximum gets the maximum. The repository never
// sees a number nobody agreed to.
const (
	DefaultPerPage = 20
	MaxPerPage     = 200
)

// ListParams is the state of a listing that lives in the URL: which page, how
// big, ordered by what, filtered by what text. Embed it in the struct of the
// screen and Bind fills it from the query, applies the limits and remembers
// the rest of the query so Href can preserve it.
//
//	type Listing struct {
//		trilha.ListParams
//		Status string `form:"status" validate:"oneof=|pending|done"`
//	}
//
//	var q Listing
//	if err := c.Bind(&q); err != nil { return nil, err }
//	docs, total, err := repo.List(c.Context(), q.Sort, q.Asc(), q.Limit(), q.Offset())
//
// Sort is a column name typed by whoever wrote the address, so it is not a
// column name until Restrict says it is — ui.DataTable calls Restrict with the
// columns it declared sortable.
type ListParams struct {
	Page    int    `form:"page" json:"page"`
	PerPage int    `form:"per_page" json:"per_page"`
	Sort    string `form:"sort" json:"sort"`
	Dir     string `form:"dir" json:"dir"`
	Q       string `form:"q" json:"q"`

	base url.Values // the query this came from, for Href
}

// after applies the defaults and limits and keeps the query for Href. Bind
// calls it once, right after the fields were read.
func (p *ListParams) after(form map[string][]string) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage <= 0 {
		p.PerPage = DefaultPerPage
	}
	if p.PerPage > MaxPerPage {
		p.PerPage = MaxPerPage
	}
	if p.Dir != "desc" {
		p.Dir = "asc"
	}
	p.base = url.Values{}
	for k, vals := range form {
		if k == CSRFField {
			continue
		}
		p.base[k] = append([]string(nil), vals...)
	}
}

// Offset is how many rows to skip, for the repository.
func (p ListParams) Offset() int { return (p.Page - 1) * p.PerPage }

// Limit is how many rows to read.
func (p ListParams) Limit() int { return p.PerPage }

// Asc reports the direction as a boolean, which is what a query builder wants.
func (p ListParams) Asc() bool { return p.Dir != "desc" }

// TotalPages is how many pages total rows make at this size, at least 1.
func (p ListParams) TotalPages(total int) int {
	if total <= 0 || p.PerPage <= 0 {
		return 1
	}
	n := (total + p.PerPage - 1) / p.PerPage
	if n < 1 {
		return 1
	}
	return n
}

// Restrict drops a Sort that is not one of cols and reports whether it did, so
// the caller can say so in the log. An address with a column that does not
// exist is a crooked address, not a failure: the listing answers unordered.
func (p *ListParams) Restrict(cols ...string) bool {
	if p.Sort == "" {
		return false
	}
	for _, c := range cols {
		if c == p.Sort {
			return false
		}
	}
	p.Sort, p.Dir = "", "asc"
	return true
}

// Href is the address of the same listing with some parameters changed and
// everything else — the filters of the screen included — preserved. Arguments
// are name/value pairs; an empty value removes the parameter, which is how a
// link goes back to the default:
//
//	p.Href("sort", "size", "dir", "desc", "page", "")
//
// The result is a query alone ("?page=3&q=note"), a relative URL that keeps
// the current path, so the same listing works wherever it is mounted.
func (p ListParams) Href(pairs ...string) string {
	v := url.Values{}
	for k, vals := range p.base {
		v[k] = append([]string(nil), vals...)
	}
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			v.Del(pairs[i])
			continue
		}
		v.Set(pairs[i], pairs[i+1])
	}
	if len(v) == 0 {
		return "?"
	}
	return "?" + v.Encode()
}

// PageHref is the address of page n, the shape ui.Pages wants.
func (p ListParams) PageHref(n int) string {
	if n <= 1 {
		return p.Href("page", "")
	}
	return p.Href("page", strconv.Itoa(n))
}
