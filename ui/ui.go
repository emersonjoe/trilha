// Package ui is Trilha's default, customizable UI kit: typed components that
// render classes consumed by public/ui.css, plus a small ui.js for behavior.
// The theme contract (CSS variables) is compatible with shadcn/ui themes.
package ui

import (
	"embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

//go:embed assets/ui.css assets/ui.theme.css assets/ui.js
var assets embed.FS

// Asset returns the embedded file (ui.css, ui.theme.css or ui.js).
func Asset(name string) []byte {
	b, err := assets.ReadFile("assets/" + name)
	if err != nil {
		panic("ui: unknown asset " + name)
	}
	return b
}

// Files lists the kit files written to a project's public/ folder.
var Files = []string{"ui.theme.css", "ui.css", "ui.js"}

// Head links the kit's stylesheets and script and applies the saved theme
// before first paint (inline script with the request nonce, so the default
// CSP accepts it). Put it inside <head>.
func Head(c *trilha.Ctx) h.Node {
	b := c.Base()
	return h.Fragment(
		h.Link(h.Rel("stylesheet"), h.Href(b+"/ui.theme.css")),
		h.Link(h.Rel("stylesheet"), h.Href(b+"/ui.css")),
		h.Script(trilha.NonceAttr(c), h.Raw(themeInit)),
		h.Script(h.Src(b+"/ui.js"), h.Defer()),
	)
}

const themeInit = `(()=>{try{var t=localStorage.getItem("ui-theme")}catch(e){}if(!t)t=matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";document.documentElement.classList.add(t)})()`

// ---- variants (extra class attributes, composable like any h attribute) -----

// Secondary, Outline, Ghost, Destructive and LinkStyle are button/badge variants.
func Secondary() h.Node   { return h.Class("ui-variant-secondary") }
func Outline() h.Node     { return h.Class("ui-variant-outline") }
func Ghost() h.Node       { return h.Class("ui-variant-ghost") }
func Destructive() h.Node { return h.Class("ui-variant-destructive") }
func LinkStyle() h.Node   { return h.Class("ui-variant-link") }

// Sm, Lg and IconSize are button sizes.
func Sm() h.Node       { return h.Class("ui-size-sm") }
func Lg() h.Node       { return h.Class("ui-size-lg") }
func IconSize() h.Node { return h.Class("ui-size-icon") }

// variant rewrites ui-variant-*/ui-size-* markers into <prefix>-* classes so
// the same Outline() works for buttons (ui-btn-outline) and badges
// (ui-badge-outline).
func variant(prefix string, nodes []h.Node) []h.Node {
	out := make([]h.Node, 0, len(nodes)+1)
	out = append(out, h.Class(prefix))
	for _, n := range nodes {
		s, ok := classOf(n)
		if !ok {
			out = append(out, n)
			continue
		}
		var cls []string
		for _, c := range strings.Fields(s) {
			switch {
			case strings.HasPrefix(c, "ui-variant-"):
				cls = append(cls, prefix+"-"+strings.TrimPrefix(c, "ui-variant-"))
			case strings.HasPrefix(c, "ui-size-"):
				cls = append(cls, prefix+"-"+strings.TrimPrefix(c, "ui-size-"))
			default:
				cls = append(cls, c)
			}
		}
		out = append(out, h.Class(cls...))
	}
	return out
}

func classOf(n h.Node) (string, bool) {
	s, err := h.Render(n)
	if err != nil || !strings.HasPrefix(s, ` class="`) {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(s, ` class="`), `"`), true
}

// ---- layout ----------------------------------------------------------------

// Body returns the class for <body>: base font and theme colors.
func Body() h.Node { return h.Class("ui-body") }

func Container(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-container")}, children...)...)
}
func Stack(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-stack")}, children...)...)
}
func Row(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-row")}, children...)...)
}
func Grid(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-grid")}, children...)...)
}
func Spacer() h.Node { return h.Div(h.Class("ui-spacer")) }

// Header is a sticky top bar; Brand is the app name link; Nav holds links.
func Header(children ...h.Node) h.Node {
	return h.Header(h.Class("ui-header"), Container(children...))
}
func Brand(href, name string) h.Node { return h.A(h.Class("ui-brand"), h.Href(href), h.Text(name)) }
func Nav(children ...h.Node) h.Node {
	return h.Nav(append([]h.Node{h.Class("ui-nav")}, children...)...)
}

// NavLink marks the current page with aria-current when current is true.
func NavLink(href, label string, current bool) h.Node {
	n := []h.Node{h.Href(href), h.Text(label)}
	if current {
		n = append(n, h.Aria("current", "page"))
	}
	return h.A(n...)
}

// Sidebar is a vertical navigation column.
func Sidebar(children ...h.Node) h.Node {
	return h.Aside(append([]h.Node{h.Class("ui-sidebar")}, children...)...)
}

// H1, H2, H3, Lead and Muted are typographic helpers.
func H1(children ...h.Node) h.Node { return h.H1(append([]h.Node{h.Class("ui-h1")}, children...)...) }
func H2(children ...h.Node) h.Node { return h.H2(append([]h.Node{h.Class("ui-h2")}, children...)...) }
func H3(children ...h.Node) h.Node { return h.H3(append([]h.Node{h.Class("ui-h3")}, children...)...) }
func Lead(children ...h.Node) h.Node {
	return h.P(append([]h.Node{h.Class("ui-lead")}, children...)...)
}
func Muted(children ...h.Node) h.Node {
	return h.Span(append([]h.Node{h.Class("ui-muted")}, children...)...)
}
func Code(s string) h.Node { return h.Code(h.Class("ui-code"), h.Text(s)) }
func Kbd(s string) h.Node  { return h.Kbd(h.Class("ui-kbd"), h.Text(s)) }

// ---- button ----------------------------------------------------------------

// Button renders <button class="ui-btn">; add Secondary(), Outline(), Ghost(),
// Destructive(), LinkStyle(), Sm(), Lg(), IconSize() and any h attribute.
func Button(children ...h.Node) h.Node {
	return h.Button(append(variant("ui-btn", children), h.Type("button"))...)
}

// Submit is a Button of type submit.
func Submit(children ...h.Node) h.Node {
	return h.Button(append([]h.Node{h.Type("submit")}, variant("ui-btn", children)...)...)
}

// ButtonLink renders an <a> styled as a button.
func ButtonLink(href string, children ...h.Node) h.Node {
	return h.A(append(variant("ui-btn", children), h.Href(href))...)
}

// ---- card ------------------------------------------------------------------

func Card(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-card")}, children...)...)
}
func CardHeader(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-card-header")}, children...)...)
}
func CardTitle(s string) h.Node       { return h.H3(h.Class("ui-card-title"), h.Text(s)) }
func CardDescription(s string) h.Node { return h.P(h.Class("ui-card-description"), h.Text(s)) }
func CardContent(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-card-content")}, children...)...)
}
func CardFooter(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-card-footer")}, children...)...)
}

// ---- forms -----------------------------------------------------------------

func Input(attrs ...h.Node) h.Node {
	return h.Input(append([]h.Node{h.Class("ui-input")}, attrs...)...)
}
func Textarea(attrs ...h.Node) h.Node {
	return h.Textarea(append([]h.Node{h.Class("ui-textarea")}, attrs...)...)
}
func Select(children ...h.Node) h.Node {
	return h.Select(append([]h.Node{h.Class("ui-select")}, children...)...)
}
func Checkbox(attrs ...h.Node) h.Node {
	return h.Input(append([]h.Node{h.Type("checkbox"), h.Class("ui-checkbox")}, attrs...)...)
}
func Radio(attrs ...h.Node) h.Node {
	return h.Input(append([]h.Node{h.Type("radio"), h.Class("ui-radio")}, attrs...)...)
}
func Switch(attrs ...h.Node) h.Node {
	return h.Input(append([]h.Node{h.Type("checkbox"), h.Role("switch"), h.Class("ui-switch")}, attrs...)...)
}
func Label(children ...h.Node) h.Node {
	return h.Label(append([]h.Node{h.Class("ui-label")}, children...)...)
}

// CheckRow puts a checkbox/switch/radio beside its label.
func CheckRow(control h.Node, label string, forID string) h.Node {
	return h.Div(h.Class("ui-check-row"), control, Label(h.For(forID), h.Text(label)))
}

// Field is label + control + optional help/error. id must match the control's id.
// An error marks the control invalid via aria-describedby on the wrapper.
func Field(id, label string, control h.Node, opts ...FieldOpt) h.Node {
	f := fieldCfg{}
	for _, o := range opts {
		o(&f)
	}
	n := []h.Node{h.Class("ui-field"), Label(h.For(id), h.Text(label)), control}
	if f.help != "" {
		n = append(n, h.P(h.Class("ui-field-help"), h.ID(id+"-help"), h.Text(f.help)))
	}
	if f.err != "" {
		n = append(n, h.P(h.Class("ui-field-error"), h.ID(id+"-error"), h.Role("alert"), h.Text(f.err)))
	}
	n = append(n, f.extra...)
	return h.Div(n...)
}

type fieldCfg struct {
	help, err string
	extra     []h.Node
}

// FieldOpt configures Field.
type FieldOpt func(*fieldCfg)

func Help(s string) FieldOpt  { return func(f *fieldCfg) { f.help = s } }
func Error(s string) FieldOpt { return func(f *fieldCfg) { f.err = s } }

// With adds attributes to the field wrapper (e.g. ShowWhen).
func With(nodes ...h.Node) FieldOpt { return func(f *fieldCfg) { f.extra = append(f.extra, nodes...) } }

// Invalid marks a control invalid (red ring). Pass it to Input/Select when
// there is an error for the field.
func Invalid() h.Node { return h.Aria("invalid", "true") }

// ShowWhen shows the element only while the named form field has one of the
// given values ("a|b"); with no values, while the field is non-empty. Hidden
// groups have their controls disabled, so they are not submitted.
func ShowWhen(field string, values ...string) h.Node {
	if len(values) == 0 {
		return h.Data("ui-show-when", field)
	}
	return h.Data("ui-show-when", field+"="+strings.Join(values, "|"))
}

// ---- badge, alert, toast ---------------------------------------------------

func Badge(children ...h.Node) h.Node { return h.Span(variant("ui-badge", children)...) }

// Alert renders a titled notice; add Destructive() for errors and an Icon first.
func Alert(title string, children ...h.Node) h.Node {
	n := variant("ui-alert", children)
	n = append(n, h.Role("alert"), h.H4(h.Class("ui-alert-title"), h.Text(title)))
	return h.Div(n...)
}

// AlertDescription is the body of an Alert.
func AlertDescription(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-alert-description")}, children...)...)
}

// Toaster is the container toasts stack into; place it once in the layout.
func Toaster(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-toaster"), h.Aria("live", "polite")}, children...)...)
}

// Toast is a message that fades out after fadeMs (0 = stays). kind: "",
// "success" or "error". Render it inside Toaster (e.g. after a form post).
func Toast(kind, text string, fadeMs int) h.Node {
	n := []h.Node{h.Class("ui-toast"), h.Role("status")}
	if kind != "" {
		n = append(n, h.Class("ui-toast-"+kind))
		icon := map[string]string{"success": "circle-check", "error": "circle-x"}[kind]
		if icon != "" {
			n = append(n, Icon(icon))
		}
	}
	if fadeMs > 0 {
		n = append(n, h.Data("ui-fade", strconv.Itoa(fadeMs)))
	}
	return h.Div(append(n, h.Span(h.Text(text)))...)
}

// ---- table -----------------------------------------------------------------

// Table wraps a <table class="ui-table"> in a horizontally scrollable box.
func Table(children ...h.Node) h.Node {
	return h.Div(h.Class("ui-table-wrap"), h.Table(append([]h.Node{h.Class("ui-table")}, children...)...))
}

// Num right-aligns numeric cells.
func Num() h.Node { return h.Class("ui-num") }

// Depth indents the first cell of a row (tree tables, drill-down).
func Depth(d int) h.Node { return h.Data("depth", strconv.Itoa(d)) }

// ---- tabs ------------------------------------------------------------------

// Tab is one tab of Tabs.
type Tab struct {
	Label   string
	Content h.Node
}

// Tabs renders an accessible tab list; the first tab starts selected.
func Tabs(id string, tabs ...Tab) h.Node {
	list := []h.Node{h.Class("ui-tabs-list"), h.Role("tablist")}
	var panels []h.Node
	for i, t := range tabs {
		tid := fmt.Sprintf("%s-tab-%d", id, i)
		pid := fmt.Sprintf("%s-panel-%d", id, i)
		sel := "false"
		tabindex := "-1"
		if i == 0 {
			sel, tabindex = "true", "0"
		}
		list = append(list, h.Button(h.Class("ui-tab"), h.Type("button"), h.Role("tab"), h.ID(tid),
			h.Aria("selected", sel), h.Aria("controls", pid), h.Tabindex(tabindex), h.Text(t.Label)))
		p := []h.Node{h.Class("ui-tab-panel"), h.Role("tabpanel"), h.ID(pid), h.Aria("labelledby", tid), t.Content}
		if i > 0 {
			p = append(p, h.Hidden())
		}
		panels = append(panels, h.Div(p...))
	}
	return h.Div(append([]h.Node{h.ID(id), h.Data("ui-tabs", "")}, append([]h.Node{h.Div(list...)}, panels...)...)...)
}

// ---- dialog ----------------------------------------------------------------

// Dialog renders a native <dialog>; open it with DialogTrigger(id, ...).
func Dialog(id, title string, children ...h.Node) h.Node {
	n := []h.Node{h.Class("ui-dialog"), h.ID(id), h.Aria("labelledby", id+"-title"),
		h.Button(h.Class("ui-btn ui-btn-ghost ui-btn-icon ui-btn-sm ui-dialog-close"), h.Type("button"), h.Data("ui-dialog-close", ""), h.Aria("label", "Fechar"), Icon("x")),
		h.H2(h.Class("ui-dialog-title"), h.ID(id+"-title"), h.Text(title))}
	return h.Dialog(append(n, children...)...)
}
func DialogDescription(s string) h.Node { return h.P(h.Class("ui-dialog-description"), h.Text(s)) }
func DialogFooter(children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-dialog-footer")}, children...)...)
}

// DialogTrigger is a Button that opens the dialog with the given id.
func DialogTrigger(id string, children ...h.Node) h.Node {
	return Button(append(children, h.Data("ui-dialog-open", id))...)
}

// DialogClose is a Button that closes the enclosing dialog.
func DialogClose(children ...h.Node) h.Node {
	return Button(append(children, h.Data("ui-dialog-close", ""))...)
}

// ---- menu (native popover) -------------------------------------------------

// MenuTrigger toggles the popover menu with the given id.
func MenuTrigger(id string, children ...h.Node) h.Node {
	return Button(append(children, h.Attr("popovertarget", id), h.Aria("haspopup", "menu"))...)
}

// Menu is a popover list of MenuItem/MenuLink.
func Menu(id string, children ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-menu"), h.ID(id), h.Attr("popover", "auto"), h.Role("menu")}, children...)...)
}
func MenuItem(children ...h.Node) h.Node {
	return h.Button(append([]h.Node{h.Type("button"), h.Role("menuitem")}, children...)...)
}
func MenuLink(href string, children ...h.Node) h.Node {
	return h.A(append([]h.Node{h.Href(href), h.Role("menuitem")}, children...)...)
}

// ---- misc ------------------------------------------------------------------

func Separator() h.Node { return h.Hr(h.Class("ui-separator")) }

// Skeleton is a loading placeholder; pass StyleAttr for size.
func Skeleton(attrs ...h.Node) h.Node {
	return h.Div(append([]h.Node{h.Class("ui-skeleton"), h.Aria("hidden", "true")}, attrs...)...)
}

// Progress renders a bar at value/max.
func Progress(value, max int) h.Node {
	pct := 0
	if max > 0 {
		pct = value * 100 / max
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
	}
	return h.Div(h.Class("ui-progress"), h.Role("progressbar"), h.Aria("valuenow", strconv.Itoa(value)), h.Aria("valuemax", strconv.Itoa(max)),
		h.Div(h.StyleAttr("width:"+strconv.Itoa(pct)+"%")))
}

// Crumb is one breadcrumb item; the last one is the current page.
type Crumb struct {
	Label string
	Href  string
}

// Breadcrumb renders a navigation trail.
func Breadcrumb(items ...Crumb) h.Node {
	n := []h.Node{h.Class("ui-breadcrumb")}
	for i, it := range items {
		if i > 0 {
			n = append(n, h.Li(h.Aria("hidden", "true"), Icon("chevron-right")))
		}
		if i == len(items)-1 || it.Href == "" {
			n = append(n, h.Li(h.Aria("current", "page"), h.Text(it.Label)))
		} else {
			n = append(n, h.Li(h.A(h.Href(it.Href), h.Text(it.Label))))
		}
	}
	return h.Nav(h.Aria("label", "breadcrumb"), h.Ol(n...))
}

// Avatar shows an image or the initials as fallback.
func Avatar(initials, src string) h.Node {
	if src != "" {
		return h.Span(h.Class("ui-avatar"), h.Img(h.Src(src), h.Alt(initials)))
	}
	return h.Span(h.Class("ui-avatar"), h.Aria("label", initials), h.Text(initials))
}

// Collapsible is a <details> with a styled summary; pass h.Open() to start open.
func Collapsible(summary string, children ...h.Node) h.Node {
	return h.Details(append([]h.Node{h.Class("ui-collapsible"), h.Summary(h.Text(summary))}, children...)...)
}

// ThemeToggle switches light/dark and remembers the choice.
func ThemeToggle() h.Node {
	return Button(Ghost(), IconSize(), h.Data("ui-theme-toggle", ""), h.Aria("label", "Alternar tema"), Icon("sun", h.Class("ui-only-light")), Icon("moon", h.Class("ui-only-dark")))
}

// Icon renders an embedded Lucide icon (ISC). It panics on unknown names,
// which is a programming error caught at first render. See Icons.
func Icon(name string, attrs ...h.Node) h.Node {
	body, ok := icons[name]
	if !ok {
		panic("ui.Icon: unknown icon " + name)
	}
	n := []h.Node{h.Class("ui-icon"), h.Attr("viewBox", "0 0 24 24"), h.Attr("fill", "none"), h.Attr("stroke", "currentColor"),
		h.Attr("stroke-width", "2"), h.Attr("stroke-linecap", "round"), h.Attr("stroke-linejoin", "round"), h.Aria("hidden", "true")}
	n = append(n, attrs...)
	return h.Svg(append(n, h.Raw(body))...)
}

// Icons lists the available icon names.
func Icons() []string {
	out := make([]string, 0, len(icons))
	for k := range icons {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
