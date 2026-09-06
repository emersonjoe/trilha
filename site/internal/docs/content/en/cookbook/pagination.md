---
title: Pagination
description: Page by offset while the list is short, by cursor when it is not, one extra row instead of a COUNT, and links a crawler can follow.
---

A list that grows gets paginated twice: once with `LIMIT`/`OFFSET`, because it is obvious, and
once with a cursor, when someone notices page 900 takes four seconds. Both are here, and the
first is fine for most lists.

## The window

```go
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
```

The ceiling is not paranoia. `OFFSET 18000` makes the database read and discard eighteen
thousand rows, and a crawler will ask for page 900 of everything you publish.

```go
// Window is a page of a list: what was asked for, and whether there is more.
type Window struct {
	Page, Size int
	HasNext    bool
}
```

## One extra row

```go
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
```

Asking for `Size+1` and showing `Size` answers "is there a next page?" without a second query
counting the whole table. `COUNT(*)` on a large table is the query that shows up in the slow
log two months later — and the total is almost never what the footer needs.

The order has two columns for a reason: `published_at` alone is not unique, and a tie split
across a page boundary shows the same row twice or skips it.

## The cursor

```go
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
```

The `WHERE (published_at, id) < ($1, $2)` is what makes it cheap: with the index on those two
columns, the database jumps to the position instead of counting to it, so page one thousand
costs what page one costs. It also fixes the bug offset pagination has by design — a row
inserted while someone reads shifts every page after it.

```go
// Cursor packs the sort key of the last row. Base64 so it survives a URL,
// not because it is a secret: whoever edits it sees another page of the
// same public list and nothing else.
func Cursor(a Article) string {
	raw := a.Published.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(a.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}
```

Base64 because it travels in a URL, not because it hides anything: whoever edits it sees
another page of the same public list.

## The footer

```go
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
```

Links, not buttons. A page number belongs in the address so it can be shared, reloaded and
indexed; `rel="prev"`/`rel="next"` is what a crawler reads to understand the sequence.

:::tip
`c.Fragment()` turns this into a list that grows in place without a full reload: the handler
answers only the `<ul>` when the request is a fragment. See
[Interactivity](/learn/interactivity).
:::

:::note
Which one to use: offset while the list is browsed by people who jump to a page, cursor for
anything a machine walks through — an API, an export, an infinite scroll. The API answer is
the cursor, always: page numbers over a changing list return duplicates.
:::
