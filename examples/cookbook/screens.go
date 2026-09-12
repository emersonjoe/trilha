package cookbook

import (
	"context"
	"strconv"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// The declarations below are one screen of an internal app, written from the
// frame inwards: the shell around every page, the numbers on top, the table
// that lives in the URL, the part that arrives late, the file beside a row and
// the form that takes three screens. None of them is a new component — every
// one is a call to something the kit already has, in the order a screen is
// actually built.

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

// SetupOrders registers the enum under the name the validation tag cites.
func SetupOrders(a *trilha.App) error {
	trilha.RegisterEnum("ops.OrderState", OrderStates)
	return nil
}

// OrdersSummary is the top of the screen in one struct: the four numbers and
// the fourteen days behind the one that has a shape.
type OrdersSummary struct {
	Open, Late, ShippedToday int
	Revenue                  float64
	LateByDay                []float64
}

// orderRepo is the half of the screen this chapter does not talk about. What
// a listing asks a database is always these four questions, and a screen that
// asks them through an interface is a screen a test can render without a
// database behind it.
type orderRepo interface {
	Search(q OrdersQuery) ([]Order, int, error)
	Find(id string) (Order, error)
	Summary() (OrdersSummary, error)
	Insights(ctx context.Context) ([]ui.Datum, error)
}

// orders is whatever implements it: a package over sql.DB in production, a
// slice in a test.
var orders orderRepo

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

// ordersAdmin is the question the menu asks, and the same one the middleware
// at the root of /settings asks again — the menu hides, the middleware
// refuses, and only one of the two is a permission.
func ordersAdmin(c *trilha.Ctx) bool {
	role, _ := c.Get("role").(string)
	return role == "admin"
}

// OrdersQuery is the state of the screen, and all of it is in the address:
// ListParams brings page, order and search, and the fields beside it are the
// filters this screen added. c.Bind fills the whole thing, applies the limits
// of ListParams and keeps the rest of the query so the links preserve it.
type OrdersQuery struct {
	trilha.ListParams
	State string `form:"state" validate:"omitempty,enum=ops.OrderState"`
}

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

// ordersFilters is the rest of the filter form — the search box is the kit's.
// The select's options come from the enum, so the filter cannot offer a state
// the validation would refuse.
func ordersFilters(q OrdersQuery) h.Node {
	return ui.Field("state", "State", ui.Select(h.ID("state"), h.Name("state"),
		OrderStates.Options(q.State, "Any state")))
}

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

// orderSteps is the new-order form, which is three screens. A step ahead of
// the current one never links, however hard somebody stares at it: a wizard
// whose third step is one click away is a wizard whose steps did not have to
// happen in order.
var orderSteps = []ui.Step{
	{Label: "Customer", Href: "/orders/new"},
	{Label: "Items", Href: "/orders/new/items"},
	{Label: "Confirm"},
}

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

// orderItemsForm is the form of the second step. It posts to itself, and the
// CSRF token is a field of the form and not a decision the handler makes.
func orderItemsForm(c *trilha.Ctx, s OrderItemsStep) h.Node {
	return h.Form(h.Method("post"), h.Action("/orders/new/items"), h.Class("ui-stack"),
		trilha.CSRFInput(c),
		ui.Field("sku", "SKU", ui.Input(h.ID("sku"), h.Name("sku"), h.Value(s.SKU))),
		ui.Field("qty", "Quantity", ui.Input(h.ID("qty"), h.Name("qty"), h.Type("number"),
			h.Value(strconv.Itoa(s.Qty)))),
		ui.Submit(h.Text("Continue")),
	)
}

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
