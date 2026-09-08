---
title: Listings
description: ListParams reads the state of a listing from the URL; ui.DataTable renders it.
---

The screen every management app has — filter on top, table in the middle, pagination at
the foot — is a convention here, not a component you configure. The whole state lives in
the address, so the page can be shared, reloaded, bookmarked and used with the back
button, and the server is the only one who decides what a column name means.

## ListParams

`trilha.ListParams` is embedded in the struct of the screen and read by `c.Bind`, which
also applies the limits and remembers the rest of the query:

```go
type Listing struct {
	trilha.ListParams
	Status string `form:"status"`
}

var q Listing
if err := c.Bind(&q); err != nil { return nil, err }
docs, total := repo.List(q.Q, q.Status, q.Sort, q.Asc(), q.Offset(), q.Limit())
```

| Field | Parameter | Default |
|---|---|---|
| `Page int` | `page` | 1; anything below is 1 |
| `PerPage int` | `per_page` | `trilha.DefaultPerPage` (20), capped at `trilha.MaxPerPage` (200) |
| `Sort string` | `sort` | empty; a column name, until `Restrict` says it is one |
| `Dir string` | `dir` | `asc`; only the exact word `desc` turns it around |
| `Q string` | `q` | empty; the free text of the search box |

| Method | Answers |
|---|---|
| `Offset() int`, `Limit() int` | what the repository wants |
| `Asc() bool` | the direction as a boolean |
| `TotalPages(total int) int` | how many pages `total` rows make, at least 1 |
| `Restrict(cols ...string) bool` | drops a `Sort` that is not one of `cols`; reports whether it did |
| `Href(pairs ...string) string` | the same listing with some parameters changed and the rest preserved |
| `PageHref(n int) string` | the address of page `n`, the shape `ui.Pages` wants |

`Href` takes name/value pairs, and an empty value removes the parameter:

```go
p.Href("sort", "size", "dir", "desc", "page", "")   // ?dir=desc&q=note&sort=size&status=done
```

The result is a query alone, a relative URL that keeps the current path — so the same
listing works wherever it is mounted, and no link has to know where it is.

**`Sort` is a column name typed by whoever wrote the address.** `Restrict` is what turns
it into one: it drops a name that is not in the list and reports it, so the repository
never receives a column nobody declared. `ui.DataTable` calls it with the columns it
marked `Sort: true`; a listing that renders its own table calls it by hand.

## ui.DataTable

```go
return ui.DataTable(c, ui.Columns[Doc]{
	{Key: "name", Label: "File", Sort: true, Cell: func(d Doc) h.Node { return h.Text(d.Name) }},
	{Key: "size", Label: "Size", Sort: true, Num: true, Cell: func(d Doc) h.Node { return h.Text(d.Size()) }},
}, docs, ui.ListState{Params: q.ListParams, Total: total, ID: "list", Search: "Search"}), nil
```

`ui.Column[T]` is generic: the row is the type of your domain, not a `map[string]any`.

| Field of `Column[T]` | Role |
|---|---|
| `Key` | the name the URL orders by; also what `Sort` accepts |
| `Label` | the header text |
| `Sort` | the header becomes a real link that orders by this column |
| `Num` | numeric column, right-aligned with tabular figures |
| `Cell func(T) h.Node` | the cell of one row |

| Field of `ListState` | Role |
|---|---|
| `Params` | what `Bind` read; the links are built from it |
| `Total` | how many rows passed the filter, for the pagination and the count |
| `ID` | id of the fragment; with it, ordering, filtering and paging do not reload |
| `Search` | placeholder (and `aria-label`) of the `q` field; no search box when empty |
| `Filters` | the other fields of the filter form — a `<select>`, a date, whatever the screen has |
| `Empty` | what to show instead of the rows when there are none |
| `Caption` | `<caption>` of the table, read by a screen reader |
| `RowHref func(int) string` | the address the row at that position links to |
| `Select *ListSelect` | a checkbox per row and a bar with the actions that take the selection |

### There is no new script

The ordering header is a link, the filter is a `<form method=get>` and the pagination is
links: with JavaScript off, all three navigate and the route answers the whole page. With
`ID` set they carry `ui.Swap(ID)`, so the kit's `ui.js` — already on the page — asks for
the fragment and replaces the table. The handler is the same one either way:

```go
tabela := lista(c, q)
if c.Fragment() == "lista" {
	return tabela, nil
}
```

### Row selection

`ListSelect{Name, Value, Action, Label, Bar}` wraps the table in a `POST` form with the
CSRF token, adds a checkbox column and shows `Bar` above it. The route reads the
selection with `c.Request().Form[Name]` after `c.ParseForm`. `Value` is by position, like
`RowHref`: the row of index `i`.

### The filter form

Whatever `Params` holds and the form does not send travels in hidden inputs — the
ordering and the page size, so searching does not throw the ordering away. The page
itself is not carried: a new filter starts on page 1, which is the only page that is
certain to exist.

## The empty state

A list with nothing in it and a list filtered down to nothing are two different screens.
Saying "nothing here" to somebody who just searched for *xyz* tells them the application is
empty, when what happened is that their term matched nothing — and the way out is one link
away. `ui.DataTable` draws that distinction on its own, so a screen gets it right without
anybody thinking about it:

| Situation | What it shows |
|---|---|
| no rows, no search | an icon, "Nothing here yet" |
| no rows, `?q=xyz` | "No results for *xyz*", and a link that clears the term and goes back to page one |

`ListState.Empty` replaces both when the application has something better to say — a
Portuguese app, for one, since the kit's own strings are English.

The component behind it stands on its own:

```go
ui.Empty(ui.EmptyOpts{
	Icon:   "info",                        // a name from the kit; an unknown one draws nothing
	Title:  "No documents yet",            // the only required field
	Hint:   "Send the first PDF and classification starts on its own.",
	Action: ui.ButtonLink("/upload", h.Text("Send a document")),
})
```

`Hint` is the field that earns its place: it is the difference between telling somebody the
screen is empty and telling them what to do about it.

For a screen that could not load, `ui.EmptyError(c, title, err, action)` shows the title, the
way to retry, and the real error **only in development** — a driver's sentence on a production
page is an information leak with a friendly font.

