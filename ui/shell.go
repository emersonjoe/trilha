package ui

import (
	"io"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// NavItem is one link of the sidebar. Icon is a name from Icons(); Badge is a
// count or a word beside the label. Hide takes the entry out of the HTML —
// and hiding a link is cosmetics: what keeps somebody out of the route is the
// middleware.go of that branch, never the menu.
//
// IconNode, when set, wins over Icon: it is the way out for a name the kit
// does not carry, such as a document or a building. The app draws its own
// node — typically h.Svg(h.Class("ui-icon"), …), so the kit keeps owning the
// size and the alignment — and Icon stays the shortcut for the common case.
type NavItem struct {
	Href     string
	Label    string
	Icon     string
	Badge    string
	IconNode h.Node
	Hide     bool
}

// NavGroup is a titled block of the sidebar. Hiding the group hides its items.
type NavGroup struct {
	Label string
	Items []NavItem
	Hide  bool
}

// UserMenu is the corner of the top bar: who is logged in and what they can do
// about it. Items are Menu children — MenuLink, MenuItem, a logout form.
type UserMenu struct {
	Name   string
	Detail string
	Items  []h.Node
}

// ShellOpts is what changes from app to app in the frame of a management app.
type ShellOpts struct {
	Brand  h.Node // Brand("/", "Acervo"), or anything else
	Nav    []NavGroup
	User   UserMenu
	Header []h.Node // extra top-bar nodes: a search box, ThemeToggle
	// Current is the path the active item is worked out from. Empty is the
	// path of the request, which is what an app wants; a fragment answering
	// for another screen is what it is for.
	Current string
}

// Shell is the frame every management app writes in its first week: sidebar
// with groups, active item, top bar with the user's menu, and the page in the
// middle. It is a composition of the kit's pieces — Sidebar, Nav, NavLink,
// Header, Brand, Menu — which stay exported for whoever wants another
// arrangement.
//
//	func Layout(c *trilha.Ctx, children h.Node) (h.Node, error) {
//		u := sessions.User(c)
//		return ui.Shell(c, ui.ShellOpts{
//			Brand: ui.Brand("/", "Acervo"),
//			Nav: []ui.NavGroup{{Label: "Documents", Items: []ui.NavItem{
//				{Href: "/docs", Label: "Documents", Icon: "file"},
//			}}},
//			User: ui.UserMenu{Name: u.Name, Items: []h.Node{ui.MenuLink("/profile", h.Text("Profile"))}},
//		}, children), nil
//	}
//
// The collapse is a class on <html>, applied by the same inline script of the
// theme before the first paint, so the sidebar does not open and close in front
// of whoever asked for it closed. On a narrow screen the same class is what
// slides the drawer.
func Shell(c *trilha.Ctx, o ShellOpts, children ...h.Node) h.Node {
	path := o.Current
	if path == "" && c != nil {
		path = c.Request().URL.Path
	}
	active := activeHref(o.Nav, path)

	side := []h.Node{h.ID(shellNavID)}
	if o.Brand != nil {
		side = append(side, h.Div(h.Class("ui-shell-brand"), o.Brand))
	}
	for _, g := range o.Nav {
		if g.Hide {
			continue
		}
		links := make([]h.Node, 0, len(g.Items))
		for _, it := range g.Items {
			if it.Hide {
				continue
			}
			n := []h.Node{h.Href(it.Href)}
			if it.Href == active {
				n = append(n, h.Aria("current", "page"))
			}
			if it.IconNode != nil {
				n = append(n, it.IconNode)
			} else if it.Icon != "" {
				n = append(n, Icon(it.Icon))
			}
			n = append(n, h.Span(h.Class("ui-nav-label"), h.Text(it.Label)))
			if it.Badge != "" {
				n = append(n, Badge(h.Text(it.Badge)))
			}
			links = append(links, h.A(n...))
		}
		if len(links) == 0 {
			continue
		}
		if g.Label != "" {
			side = append(side, h.Div(h.Class("ui-nav-group"), h.Text(g.Label)))
		}
		side = append(side, Nav(links...))
	}

	top := []h.Node{
		Button(Ghost(), IconSize(), h.Data("ui-sidebar-toggle", ""), h.Aria("controls", shellNavID),
			h.Aria("expanded", "true"), h.Aria("label", "Menu"), Icon("menu")),
	}
	if o.Brand != nil {
		top = append(top, h.Div(h.Class("ui-shell-brand-sm"), o.Brand))
	}
	top = append(top, Spacer())
	top = append(top, o.Header...)
	if o.User.Name != "" || len(o.User.Items) > 0 {
		top = append(top, userMenu(o.User))
	}

	return h.Div(h.Class("ui-shell"),
		Sidebar(side...),
		h.Div(h.Class("ui-shell-main"),
			Header(top...),
			h.Main(append([]h.Node{h.Class("ui-shell-content")}, children...)...),
		),
	)
}

const shellNavID = "ui-shell-nav"

func userMenu(u UserMenu) h.Node {
	const id = "ui-user-menu"
	label := u.Name
	if label == "" {
		label = "…"
	}
	items := u.Items
	if u.Detail != "" {
		items = append([]h.Node{h.Div(h.Class("ui-menu-label"), h.Text(u.Detail)), Separator()}, items...)
	}
	return h.Div(h.Class("ui-shell-user"),
		MenuTrigger(id, Ghost(), Sm(), Icon("user"), h.Span(h.Class("ui-nav-label"), h.Text(label))),
		Menu(id, items...),
	)
}

// activeHref picks the longest declared prefix that matches the path, so
// /docs and /docs/new in the same menu light up one item, not two. An exact
// match always wins, and "/" only matches the root.
func activeHref(groups []NavGroup, path string) string {
	best := ""
	for _, g := range groups {
		if g.Hide {
			continue
		}
		for _, it := range g.Items {
			if it.Hide || it.Href == "" {
				continue
			}
			switch {
			case it.Href == path:
				return it.Href
			case it.Href == "/":
				continue
			case strings.HasPrefix(path, strings.TrimSuffix(it.Href, "/")+"/"):
				if len(it.Href) > len(best) {
					best = it.Href
				}
			}
		}
	}
	return best
}

// Back is the "up one level" link of a PageHeader. It is written among the
// header's children and PageHeader puts it where it belongs, above the title.
type Back struct {
	Href  string
	Label string
}

// Render draws nothing: Back is a marker PageHeader reads. A stray one
// elsewhere is silent instead of being a second link in the wrong place.
func (Back) Render(io.Writer) error { return nil }

// PageHeader is the top of a screen: the way back, the title and the actions
// on the right.
//
//	ui.PageHeader("Contract 41", ui.Back{Href: "/docs", Label: "Documents"},
//		ui.ButtonLink("/docs/41/edit", h.Text("Edit")))
func PageHeader(title string, children ...h.Node) h.Node {
	var back *Back
	actions := make([]h.Node, 0, len(children))
	for _, ch := range children {
		if b, ok := ch.(Back); ok {
			b := b
			back = &b
			continue
		}
		actions = append(actions, ch)
	}
	n := []h.Node{h.Class("ui-page-header")}
	if back != nil {
		label := back.Label
		if label == "" {
			label = "Back"
		}
		n = append(n, h.A(h.Class("ui-back"), h.Href(back.Href), Icon("chevron-left"), h.Text(label)))
	}
	row := []h.Node{h.Class("ui-page-header-row"), H1(h.Text(title))}
	if len(actions) > 0 {
		row = append(row, h.Div(append([]h.Node{h.Class("ui-page-actions")}, actions...)...))
	}
	return h.Div(append(n, h.Div(row...))...)
}
