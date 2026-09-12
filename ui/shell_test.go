package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

var menu = []NavGroup{
	{Label: "Documents", Items: []NavItem{
		{Href: "/docs", Label: "Documents", Icon: "search"},
		{Href: "/docs/new", Label: "New", Icon: "plus", Badge: "3"},
	}},
	{Label: "Admin", Hide: true, Items: []NavItem{{Href: "/users", Label: "Users"}}},
}

// SC-001 — the longest declared prefix wins, so /docs and /docs/new in the
// same menu light up one item and never both.
func TestShellMarksTheLongestPrefix(t *testing.T) {
	cases := []struct{ path, want string }{
		{"/docs", "/docs"},
		{"/docs/new", "/docs/new"},
		{"/docs/41", "/docs"},
		{"/docs/41/edit", "/docs"},
		{"/other", ""},
	}
	for _, c := range cases {
		if got := activeHref(menu, c.path); got != c.want {
			t.Errorf("%s → %q, want %q", c.path, got, c.want)
		}
		got := render(t, Shell(nil, ShellOpts{Nav: menu, Current: c.path}))
		if n := strings.Count(got, `aria-current="page"`); n > 1 {
			t.Errorf("%s marked %d items", c.path, n)
		}
		if c.want != "" && !strings.Contains(got, `href="`+c.want+`" aria-current="page"`) {
			t.Errorf("%s did not mark %s: %s", c.path, c.want, got)
		}
	}
}

// #168 — a NavItem for a name outside the kit's 31 icons draws its own node
// instead of panicking in Icon; IconNode wins even when Icon is also set, and
// whatever the app hands in renders through the same h escaping as any other
// node, not as raw markup.
func TestNavItemIconNodeWinsOverIcon(t *testing.T) {
	custom := h.Svg(h.Class("ui-icon"), h.Attr("viewBox", "0 0 24 24"),
		h.El("path", h.Attr("d", "M4 3h10l6 6v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z")),
		h.Text("<script>"))
	items := []NavGroup{{Items: []NavItem{
		{Href: "/docs", Label: "Documents", Icon: "house", IconNode: custom},
	}}}
	got := render(t, Shell(nil, ShellOpts{Nav: items}))
	has(t, got, `<path d="M4 3h10l6 6v12a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1Z"></path>`, "&lt;script&gt;")
	if strings.Contains(got, "<script>") {
		t.Fatalf("text inside the custom node was not escaped: %s", got)
	}
	if strings.Contains(got, "M15 21v-8") {
		t.Fatalf("Icon still drew the built-in house icon over IconNode: %s", got)
	}
}

// SC-002 — hiding is cosmetics, but what is hidden is not in the HTML either.
func TestShellLeavesOutWhatIsHidden(t *testing.T) {
	got := render(t, Shell(nil, ShellOpts{Nav: menu, Current: "/docs"}))
	if strings.Contains(got, "/users") || strings.Contains(got, "Admin") {
		t.Error(got)
	}
	one := []NavGroup{{Label: "G", Items: []NavItem{{Href: "/a", Label: "A", Hide: true}, {Href: "/b", Label: "B"}}}}
	got = render(t, Shell(nil, ShellOpts{Nav: one, Current: "/b"}))
	if strings.Contains(got, `href="/a"`) || !strings.Contains(got, `href="/b"`) {
		t.Error(got)
	}
	// A group whose every item is hidden takes its own title with it.
	none := []NavGroup{{Label: "Empty", Items: []NavItem{{Href: "/a", Label: "A", Hide: true}}}}
	if got := render(t, Shell(nil, ShellOpts{Nav: none})); strings.Contains(got, "Empty") {
		t.Error(got)
	}
}

// SC-003 — the collapse is a button that says what it did, over a class the
// page already had when it painted.
func TestShellCollapseIsAnnounced(t *testing.T) {
	got := render(t, Shell(nil, ShellOpts{Nav: menu, Current: "/docs"}))
	if !strings.Contains(got, `data-ui-sidebar-toggle=""`) || !strings.Contains(got, `aria-controls="ui-shell-nav"`) {
		t.Error(got)
	}
	if !strings.Contains(got, `aria-expanded="true"`) || !strings.Contains(got, `id="ui-shell-nav"`) {
		t.Error(got)
	}
	if !strings.Contains(themeInit, "ui-sidebar-collapsed") {
		t.Error("the class has to be on <html> before the first paint")
	}
	js := string(Asset("ui.js"))
	if !strings.Contains(js, "data-ui-sidebar-toggle") || !strings.Contains(js, `localStorage.setItem("ui-sidebar"`) {
		t.Error("ui.js does not remember the choice")
	}
}

// SC-005 — the corner of the top bar, and the page in the middle.
func TestShellPutsTheUserAndThePage(t *testing.T) {
	got := render(t, Shell(nil, ShellOpts{
		Brand:  Brand("/", "Acervo"),
		Nav:    menu,
		User:   UserMenu{Name: "Ana", Detail: "ana@example.com", Items: []h.Node{MenuLink("/profile", h.Text("Profile"))}},
		Header: []h.Node{ThemeToggle()},
	}, h.P(h.Text("the page"))))
	for _, want := range []string{`popovertarget="ui-user-menu"`, `id="ui-user-menu"`, ">Ana<", "ana@example.com", `href="/profile"`,
		`data-ui-theme-toggle`, `<main class="ui-shell-content"><p>the page</p></main>`, `class="ui-badge">3<`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

// The path of the request is the default, which is what a layout wants.
func TestShellReadsThePathOfTheRequest(t *testing.T) {
	a := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var got string
	a.Register(trilha.Route{Pattern: "/docs/{id}", Page: func(c *trilha.Ctx) (h.Node, error) {
		got = render(t, Shell(c, ShellOpts{Nav: menu}))
		return h.P(), nil
	}})
	a.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/docs/41", nil))
	if !strings.Contains(got, `href="/docs" aria-current="page"`) {
		t.Error(got)
	}
}

// SC-004 — the way back above the title, the actions to the right.
func TestPageHeaderPlacesTheWayBack(t *testing.T) {
	got := render(t, PageHeader("Contract 41", Back{Href: "/docs", Label: "Documents"}, ButtonLink("/docs/41/edit", h.Text("Edit"))))
	back := strings.Index(got, `class="ui-back"`)
	title := strings.Index(got, "Contract 41")
	actions := strings.Index(got, `class="ui-page-actions"`)
	if back < 0 || title < 0 || actions < 0 || !(back < title && title < actions) {
		t.Fatal(got)
	}
	if !strings.Contains(got, `href="/docs"`) || !strings.Contains(got, ">Documents<") {
		t.Error(got)
	}
	// Without a Back there is no empty link, and a stray one renders nothing.
	if s := render(t, PageHeader("Users")); strings.Contains(s, "ui-back") {
		t.Error(s)
	}
	if s := render(t, h.Div(Back{Href: "/x", Label: "x"})); s != "<div></div>" {
		t.Error(s)
	}
}
