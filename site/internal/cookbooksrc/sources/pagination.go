package cookbook

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// PageSize is how many rows one page shows.
const PageSize = 20

// Window is a page of a list: what was asked for, and whether there is more.
type Window struct {
	Page, Size int
	HasNext    bool
}

// WindowFrom reads ?page= and refuses what it cannot serve. The ceiling is
// not paranoia: OFFSET 900000 makes the database walk every row it skips,
// and a crawler will ask.
func WindowFrom(c *trilha.Ctx) Window {
	n, err := strconv.Atoi(c.Query("page"))
	if err != nil || n < 1 {
		n = 1
	}
	if n > 500 {
		n = 500
	}
	return Window{Page: n, Size: PageSize}
}

// ArticlesPage reads one page by offset. It asks for one row more than it
// shows: that extra row is how you know there is a next page without a
// second query counting the whole table.
func ArticlesPage(ctx context.Context, w Window) ([]Article, Window, error) {
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles ORDER BY published_at DESC, id DESC LIMIT $1 OFFSET $2`,
		w.Size+1, (w.Page-1)*w.Size)
	if err != nil {
		return nil, w, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, w, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, w, err
	}
	if len(out) > w.Size {
		out, w.HasNext = out[:w.Size], true
	}
	return out, w, nil
}

// ArticlesAfter reads the next rows by cursor. The database jumps straight
// to the position with the index, so page one thousand costs the same as
// page one — and a row inserted meanwhile does not shift the whole list.
func ArticlesAfter(ctx context.Context, cursor string, size int) ([]Article, string, error) {
	at, id := time.Now().Add(100*365*24*time.Hour), int64(0)
	if cursor != "" {
		var err error
		if at, id, err = parseCursor(cursor); err != nil {
			return nil, "", err
		}
	}
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles
		 WHERE (published_at, id) < ($1, $2)
		 ORDER BY published_at DESC, id DESC LIMIT $3`,
		at, id, size)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, "", err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) < size {
		return out, "", nil // the end: no cursor to hand back
	}
	return out, Cursor(out[len(out)-1]), nil
}

// Cursor packs the sort key of the last row. Base64 so it survives a URL,
// not because it is a secret: whoever edits it sees another page of the
// same public list and nothing else.
func Cursor(a Article) string {
	raw := a.Published.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(a.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func parseCursor(s string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, err
	}
	stamp, id, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, 0, errors.New("cookbook: malformed cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return time.Time{}, 0, err
	}
	n, err := strconv.ParseInt(id, 10, 64)
	return at, n, err
}

// Pages renders the footer of a list. They are links, not buttons: a page
// number belongs in the address, so it can be shared, reloaded and read by
// whoever indexes the site.
func Pages(path string, w Window) h.Node {
	href := func(n int) string { return path + "?page=" + strconv.Itoa(n) }
	return h.Nav(h.Class("paginas"), h.Aria("label", "Pagination"),
		h.If(w.Page > 1, h.A(h.Rel("prev"), h.Href(href(w.Page-1)), h.Text("Previous"))),
		h.Span(h.Textf("Page %d", w.Page)),
		h.If(w.HasNext, h.A(h.Rel("next"), h.Href(href(w.Page+1)), h.Text("Next"))),
	)
}
