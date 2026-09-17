package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// cssRule returns the declarations of the first rule whose selector list is
// exactly sel, or "" when there is none. ui.css is one rule per line, so a
// line is a rule.
func cssRule(t *testing.T, sel string) string {
	t.Helper()
	for _, line := range strings.Split(string(Asset("ui.css")), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, sel+" {") || strings.HasPrefix(line, sel+"{") {
			return line[strings.Index(line, "{")+1:]
		}
	}
	return ""
}

// #255 — a sidebar with display:none is not a grid item, so the page is what
// lands on the 0 track: collapsed is one track, not a zero and a rest.
func TestCollapsedShellKeepsThePageWide(t *testing.T) {
	rule := cssRule(t, "html.ui-sidebar-collapsed .ui-shell")
	if !strings.Contains(rule, "grid-template-columns: 1fr") || strings.Contains(rule, "0 1fr") {
		t.Fatalf("collapsed shell is still two tracks: %q", rule)
	}
}

// #256 — the desktop preference and the phone drawer are two flags: the
// drawer is never persisted, closes on navigation, and can be closed from
// inside the drawer itself.
func TestPhoneDrawerIsNotTheDesktopPreference(t *testing.T) {
	css := string(Asset("ui.css"))
	js := string(Asset("ui.js"))
	if !strings.Contains(css, "html.ui-drawer-open") {
		t.Fatal("ui.css has no drawer state of its own")
	}
	if !strings.Contains(js, "ui-drawer-open") || !strings.Contains(js, "(max-width: 767px)") {
		t.Fatal("ui.js does not tell the drawer from the preference")
	}
	if strings.Contains(themeInit, "max-width") {
		t.Fatal("the inline script still turns the collapse on below 768px; that is the stylesheet's job now")
	}
	got := render(t, Shell(nil, ShellOpts{Nav: menu, Current: "/docs"}))
	if strings.Count(got, `data-ui-sidebar-toggle=""`) != 2 || !strings.Contains(got, ` ui-shell-close"`) {
		t.Fatalf("the drawer has no close button of its own:\n%s", got)
	}
}

// #257 — an alert with no icon is one column: the title above the description,
// never beside it.
func TestAlertWithoutIconIsOneColumn(t *testing.T) {
	rule := cssRule(t, ".ui-alert:not(:has(> .ui-icon))")
	if !strings.Contains(rule, "grid-template-columns") {
		t.Fatal("no one-column rule for an alert without an icon")
	}
	if !strings.Contains(cssRule(t, ".ui-alert:not(:has(> .ui-icon)) .ui-alert-title, .ui-alert:not(:has(> .ui-icon)) .ui-alert-description"), "grid-column: 1") {
		t.Fatal("the description is still pinned to column 2")
	}
}

// #260 — an action put in the card header sits to the right of the title,
// its own size, instead of stretching across the card.
func TestCardHeaderMakesRoomForAnAction(t *testing.T) {
	css := string(Asset("ui.css"))
	if !strings.Contains(css, ".ui-card-header:has(> .ui-btn)") || !strings.Contains(css, "justify-self: end") {
		t.Fatal("a button in the card header still stretches")
	}
}

// #261 — the tabs strip scrolls sideways rather than cutting the last tabs off.
func TestTabsListScrollsInsteadOfClipping(t *testing.T) {
	rule := cssRule(t, ".ui-tabs-list")
	if !strings.Contains(rule, "overflow-x: auto") || !strings.Contains(rule, "max-width: 100%") {
		t.Fatalf("tabs still clip: %q", rule)
	}
}

// #262 — the subtitle is a marker like Back: it lands under the title row,
// and never among the actions.
func TestPageHeaderPlacesTheSubtitle(t *testing.T) {
	got := render(t, PageHeader("Documents", Subtitle("Everything ingested, and how it went."),
		ButtonLink("/upload", h.Text("Upload"))))
	title := strings.Index(got, "Documents")
	sub := strings.Index(got, `class="ui-page-subtitle"`)
	actions := strings.Index(got, `class="ui-page-actions"`)
	if title < 0 || sub < 0 || actions < 0 || !(title < actions && actions < sub) {
		t.Fatal(got)
	}
	if strings.Contains(got[actions:sub], "Everything ingested") {
		t.Fatalf("the subtitle is among the actions:\n%s", got)
	}
	if s := render(t, PageHeader("Users")); strings.Contains(s, "ui-page-subtitle") {
		t.Error(s)
	}
	if s := render(t, h.Div(Subtitle("x"))); s != "<div></div>" {
		t.Error(s)
	}
	if cssRule(t, ".ui-page-subtitle") == "" {
		t.Error("ui.css does not draw the subtitle")
	}
}

// #263 — the caller says how many columns, per band, and the stylesheet reads it.
func TestGridColsAreTheCallersToSay(t *testing.T) {
	// Classes and not a style attribute: a kit that writes style="" is a kit
	// that needs 'unsafe-inline' in style-src, which #252 is about removing.
	got := render(t, Grid(Cols(2, 4), h.Div()))
	if !strings.Contains(got, `class="ui-grid ui-grid-2 ui-grid-lg-4"`) || strings.Contains(got, "style=") {
		t.Fatal(got)
	}
	if got := render(t, Grid(Cols(3), h.Div())); !strings.Contains(got, `class="ui-grid ui-grid-3"`) {
		t.Fatal(got)
	}
	if got := render(t, Grid(h.Div())); strings.Contains(got, "ui-grid-") {
		t.Fatal(got)
	}
	// Out of the drawn range is clamped, not dropped: nine columns is six.
	if got := render(t, Grid(Cols(0, 9))); !strings.Contains(got, `class="ui-grid ui-grid-1 ui-grid-lg-6"`) {
		t.Fatal(got)
	}
	// A class of the caller's beside Cols still travels.
	if got := render(t, Grid(Cols(2), h.Class("mine"))); !strings.Contains(got, `class="ui-grid ui-grid-2 mine"`) {
		t.Fatal(got)
	}
	css := string(Asset("ui.css"))
	for _, want := range []string{".ui-grid-1", ".ui-grid-6", ".ui-grid-lg-1", ".ui-grid-lg-6", "@media (min-width: 1024px)"} {
		if !strings.Contains(css, want) {
			t.Fatalf("ui.css does not draw %s", want)
		}
	}
}

// #264 — the box can carry a send button, for the form that has to work
// without Enter; and the shortcut hint does not fold in two.
func TestSearchBoxSubmitButton(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return SearchBox(c, "/search", SearchBoxOpts{Submit: "Search"})
	})
	if !strings.Contains(got, `type="submit"`) || !strings.Contains(got, ">Search</button>") {
		t.Fatalf("no send button:\n%s", got)
	}
	if kbd, btn := strings.Index(got, "ui-kbd"), strings.Index(got, `type="submit"`); kbd > btn {
		t.Fatalf("the button comes before the hint:\n%s", got)
	}
	if got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return SearchBox(c, "/search", SearchBoxOpts{})
	}); strings.Contains(got, `type="submit"`) {
		t.Fatalf("a button nobody asked for:\n%s", got)
	}
	if rule := cssRule(t, ".ui-search-box .ui-kbd"); !strings.Contains(rule, "flex: none") {
		t.Fatalf("the hint still shrinks: %q", rule)
	}
}

// #258 — Portuguese months and years take their plural.
func TestRelativeTextPluralizesInPortuguese(t *testing.T) {
	cases := map[time.Duration]string{
		8 * 30 * 24 * time.Hour:   "há 8 meses",
		1 * 30 * 24 * time.Hour:   "há 1 mês",
		-2 * 365 * 24 * time.Hour: "em 2 anos",
		-1 * 365 * 24 * time.Hour: "em 1 ano",
		3 * 24 * time.Hour:        "há 3 d",
	}
	for d, want := range cases {
		if got := relativeText("pt-BR", d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
	if got := relativeText("en", 8*30*24*time.Hour); got != "8mo ago" {
		t.Errorf("en changed: %q", got)
	}
}

// hookClasses are the classes the kit writes on purpose without a rule of its
// own: a name for the application's stylesheet to hang on. They are listed in
// the reference (ui.md, "Hooks"); a class here that is not there is a
// promise the reader cannot find.
var hookClasses = map[string]bool{
	"ui-assistant":           true, // the assistant panel's shell
	"ui-assistant-label":     true, // the launcher's text beside its icon
	"ui-audio":               true, // the player's frame; the player and the duration are drawn
	"ui-btn-primary":         true, // .ui-btn is already the primary button
	"ui-connections":         true, // the whole ConnectionsPanel
	"ui-connections-actions": true,
	"ui-connections-buttons": true,
	"ui-connections-form":    true,
	"ui-connections-group":   true,
	"ui-connections-kind":    true,
	"ui-connections-list":    true,
	"ui-deadline-card":       true, // it is a .ui-card, already framed
	"ui-install-app":         true, // the install prompt's frame; it is a .ui-alert
	"ui-list-form":           true, // the <form> around a List with bulk actions
	"ui-md-img":              true, // .ui-md img is the rule; the class names the origin
	"ui-md-list":             true, // .ui-md ul/ol likewise
	"ui-quote":               true, // .ui-md blockquote likewise
	"ui-shell-user":          true, // the signed-in block of the top bar
	"ui-task":                true, // the polling frame of TaskPanel; its steps are drawn
	"ui-tree-branch":         true, // a branch beside .ui-tree-item
	"ui-tree-radio":          true, // the hidden input of a TreePicker
	"ui-webhooks":            true, // the whole WebhooksPanel; its actions are drawn
	"ui-webhooks-events":     true,
	"ui-webhooks-form":       true,
	"ui-webhooks-title":      true,
}

// #265 — every class the kit writes is either drawn by ui.css or declared a
// hook. The reporter found the gap by crossing 47 screens with the
// stylesheet; this crosses the source instead, so it runs before any screen.
func TestEveryClassTheKitWritesIsDrawnOrAHook(t *testing.T) {
	css := string(Asset("ui.css"))
	lit := regexp.MustCompile(`h\.Class\("([^"]+)"`)
	files, _ := filepath.Glob("*.go")
	written := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range lit.FindAllStringSubmatch(string(src), -1) {
			for _, c := range strings.Fields(m[1]) {
				// ui-variant-* and ui-size-* are markers variant() rewrites;
				// a name ending in "-" is a prefix completed at runtime.
				if !strings.HasPrefix(c, "ui-") || strings.HasPrefix(c, "ui-variant-") || strings.HasPrefix(c, "ui-size-") || strings.HasSuffix(c, "-") {
					continue
				}
				written[c] = true
			}
		}
	}
	var missing []string
	for c := range written {
		if hookClasses[c] {
			continue
		}
		if !strings.Contains(css, "."+c+" ") && !strings.Contains(css, "."+c+",") && !strings.Contains(css, "."+c+":") && !strings.Contains(css, "."+c+"[") && !strings.Contains(css, "."+c+">") && !strings.Contains(css, "."+c+".") {
			missing = append(missing, c)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("classes the kit writes and ui.css does not draw (draw them, or add them to hookClasses with the reason):\n  %s", strings.Join(missing, "\n  "))
	}
	for c := range hookClasses {
		if !written[c] {
			t.Errorf("hookClasses[%q] is not written by the kit anymore", c)
		}
	}
	// #265 (comment): the four of SearchResults and the two with visible
	// effect are drawn, not hooked.
	for _, c := range []string{"ui-search-results", "ui-search-hit", "ui-search-kind", "ui-search-title", "ui-badge-sm", "ui-field-display"} {
		if hookClasses[c] {
			t.Errorf("%s is a class with a visible effect, not a hook", c)
		}
	}
}
