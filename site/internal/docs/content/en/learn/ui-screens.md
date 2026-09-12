---
title: Ready-made screens
description: One internal-app screen end to end — frame, numbers, a table that lives in the URL, the slow part, the file, the form in steps — built out of the kit you already have.
---

The [previous chapter](/learn/ui-kit) is a catalogue: thirty components, one at a time, each
with its demo. This one is a screen. The same pieces, in the order somebody actually writes
them, with the reason each one is there — because knowing what `ui.DataTable` is does not tell
you where it sits, who fills it, or what the handler above it still owes.

The screen is the orders console of an internal app: the frame around it, four numbers on top,
a filterable table in the middle, a chart that arrives late, a row that opens a file, a form
that takes three screens, and one cell that changes without anybody clicking. Every block below
is a declaration of `examples/cookbook/screens.go`, so all of it compiles with the rest of the
repository.

## The screen, and where its files go

Routing is the folder tree, so the screen is already described by where its files are:

```text
app/
  layout.go                    ← the frame: ui.Shell around every page
  orders/
    page.go                    ← the listing, and the table as a fragment
    insights/page.go           ← what ui.Defer asks for
    new/
      page.go                  ← step one
      items/page.go            ← step two: GET renders, POST saves the draft
      confirm/page.go          ← step three
    id_/
      page.go                  ← one order: metadata and the invoice
      state/page.go            ← the live cell, on its own route
```

No route is registered by hand and no file registers itself: `trilha gen` reads the tree. See
[Pages and routes](/learn/pages-and-routes).

## The frame

`ui.Shell` is the sidebar, the top bar and the user's menu written once, in the root layout.
It is a composition of components that already existed — `ui.Sidebar`, `ui.Nav`, `ui.Menu` —
so anything it does not cover you write beside it with the same pieces. The active item is the
one whose `Href` is the longest prefix of the current path, which is why `/orders/42` lights up
**Orders** and not **Dashboard**.

```go
// OrdersLayout is the frame, written once for every screen of the app.
// ui.Shell is a composition of components that already existed, so anything it
// does not cover is written beside it with the same pieces. ui.LiveScript goes
// in the head because the screen has a fragment that arrives late and a cell
// that refreshes itself; a page with neither never downloads it.
func OrdersLayout(c *trilha.Ctx, children h.Node) (h.Node, error) {
	return h.Html(h.Lang("en"),
		h.Head(
			h.Meta(h.Charset("utf-8")),
			h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
			h.Title(h.Text("Acme Ops")),
			ui.Head(c),
			ui.LiveScript(c),
		),
		h.Body(ui.Body(),
			ui.Shell(c, ui.ShellOpts{
				Brand: h.A(h.Class("ui-brand"), h.Href("/"), h.Text("Acme Ops")),
				Nav: []ui.NavGroup{
					{Label: "Work", Items: []ui.NavItem{
						{Href: "/", Label: "Dashboard", Icon: "house"},
						{Href: "/orders", Label: "Orders", Icon: "search"},
					}},
					// Hide is a courtesy to whoever reads the menu, never a
					// permission: what keeps somebody out of /settings is the
					// middleware.go at the root of that folder.
					{Label: "Admin", Hide: !ordersAdmin(c), Items: []ui.NavItem{
						{Href: "/settings", Label: "Settings", Icon: "settings"},
					}},
				},
				User:   ui.UserMenu{Name: "Ana Reis", Detail: "ana@acme.example", Items: []h.Node{ui.MenuLink("/profile", h.Text("Profile"))}},
				Header: []h.Node{ui.ThemeToggle()},
			}, children),
			ui.Flashes(c),
		),
	), nil
}
```

Two decisions worth naming. `ui.LiveScript(c)` is in the head because this screen has a
fragment that arrives late and a cell that refreshes itself; `ui.Head` does not load it, and a
page with neither never downloads it. And `Hide` on the Admin group is a **courtesy to whoever
reads the menu, not a permission** — the same question is asked again, for real, by the
`middleware.go` at the root of `/settings`:

```go
// ordersAdmin is the question the menu asks, and the same one the middleware
// at the root of /settings asks again — the menu hides, the middleware
// refuses, and only one of the two is a permission.
func ordersAdmin(c *trilha.Ctx) bool {
	role, _ := c.Get("role").(string)
	return role == "admin"
}
```

See [Shell](/reference/shell) for `Current`, the collapsing sidebar and `IconNode`, and
[the demo](/learn/ui-kit#the-frame-of-an-internal-app) for the frame rendered.

## What an order is, and what it can be

The row is the app's own type. `ui.Columns[T]` is generic, so the table is checked by the
compiler instead of by a `map[string]any` that renders blank when a field changes name:

```go
// Order is the row of the screen. The type is the app's, not the kit's:
// ui.Columns is generic, so a field that changes name stops compiling instead
// of rendering blank.
type Order struct {
	ID       string
	Customer string
	Total    float64
	State    string
	Updated  time.Time
	Invoice  string // where the PDF is; empty until the order ships
}
```

The states are one list, declared once:

```go
// OrderStates is the single declaration of what an order can be: the value the
// database holds, the word a person reads and the tone the badge wears. The
// filter's select, the form's validation and ui.Status all read this list, so
// a state added here shows up in the three of them at once.
var OrderStates = trilha.Enum{
	{Value: "draft", Label: "Draft"},
	{Value: "picking", Label: "Picking", Tone: "info"},
	{Value: "shipped", Label: "Shipped", Tone: "success"},
	{Value: "returned", Label: "Returned", Tone: "danger"},
}
```

That list is read by four different things — the filter's `<select>`, the form's validation
(`validate:"enum=ops.OrderState"`, after `trilha.RegisterEnum`), the badge in the table, and
the badge on the detail screen. Adding a state is one line here, and the four follow. See
[Validation](/reference/validation#enum-a-domain-list-declared-once) and
[the demo](/learn/ui-kit#one-value-of-an-enum-as-a-badge).

## One handler, three answers

The whole screen is one function. It answers the page, and it answers the table alone when the
kit swaps it — the id of the fragment is the id of the element being replaced, and that is the
entire protocol:

```go
// OrdersPage is the screen and each of its pieces. One handler answers three
// requests — the whole page, the table when the kit swaps it, and nothing else
// — because the id of the fragment is the id of the element being replaced,
// and that is the entire protocol.
func OrdersPage(c *trilha.Ctx) (h.Node, error) {
	var q OrdersQuery
	if err := c.Bind(&q); err != nil {
		return nil, err
	}
	// Sort arrived from the address, typed by whoever wrote it. Restrict is
	// what turns it into a column name, before the repository sees it.
	q.Restrict("customer", "total", "updated")
	rows, total, err := orders.Search(q)
	if err != nil {
		return nil, err
	}
	table := ordersTable(c, q, rows, total)
	if c.Fragment() == "orders" {
		return table, nil
	}
	sum, err := orders.Summary()
	if err != nil {
		return nil, err
	}
	c.SetTitle("Orders")
	return h.Div(
		ui.PageHeader("Orders", ui.ButtonLink("/orders/new", ui.Icon("plus"), h.Text("New order"))),
		ui.Grid(orderStats(sum)...),
		ui.Defer(c, "insights", "/orders/insights", ui.DeferOpts{Height: "12rem"}),
		table,
	), nil
}
```

`c.Bind` fills the struct from the query and applies the limits of `ListParams`; `Restrict` is
the line that matters for safety, because `Sort` arrived from the address, typed by whoever
wrote it, and it is not a column name until the screen says which names exist. See
[Interactivity](/learn/interactivity) for the fragment and [Listings](/reference/listings) for
the rest.

## Four numbers on top

Three counts and one number that has a shape. The sparkline is the size of a line of text, in
SVG written by the server: it arrives with the page, it prints, and it costs no download.

```go
// orderStats is the row of numbers on top. Three of them are counts and the
// fourth has a shape, which is what a sparkline is for: a drawing the size of
// a line of text, in SVG written by the server, next to the number it is
// about.
func orderStats(s OrdersSummary) []h.Node {
	return []h.Node{
		ui.Card(ui.Stat("Open", strconv.Itoa(s.Open))),
		ui.Card(ui.Stat("Shipped today", strconv.Itoa(s.ShippedToday))),
		ui.Card(ui.Stat("Revenue", dollars(s.Revenue), ui.StatHint("this month"))),
		ui.Card(ui.Stat("Late", strconv.Itoa(s.Late),
			ui.SparklineTitle(s.LateByDay, ui.SparkOpts{}, ui.ChartTitle("Late orders, last 14 days")))),
	}
}

// dollars is money as text, because ui.Stat takes a string: the framework has
// no currency, so where the symbol goes is the app's decision. In a cell,
// where a node fits, ui.Number does the digits by the locale of the Ctx.
func dollars(v float64) string { return "$" + strconv.FormatFloat(v, 'f', 2, 64) }
```

`ui.ChartTitle` is what turns the drawing into an image with a name instead of decoration, and
`dollars` is a reminder that the framework has no currency: `ui.Stat` takes text, so the symbol
is the app's decision. Where a node fits — a table cell — `ui.Number` and `ui.Date` read the
locale off the `Ctx` instead. See [Charts](/reference/charts),
[Formatting](/reference/ui#formatting) and
[the demo](/learn/ui-kit#four-numbers-and-the-drawings-beside-them).

## The table lives in the URL

The state of the listing is a struct, and all of it is in the address, so the screen can be
shared, reloaded, bookmarked and walked back with the browser's own button:

```go
// OrdersQuery is the state of the screen, and all of it is in the address:
// ListParams brings page, order and search, and the fields beside it are the
// filters this screen added. c.Bind fills the whole thing, applies the limits
// of ListParams and keeps the rest of the query so the links preserve it.
type OrdersQuery struct {
	trilha.ListParams
	State string `form:"state" validate:"omitempty,enum=ops.OrderState"`
}
```

The columns are the app's type again. Two of the cells are formatted, which is why they take
the `Ctx`:

```go
// ordersColumns names the table. Key is what the URL orders by and what
// Restrict accepts; Cell renders one row of the app's own type. It takes the
// Ctx because two of the cells are formatted — the number and the date read
// the locale off it, and the same screen serves both languages.
func ordersColumns(c *trilha.Ctx) ui.Columns[Order] {
	return ui.Columns[Order]{
		{Key: "customer", Label: "Customer", Sort: true, Cell: func(o Order) h.Node { return h.Text(o.Customer) }},
		{Key: "total", Label: "Total", Sort: true, Num: true, Cell: func(o Order) h.Node {
			return ui.Number(c, o.Total, ui.Decimals(2))
		}},
		{Key: "state", Label: "State", Cell: orderStateCell},
		{Key: "updated", Label: "Updated", Sort: true, Cell: func(o Order) h.Node {
			return ui.Date(c, o.Updated, ui.Relative())
		}},
	}
}
```

And the table is one call, with everything the URL said about it:

```go
// ordersTable is the listing: the filter form on top, the sortable headers,
// the pagination at the foot and the empty state, all of it built from what
// Bind read. ID is what makes ordering, filtering and paging swap the table
// instead of reloading the screen.
func ordersTable(c *trilha.Ctx, q OrdersQuery, rows []Order, total int) h.Node {
	return ui.DataTable(c, ordersColumns(c), rows, ui.ListState{
		Params:  q.ListParams,
		Total:   total,
		ID:      "orders",
		Search:  "Search by customer",
		Filters: ordersFilters(q),
		Empty:   ordersEmpty(q),
		Cards:   true,
		RowHref: func(i int) string { return "/orders/" + rows[i].ID },
	})
}
```

```go
// ordersFilters is the rest of the filter form — the search box is the kit's.
// The select's options come from the enum, so the filter cannot offer a state
// the validation would refuse.
func ordersFilters(q OrdersQuery) h.Node {
	return ui.Field("state", "State", ui.Select(h.ID("state"), h.Name("state"),
		OrderStates.Options(q.State, "Any state")))
}
```

`ID` is what makes ordering, filtering and paging swap the table instead of reloading the
screen — with JavaScript off, the same links navigate and the same form submits. `RowHref`
makes the row open the order. The filter's options come from the enum, so the screen cannot
offer a state the validation would refuse. See [Listings](/reference/listings) and
[the demo](/learn/ui-kit#tables-that-live-in-the-url).

## Filtered down to nothing

A list with nothing in it and a list a filter emptied are two different screens. The first one
says "create the first order"; the second one has to say **what to undo**, or the person is
looking at an app that appears to have lost their data:

```go
// ordersEmpty is the screen a filter that matched nothing shows. It is not the
// same screen as an app with no orders at all: this one says what to undo, and
// the way out is the same listing with the filter dropped and the rest of the
// address kept.
func ordersEmpty(q OrdersQuery) h.Node {
	return ui.Empty(ui.EmptyOpts{
		Icon:   "funnel",
		Title:  "No order matches this filter",
		Hint:   "Try another state, or clear the search.",
		Action: ui.ButtonLink(q.Href("q", "", "state", "", "page", ""), ui.Outline(), h.Text("Clear the filter")),
	})
}
```

`q.Href` keeps the rest of the address, so the way out drops the filter and the search and
nothing else. `ui.DataTable` draws the distinction on its own, in English; `ListState.Empty` is
how an application says it in its own words. See
[the demo](/learn/ui-kit#empty-and-filtered-down-to-empty).

## The slow part, after the page

The chart groups a month of orders by channel and takes two seconds. Nothing else on the screen
should wait for it, and nobody should be looking at a spinner where the whole page used to be:

```go
// OrdersInsights is the route the placeholder asks for once the page has
// loaded. It is an ordinary page: with no fragment header it answers the whole
// thing, which is where the placeholder's <noscript> link goes — without
// JavaScript the slow part is one click away instead of missing.
func OrdersInsights(c *trilha.Ctx) (h.Node, error) {
	data, err := orders.Insights(c.Context())
	if err != nil {
		return nil, err
	}
	block := h.Div(h.ID("insights"), ui.Bars(data, ui.ChartTitle("Orders by channel")))
	if c.Fragment() == "insights" {
		return block, nil
	}
	return h.Div(ui.PageHeader("Insights"), block), nil
}
```

`ui.Defer` renders a placeholder of a known height — the page must not jump when the content
lands — and asks for the fragment once the page has loaded. The route is an ordinary page: with
no fragment header it answers the whole thing, which is where the placeholder's `<noscript>`
link goes, so without JavaScript the slow part is one click away instead of missing.

One `Defer` per part of a screen, never one per row of a list: a page that defers twenty
fragments made twenty requests to render itself, which is the SPA it was avoiding. See
[Live fragments](/reference/live) and [the demo](/learn/ui-kit#the-slow-part-a-moment-later).

## The cell that moves on its own

An order is picked and shipped by people who are not looking at this screen. The state cell
watches for it:

```go
// orderStateCell is the one cell that moves without anybody clicking. ui.On
// waits for an event named after the order and asks this same fragment's route
// again — the event carries the name, never the HTML, so the authorization and
// the rendering stay where they already are.
func orderStateCell(o Order) h.Node {
	return h.Div(h.ID("order-"+o.ID+"-state"), ui.On("order:"+o.ID, "/orders/"+o.ID+"/state"),
		ui.Status(OrderStates, o.State))
}

// OrderStatePage is what the event makes the browser ask for. A value the enum
// no longer knows renders muted instead of taking the screen down.
func OrderStatePage(c *trilha.Ctx) (h.Node, error) {
	o, err := orders.Find(c.Param("id"))
	if err != nil {
		return nil, err
	}
	return orderStateCell(o), nil
}
```

`ui.On` waits for an event named after the order and asks the cell's own route again. **The
event carries the name, never the HTML**: the authorization and the rendering stay where they
already were, and the stream never becomes a channel for data. `ui.Poll("30s", src)` is the
same idea on a clock, for a screen with no bus behind it.

The cell is the same function in the table and on the detail page, because a fragment does not
care which screen it is on. See [Live fragments](/reference/live) and
[the demo](/learn/ui-kit#a-cell-that-refreshes-itself).

## The row, opened

```go
// OrderPage is what a row opens: the metadata on one side, the invoice on the
// other, and the same live cell the table had — the fragment does not care
// which screen it is on.
func OrderPage(c *trilha.Ctx) (h.Node, error) {
	o, err := orders.Find(c.Param("id"))
	if err != nil {
		return nil, err
	}
	c.SetTitle("Order " + o.ID)
	return h.Div(
		ui.PageHeader("Order "+o.ID, ui.ButtonLink("/orders", ui.Outline(), ui.Icon("arrow-left"), h.Text("Back"))),
		ui.Grid(
			ui.Card(ui.Stack(
				ui.Stat("Customer", o.Customer),
				ui.Stat("Total", dollars(o.Total)),
				orderStateCell(o),
			)),
			orderInvoice(c, o),
		),
	), nil
}
```

The invoice is a file, and a file has three cases: one a browser draws, one it renders inline,
and one it cannot show at all. `ui.Preview` picks; the third case is a card with a download
button instead of a frame that renders blank. The fourth case is the file that does not exist
yet, and that is `ui.Empty` again — the same component, a different sentence:

```go
// orderInvoice is the file beside its metadata. ui.Preview decides how to show
// it from the type — an image as an <img>, a PDF in a frame, anything a
// browser cannot render as a card with a download button instead of a frame
// that renders blank.
func orderInvoice(c *trilha.Ctx, o Order) h.Node {
	if o.Invoice == "" {
		return ui.Empty(ui.EmptyOpts{Icon: "download", Title: "No invoice yet",
			Hint: "It is issued when the order ships."})
	}
	return ui.Preview(c, o.Invoice, ui.PreviewOpts{Title: "Invoice " + o.ID, Height: "24rem"})
}
```

See [the demo](/learn/ui-kit#a-file-next-to-its-metadata) and [Uploads](/cookbook/uploads) for
where `o.Invoice` comes from.

## The form that takes three screens

`ui.Steps` draws where somebody is: what is behind them links back, what is ahead is plain
text, and the current step carries `aria-current="step"`.

```go
// orderSteps is the new-order form, which is three screens. A step ahead of
// the current one never links, however hard somebody stares at it: a wizard
// whose third step is one click away is a wizard whose steps did not have to
// happen in order.
var orderSteps = []ui.Step{
	{Label: "Customer", Href: "/orders/new"},
	{Label: "Items", Href: "/orders/new/items"},
	{Label: "Confirm"},
}
```

What travels between the screens is a draft, one struct per step, so that each screen validates
its own fields and no message ever points at a field two screens back:

```go
// OrderDraft is what travels between the three screens, one struct per step so
// that each screen validates its own fields and no message ever points at a
// field two screens back.
type OrderDraft struct {
	Who   OrderWhoStep   `json:"who"`
	Items OrderItemsStep `json:"items"`
}

// OrderWhoStep is the first screen.
type OrderWhoStep struct {
	Customer string `form:"customer" validate:"required,max=80"`
	Email    string `form:"email"    validate:"required,email"`
}

// OrderItemsStep is the second.
type OrderItemsStep struct {
	SKU string `form:"sku" validate:"required"`
	Qty int    `form:"qty" validate:"required,min=1"`
}
```

```go
// OrderItemsPage is every screen after the first. No draft is not a failure:
// it is somebody whose draft expired or who typed the address of step two, and
// the answer is step one — not an empty form that would lose what they fill in
// here.
func OrderItemsPage(c *trilha.Ctx) (h.Node, error) {
	var d OrderDraft
	if err := c.Draft("new-order").Load(&d); err != nil {
		return nil, c.Redirect("/orders/new")
	}
	return h.Div(
		ui.PageHeader("New order"),
		ui.Steps(orderSteps, 2),
		orderItemsForm(c, d.Items),
	), nil
}
```

```go
// OrderItemsPOST is the end of a step: load what is there, bind only this
// step, save, move on. A 422 here costs nothing that was typed on an earlier
// screen, because the earlier screens are in the draft and not in this form.
func OrderItemsPOST(c *trilha.Ctx) error {
	var d OrderDraft
	_ = c.Draft("new-order").Load(&d)
	if err := c.Bind(&d.Items); err != nil {
		return err // FieldErrors: 422 with the messages next to the fields
	}
	if err := c.Draft("new-order").Save(d, 30*time.Minute); err != nil {
		return err
	}
	return c.Redirect("/orders/new/confirm")
}
```

`c.Draft` is a signed cookie or a store, depending on `Config.Drafts`; either way a refresh, a
422 or a phone call in the middle costs nothing that was typed. No draft is not a failure — it
is an expired draft or a pasted address, and the answer is step one. The form itself posts to
its own route with `trilha.CSRFInput(c)` in it. See [A form in steps](/cookbook/wizard) and
[the demo](/learn/ui-kit#a-form-in-several-screens).

## What the screen cost

No client router, no build step, no state duplicated between server and browser, and no
JavaScript that the screen stops working without: ordering, filtering, paging, the wizard and
the detail page all work with `ui.js` off, and what the script adds is that they do not reload.
The whole thing is one folder of handlers and about two hundred lines.

What did not fit here, and fits the same way when the screen needs it:
[`ui.Tree`](/learn/ui-kit#a-hierarchy-that-opens-node-by-node) for a filter that is a hierarchy,
[`ui.Combobox`](/learn/ui-kit#a-field-that-searches-as-you-type) for a field with thousands of
options, [`ui.SearchBox`](/learn/ui-kit#one-box-several-kinds-of-thing) for the search in the
top bar, and [`ui.EmptyError`](/learn/ui-kit#nothing-to-show-and-what-could-not-load) for the
part of the screen that failed to load — the error itself only in development.

## Challenge

The people who run this console want to see, at a glance, which orders are late: not shipped
and untouched for more than two days. Add the column without touching the query, the URL or the
empty state.

:::solution
```go
// ordersLateColumn is the answer to the chapter's challenge: one more column,
// and nothing else. The row already knows whether it is late, and the cell is
// the same badge the state uses — a second vocabulary of colours on one screen
// is how two tables end up with two different reds.
var ordersLateColumn = ui.Column[Order]{Key: "late", Label: "Late", Cell: func(o Order) h.Node {
	if o.State == "shipped" || time.Since(o.Updated) < 48*time.Hour {
		return ui.Muted(h.Text("—"))
	}
	return ui.Badge(ui.Destructive(), h.Text("late"))
}}
```

One entry in the columns of `ordersColumns`, and nothing else moves: the row already carries
what the answer needs, so no filter, no parameter and no second query appear. The badge is the
one the state cell uses, because a second vocabulary of colours on one screen is how two tables
end up with two different reds.

Note what the column is *not*: `Sort: true`. Sorting by it would put `late` in the URL, and
`Restrict` would drop it — the database has no such column, and the screen would be lying about
what it can order by.
:::
