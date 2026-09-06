---
title: Shell
description: ui.Shell is the frame of an internal app — sidebar, header, user menu — and ui.PageHeader is the title of the screen inside it.
---

Every management app has the same frame: a sidebar with the sections, a header with the
user, and the screen in the middle. `ui.Shell` is that frame written once. It is a
composition of components that already existed — `ui.Sidebar`, `ui.Nav`, `ui.Menu`,
`ui.ThemeToggle` — not a new primitive, so anything it does not cover you write beside
it with the same pieces.

## Shell

```go
func Layout(c *trilha.Ctx, children ...h.Node) h.Node {
	u := session.User(c)
	return ui.Shell(c, ui.ShellOpts{
		Brand: h.A(h.Class("ui-brand"), h.Href("/"), h.Text("Acme")),
		Nav: []ui.NavGroup{
			{Label: "Work", Items: []ui.NavItem{
				{Href: "/", Label: "Dashboard", Icon: "house"},
				{Href: "/items", Label: "Items", Icon: "search", Badge: "12"},
			}},
			{Label: "Admin", Hide: u.Role != "admin", Items: []ui.NavItem{
				{Href: "/users", Label: "Users", Icon: "user"},
			}},
		},
		User:   ui.UserMenu{Name: u.Name, Detail: u.Email, Items: []h.Node{logoutForm(c)}},
		Header: []h.Node{ui.ThemeToggle()},
	}, children...)
}
```

`ShellOpts`:

| Field | What it is |
|---|---|
| `Brand` | what sits at the top of the sidebar; usually a link to `/` |
| `Nav` | the groups of the menu, in the order they are shown |
| `User` | the name, the detail line and the items of the menu at the top right |
| `Header` | extra nodes for the header, to the left of the user menu |
| `Current` | the path to mark as active; the request's path when empty |

A `NavItem` is `{Href, Label, Icon, Badge, Hide}`. `Icon` is a name from `ui.Icon`;
`Badge` is a short count beside the label.

## The active item is the longest prefix

The item marked with `aria-current="page"` is the one whose `Href` is the longest prefix
of the current path, so `/items/42/edit` lights up `/items` and not `/`. An exact match
always wins, and `/` only matches `/` — otherwise the dashboard would be active on every
screen.

## Hiding is cosmetics

`Hide: true` leaves the item (or the whole group) out of the HTML. That is a courtesy to
whoever is reading the menu, **not** a permission. What actually keeps somebody out is a
middleware at the root of the folder:

```go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return session.Flow.RequireRole("admin")(c, next)
}
```

A link that is hidden and a route that is not protected is a route anybody can type.

## Collapsing brings no new asset

The toggle in the header stamps `ui-sidebar-collapsed` on `<html>` and remembers the
choice in `localStorage`. The inline script `ui.Head` already emits reads it back before
the first paint, so the sidebar does not jump open and shut on every navigation; on a
narrow screen the same class starts on, and the sidebar becomes a drawer over the
content. The code lives in `ui.js`, which the shell already needs — there is nothing new
to download.

## PageHeader

Inside the shell, each screen opens with its own title and its buttons:

```go
ui.PageHeader("Items",
	ui.Back{Href: "/", Label: "Dashboard"},
	ui.ButtonLink("/items/new", ui.Icon("plus"), h.Text("New item")),
)
```

`ui.Back` is read by `PageHeader` and rendered as the way back, above the title; every
other child goes to the right of it, which is where the actions of the screen belong.

## Starting from here

`trilha new my-app --template app` writes a project that already has all of this: the
shell, a login, a dashboard with charts, a listing and a form. See
[the CLI](/reference/cli).
