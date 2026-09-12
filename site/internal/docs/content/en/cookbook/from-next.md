---
title: From Next.js to Trilha
description: The React patterns you reach for without thinking — useEffect + fetch, useState for a modal, toast, useSearchParams — and the one line of Go that replaces each.
---

This page is for whoever already builds screens in Next.js and is now looking at a Go file
wondering where the state went. It is a translation table, not an argument: the app being
moved is a dashboard of articles — a list with filter and ordering, a queue that refreshes
itself, a confirmation modal, a PDF — and each section shows the React you would write and
the Trilha that replaces it.

The short version of the whole page: **the state that lived in the browser lives in the URL
or on the server**, and what is left in the browser is the little the kit already ships.

| In the React app | Here |
|---|---|
| `useEffect` + `fetch` + `useState(loading)` | a page function that reads the data and returns the HTML |
| a Server Action (`"use server"`) and `<form action={…}>` | a `POST` beside the page that writes and `c.Redirect`s |
| `useSearchParams` and a state per filter | [`trilha.ListParams`](/reference/listings) read by `c.Bind` |
| `setInterval` + cleanup in `useEffect` | [`ui.Poll`](/reference/live) |
| `toast()` from a provider | `c.Flash` |
| `useState(open)` + portal + Escape handler | [`ui.Dialog`](/reference/ui) |
| `createObjectURL` over a blob | `c.Inline` |
| `if (!user) router.replace("/login")` | `Auth.Require()` in the branch's `middleware.go` |
| sidebar state kept in `localStorage` | [`ui.Shell`](/reference/shell) |
| `dangerouslySetInnerHTML` | [`ui.Markdown`](/reference/ui) |
| a chart library on the client | `c.Island` |
| `next.config.js` | `app/setup.go` and `Config` |

Before the first line of Go, `trilha migrate next ../web` writes the folder tree of `app/` and
a `MIGRATION.md` saying, screen by screen, which file it came from, what it called and which
of the shapes below it probably is. The skeleton compiles; the port is what is left, and this
page is the reference for it. See
[the command](/reference/cli#trilha-migrate).

## The screen that fetches

The React dashboard: an effect, three states, and a render that has to say something
sensible in each of them.

```js
export default function Dashboard() {
  const [rows, setRows] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  useEffect(() => {
    fetch("/api/articles")
      .then(r => r.json()).then(setRows)
      .catch(setError).finally(() => setLoading(false))
  }, [])
  if (loading) return <Spinner />
  if (error) return <Error err={error} />
  return <Table rows={rows} />
}
```

The same screen here is one function. It runs on the server, so reading the database is a
function call and the page arrives filled:

```go
// Dashboard is app/dashboard/page.go. There is no useEffect, no loading flag
// and no error state: the function runs on the server, so it reads the
// database the way any Go function does and returns the page already filled.
// What React needed three renders for happens here before the first byte.
func Dashboard(c *trilha.Ctx) (h.Node, error) {
	var q DashQuery
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	rows, total, err := dashRows(c.Context(), q)
	if err != nil {
		return nil, err
	}
	return ui.DataTable(c, DashColumns, rows, ui.ListState{
		Params: q.ListParams,
		Total:  total,
		ID:     "articles",
		Search: "Search articles",
		Empty:  h.Text("No article matches this filter."),
		RowHref: func(i int) string {
			return "/dashboard/" + rows[i].Slug
		},
	}), nil
}
```

There is no loading state because there is no moment when the page exists without the data,
and no error state because the error is a returned `error` — the framework turns it into the
status and the page the visitor sees.

## The filters that were state

Every filter in the React app was a `useState` plus a `useSearchParams` to keep it in the
address, plus an effect to refetch when either changed. Here the address *is* the state:

```go
// DashQuery is the whole state the dashboard used to keep in React: the page,
// the ordering, the search box and the type filter. Embedding ListParams is
// what makes them live in the address instead of in memory — the visitor can
// bookmark the screen, reload it, or send it to somebody else.
type DashQuery struct {
	trilha.ListParams
	Status string `form:"status"`
}
```

`c.Bind` fills both halves from the query string, applies the ceilings of a listing (page at
least 1, page size capped) and hands you a struct. The columns say which of them may be
ordered by:

```go
// DashColumns replaces the <thead> written by hand and the onClick that
// re-sorted the array in the browser. Sort: true turns the header into a real
// link — the ordering is a GET, so it works with the script blocked and it is
// still there after a reload.
var DashColumns = ui.Columns[Article]{
	{Key: "title", Label: "Title", Sort: true, Cell: func(a Article) h.Node {
		return h.Text(a.Title)
	}},
	{Key: "published", Label: "Published", Sort: true, Cell: func(a Article) h.Node {
		return h.Text(a.Published.Format("2006-01-02"))
	}},
}
```

And the query is where the URL stops being trusted:

```go
// dashRows is the query behind the screen. The ordering comes from the URL,
// so it is checked before it reaches SQL: Restrict drops a column nobody
// declared, and what is left is one of two names this function knows.
func dashRows(ctx context.Context, q DashQuery) ([]Article, int, error) {
	q.Restrict("title", "published")
	order := `published_at DESC`
	if q.Sort == "title" {
		order = `title`
		if !q.Asc() {
			order = `title DESC`
		}
	}
	rows, err := DB.QueryContext(ctx,
		`SELECT id, slug, title, published_at FROM articles WHERE ($1 = '' OR title ILIKE '%' || $1 || '%') AND ($2 = '' OR status = $2) ORDER BY `+order+` LIMIT $3 OFFSET $4`,
		q.Q, q.Status, q.Limit(), q.Offset())
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Published); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	err = DB.QueryRowContext(ctx, `SELECT count(*) FROM articles WHERE ($1 = '' OR status = $1)`, q.Status).Scan(&total)
	return out, total, err
}
```

## The interval that leaked

```js
useEffect(() => {
  const id = setInterval(() => refetch(), 6000)
  return () => clearInterval(id)
}, [])
```

The fragment asks for itself, and the server is the one that decides when to stop:

```go
// Queue is the block that used to be a useEffect with a setInterval and a
// cleanup nobody remembered to write. ui.Poll asks this same route for the
// fragment every six seconds; when the work is done the handler stops the
// clock from the server side and the browser stops asking.
func Queue(c *trilha.Ctx) (h.Node, error) {
	left, err := pendingArticles(c.Context())
	if err != nil {
		return nil, err
	}
	if left == 0 {
		c.PollStop()
		return h.Div(h.ID("queue"), h.Text("Everything processed.")), nil
	}
	return h.Div(h.ID("queue"), ui.Poll("6s", "#queue"),
		h.Text(strconv.Itoa(left)+" articles in the queue"),
	), nil
}
```

`c.PollStop()` writes a header the kit reads; `c.PollEvery(d)` changes the interval from the
same place. The browser also pauses while the tab is hidden — behaviour you would have had to
write, and remember to write again on the next screen.

## The toast

`toast.success("Article published")` needs a provider at the root, a portal and a state that
survives the navigation. A flash is written before the redirect and shown by the page that
comes after it:

```go
// Publish is the POST behind the button. The toast() of the React app is a
// flash here: the message is written before the redirect and shown by the
// page that comes next, so a refresh does not repeat the action and the
// message survives the navigation without any state to carry.
func Publish(c *trilha.Ctx) error {
	if _, err := DB.ExecContext(c.Context(), `UPDATE articles SET status = 'published' WHERE slug = $1`, c.Param("slug")); err != nil {
		return err
	}
	c.Flash(ui.FlashSuccess, "Article published")
	return c.Redirect("/dashboard")
}
```

## The modal

The React modal is a `useState(false)`, a portal, an effect for Escape and a focus trap from
a library. `<dialog>` does all four:

```go
// ConfirmDelete is the modal that used to be a useState(false), a portal and
// an effect closing it on Escape. The dialog is in the HTML from the start,
// closed; the browser opens it, traps the focus and closes it on Escape by
// itself, because <dialog> already does all of that.
func ConfirmDelete(slug string) h.Node {
	return h.Div(
		ui.DialogTrigger("delete", h.Text("Delete")),
		ui.Dialog("delete", "Delete this article?",
			h.P(h.Text("This cannot be undone.")),
			h.Form(h.Method("post"), h.Action("/dashboard/"+slug+"/delete"),
				ui.Button(ui.Destructive(), h.Text("Delete")),
			),
		),
	)
}
```

## The blob

```js
const res = await fetch(`/api/preview/${slug}`)
const url = URL.createObjectURL(await res.blob())
setSrc(url)                       // and revoke it, if the component lives that long
```

An address is simpler than a blob, and it survives a reload:

```go
// Preview answers the PDF the <iframe> shows. In the Next.js app this was a
// route handler returning a blob, a createObjectURL and a revoke that leaked
// when the component unmounted early: here the browser asks for a URL and
// gets a document.
//
// Nothing else is needed for the <iframe>: Inline is the answer that says it
// may be framed by a page of this origin. It is the framed document that
// refuses, never the page around it — a page adding frame-src to its own
// policy changes nothing while the file it frames still carries DENY.
func Preview(c *trilha.Ctx) error {
	f, err := os.Open("var/previews/" + c.Param("slug") + ".pdf")
	if err != nil {
		return trilha.ErrNotFound
	}
	defer f.Close()
	return c.Inline("preview.pdf", f, "application/pdf")
}
```

## The redirect nobody wrote twice

`if (!user) router.replace("/login")` at the top of every page is a rule enforced by
repetition — and the eleventh page forgets. Here the rule is the folder:

```go
// Middleware is the whole of app/dashboard/middleware.go, and it replaces the
// `if (!user) router.replace("/login")` that every page repeated. The check
// runs before the handler, so a route added tomorrow is covered by having been
// put in this folder — not by remembering the first line.
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return Panel.Require()(c, next)
}
```

Everything under `app/dashboard/` is covered by having been put there. The `Panel` it uses is
built once, in `app/setup.go`:

```go
// Panel is the session of the migrated app. It is set in app/setup.go; the
// pages never touch it.
var Panel *auth.Auth
```

## The frame

Sidebar, top bar, user menu, the active link, the collapsed state in `localStorage`: a week of
work in the React app, and an options struct here.

```go
// Frame is app/dashboard/layout.go: the sidebar, the top bar and the user
// menu that the React app assembled from a component library, a state for the
// collapsed sidebar and a localStorage to remember it. Shell renders the
// frame; the current item comes from the path, so no page has to say which
// link is active.
func Frame(c *trilha.Ctx, children ...h.Node) h.Node {
	return ui.Shell(c, ui.ShellOpts{
		Brand:   h.Text("Editorial"),
		Current: c.Request().URL.Path,
		Nav: []ui.NavGroup{{Label: "Content", Items: []ui.NavItem{
			{Href: "/dashboard", Label: "Articles", Icon: "file"},
			{Href: "/dashboard/queue", Label: "Queue", Icon: "clock"},
		}}},
		User: ui.UserMenu{Name: "Editor", Items: []h.Node{
			ui.MenuLink("/logout", h.Text("Sign out")),
		}},
	}, children...)
}
```

## The HTML you did not write

`dangerouslySetInnerHTML` is the escape hatch that ends in an incident. `ui.Markdown` renders
to a tree of nodes, so a tag in the source is text in the output:

```go
// Notes renders text written by a person or by a model. It is the answer to
// dangerouslySetInnerHTML: the source becomes a tree of nodes, a tag in the
// source comes out as text, and there is no string on the way that anybody
// could hand to h.Raw.
func Notes(src string) h.Node {
	return ui.Markdown(src, ui.MarkdownOpts{HeadingBase: 3})
}
```

## What stays in the browser

Not everything should move. A chart library, a map, a rich-text editor — code that genuinely
runs on the client — becomes an island: a module, its props as JSON, and the server-rendered
children as the fallback.

```go
// Sales is the one place the React answer was right: a chart library that
// runs in the browser. The island loads the module, hands it the data as
// JSON and mounts it on this element — and the children are what a visitor
// with the script blocked reads instead.
func Sales(c *trilha.Ctx, points []float64) h.Node {
	return c.Island("/chart.js", map[string]any{"points": points},
		h.Class("chart"),
		ui.Sparkline(points, ui.SparkOpts{}),
	)
}
```

That is the whole client-side story: no bundler, no hydration of the rest of the page, and
the fallback is what a visitor with the script blocked reads.

## `next.config.js`

```go
// SetupPreviews is the last piece of the move. next.config.js had rewrites,
// headers and a public/ folder; here public/ is served by the framework and
// the rest is this: a mount for the generated files. The <iframe> needs no
// entry at all — that line used to be here, and it never did anything.
func SetupPreviews(cfg *trilha.Config) {
	cfg.Mounts = map[string]fs.FS{"/previews/": os.DirFS("var/previews")}
}
```

## Finding the name

The reflex "which component is this called?" has an answer that does not need the browser:

```bash
trilha ui describe            # everything, grouped
trilha ui describe DataTable  # signature, what it is for, an example
```

It reads a catalogue built from the kit's own doc comments, so it cannot drift from the code
it describes — and `--json` makes it readable by the agent writing the screen with you.

:::note
Two habits are worth dropping early. The first is reaching for an API route: in Next.js a
page that needs data needs an endpoint, and here the page *is* the server — an `app/api/`
route is for what somebody else calls. The second is the client component: there is no
client bundle to be careful about, so the question is never "server or client?", only "does
this need to run in the browser at all?".
:::
