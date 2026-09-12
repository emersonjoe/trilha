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

//go:embed assets/ui.css assets/ui.theme.css assets/ui.js assets/ui.nav.js assets/ui.upload.js assets/ui.live.js assets/ui.chat.js assets/ui.island.js assets/ui.tree.js
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
var Files = []string{"ui.theme.css", "ui.css", "ui.js", "ui.nav.js", "ui.upload.js", "ui.live.js", "ui.chat.js", "ui.island.js", "ui.tree.js"}

// Head links the kit's stylesheets and script and applies the saved theme
// before first paint (inline script with the request nonce, so the default
// CSP accepts it). Put it inside <head>.
func Head(c *trilha.Ctx) h.Node {
	return h.Fragment(
		h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/ui.theme.css"))),
		h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/ui.css"))),
		h.Script(trilha.NonceAttr(c), h.Raw(themeInit)),
		h.Script(h.Src(c.Asset("/ui.js")), h.Defer()),
	)
}

const themeInit = `(()=>{var s;try{var t=localStorage.getItem("ui-theme");s=localStorage.getItem("ui-sidebar")}catch(e){}if(!t)t=matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light";document.documentElement.classList.add(t);if(s==="collapsed"||(!s&&matchMedia("(max-width: 767px)").matches))document.documentElement.classList.add("ui-sidebar-collapsed")})()`

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

// Container, Stack, Row, Grid and Spacer are the layout boxes: Container
// centers the page at a readable width, Stack piles children vertically, Row
// puts them side by side, Grid wraps them into columns and Spacer pushes what
// comes after it to the far end of a Row.
//
//	ui.Container(ui.Stack(ui.H1(h.Text("Invoices")), ui.Row(filters, ui.Spacer(), newButton)))
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
//
//	ui.Nav(ui.NavLink("/invoices", "Invoices", c.Pattern() == "/invoices"))
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

// H1, H2, H3, Lead, Muted, Code and Kbd are the typographic helpers: the three
// headings, the opening paragraph, secondary text, a snippet inline and a key
// on the keyboard.
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

// Card, CardHeader, CardTitle, CardDescription, CardContent and CardFooter are
// the panel and its parts. Only Card is required; the rest are there so the
// spacing inside is not repeated in every screen.
//
//	ui.Card(
//		ui.CardHeader(ui.CardTitle("Invoice 42"), ui.CardDescription("Due in 3 days")),
//		ui.CardContent(body),
//		ui.CardFooter(ui.Button(h.Text("Pay"))),
//	)
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

// Input, Textarea, Select, Checkbox, Radio, Switch and Label are the form
// controls, styled and nothing else: they take the attributes of the element
// they are, so h.Name, h.Required and h.Value work as they always did.
//
//	ui.Input(h.Type("email"), h.Name("email"), h.Required())
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

// CheckRow puts a checkbox/switch/radio beside its label. forID is the
// control's id: without it the label is decoration and the click area is the
// box alone.
//
//	ui.CheckRow(ui.Checkbox(h.ID("terms"), h.Name("terms")), "I agree", "terms")
func CheckRow(control h.Node, label string, forID string) h.Node {
	return h.Div(h.Class("ui-check-row"), control, Label(h.For(forID), h.Text(label)))
}

// Field is label + control + optional help/error. id must match the control's id.
// An error marks the control invalid via aria-describedby on the wrapper.
//
//	ui.Field("email", "E-mail",
//		ui.Input(h.ID("email"), h.Type("email"), h.Name("email"), ui.InvalidIf(errs, "email")),
//		ui.Errors(errs, "email"))
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

// Help, Error, Errors and With are the FieldOpt options: a hint under the
// control, one message, the messages a FieldErrors carries for that name, and
// anything else to put inside the field.
func Help(s string) FieldOpt  { return func(f *fieldCfg) { f.help = s } }
func Error(s string) FieldOpt { return func(f *fieldCfg) { f.err = s } }

// With adds attributes to the field wrapper (e.g. ShowWhen).
func With(nodes ...h.Node) FieldOpt { return func(f *fieldCfg) { f.extra = append(f.extra, nodes...) } }

// Invalid marks a control invalid (red ring). Pass it to Input/Select when
// there is an error for the field.
func Invalid() h.Node { return h.Aria("invalid", "true") }

// Navigate marks a region whose links navigate on the client: a click on a
// same-origin link inside it fetches the next page and replaces the element
// with the given id, instead of reloading the document. An empty id means the
// marked element is itself the one to replace:
//
//	h.Main(h.ID("conteudo"), ui.Navigate(""), children)
//
// It is off until the region is marked and NavigateScript is on the page, and
// it never changes what the server answers: the address in the bar is the one
// a normal navigation would use, and reloading it renders the same page.
func Navigate(id string) h.Node { return h.Data("trilha-nav", id) }

// NoNavigate keeps one link out of it (a download, another app, a route that
// must reload for real), inside a region marked with Navigate.
func NoNavigate() h.Node { return h.Data("trilha-nav", "false") }

// NavigateScript loads ui.nav.js, the behavior behind Navigate. Put it once,
// in the layout of the area that uses it — the kit's Head does not load it, so
// an app without client navigation does not download it.
func NavigateScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.nav.js")), h.Defer())
}

// UploadTo marks a form that sends a file: the kit sends it with XHR, shows how
// much has gone out and swaps element id with what comes back — the same
// fragment the route already answers. Without JavaScript the form submits
// normally and the route answers the whole page.
//
//	h.Form(h.Method("post"), h.Action("/anexos"), h.EncType("multipart/form-data"),
//		ui.UploadTo("lista"), trilha.CSRFInput(c),
//		ui.Input(h.Type("file"), h.Name("arquivo")), ui.UploadBar(),
//		ui.Submit(h.Text("Send")))
//
// It is a different attribute from Swap on purpose: a form carrying both would
// be sent twice.
func UploadTo(id string) h.Node { return h.Data("trilha-upload", id) }

// UploadBar is the progress element UploadTo fills in. It starts hidden and is
// only shown while sending; style it through the progress selector.
func UploadBar(attrs ...h.Node) h.Node {
	return h.Progress(append([]h.Node{h.Data("trilha-progress", ""), h.Class("ui-upload"), h.Hidden(), h.Value("0"), h.Max("100")}, attrs...)...)
}

// UploadScript loads ui.upload.js, the behavior behind UploadTo. Put it once,
// in the layout or page that has the form; ui.Head does not load it.
func UploadScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.upload.js")), h.Defer())
}

// Swap marks a link or form as a fragment request: clicking or submitting it
// asks the same route for the part of the page with the given id and replaces
// that element, without reloading.
//
//	h.Form(h.Method("get"), ui.Swap("lista"), busca())
//	h.Div(h.ID("lista"), ...)
//
// With JavaScript off nothing here applies: the link navigates and the form
// submits as usual, and the route answers with the whole page. On the server,
// Ctx.Fragment reports the requested id.
func Swap(id string) h.Node { return h.Data("trilha-target", id) }

// Indicator marks an element as the pending signal for the target with the
// given id: it stays hidden while nothing is happening and appears once the
// request for that target has been in flight past the threshold. Put it
// anywhere — beside the target, in the header, inside the button.
//
//	ui.Spinner(ui.Indicator("lista"))
//
// The threshold is what keeps a fast answer from making it blink; see
// PendingAfter. Several indicators may watch the same target.
func Indicator(id string) h.Node { return h.Data("trilha-indicator", id) }

// PendingAfter sets, in milliseconds, how long a request may be in flight
// before the page says so. Put it on the element that carries Swap or on the
// one that carries Navigate. The default is 120 ms, which is about where a
// person starts to notice a wait; zero or less means the default.
//
// The threshold is the whole point: showing a spinner for the 40 ms answer is
// the flash people complain about, not a courtesy.
func PendingAfter(ms int) h.Node {
	if ms <= 0 {
		return h.Fragment()
	}
	return h.Data("trilha-pending-after", strconv.Itoa(ms))
}

// NoTransition turns off the crossfade a swap or a navigation would animate on
// this trigger. The transition already does nothing where the browser has no
// startViewTransition and where the system asks for less motion, so this is for
// the page that wants the instant replacement on purpose.
func NoTransition() h.Node { return h.Data("trilha-transition", "false") }

// Spinner is a turning ring, sized by the font it sits in. It is decorative —
// the waiting is announced by aria-busy on the target — so it is hidden from
// assistive technology.
func Spinner(attrs ...h.Node) h.Node {
	return h.Span(append([]h.Node{h.Class("ui-spinner"), h.Aria("hidden", "true")}, attrs...)...)
}

// NoPush turns off the history entry a fragment link would create.
func NoPush() h.Node { return h.Data("trilha-push", "false") }

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

// Badge is a small label for a status; the variants apply.
//
//	ui.Badge(ui.Secondary(), h.Text("draft"))
func Badge(children ...h.Node) h.Node { return h.Span(variant("ui-badge", children)...) }

// Alert renders a titled notice; add Destructive() for errors and an Icon first.
func Alert(title string, children ...h.Node) h.Node {
	n := variant("ui-alert", children)
	head := []h.Node{n[0], h.Role("alert"), h.H4(h.Class("ui-alert-title"), h.Text(title))}
	return h.Div(append(head, n[1:]...)...)
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
//
//	ui.Toaster(ui.Toast("success", "Invoice sent.", 4000))
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

// Kinds of trilha.Flash, which is where the toast that shows it takes its
// colour from. FlashInfo is the neutral one.
const (
	FlashInfo    = ""
	FlashSuccess = "success"
	FlashError   = "error"
)

// FlashFadeMs is how long a flash toast stays on screen.
const FlashFadeMs = 5000

// Flashes renders the messages left by c.Flash on the request before, inside
// the toaster. It replaces Toaster() in the layout:
//
//	h.Body(..., ui.Flashes(c))
//
// An empty toaster is still rendered, because ui.js appends live toasts to it.
func Flashes(c *trilha.Ctx) h.Node {
	var toasts []h.Node
	for _, f := range c.Flashes() {
		toasts = append(toasts, Toast(f.Kind, f.Text, FlashFadeMs))
	}
	return Toaster(toasts...)
}

// Confirm asks before the form it sits in is submitted. Put it in the form,
// next to the CSRF input:
//
//	h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
//		ui.Confirm("Delete this post?", "There is no undo."),
//		ui.Submit(Destructive(), h.Text("Delete")))
//
// ui.js holds the submit, opens the kit's dialog and only then lets it
// through, fragment forms included. The confirming button repeats the label of
// the button that was pressed; the other one says "Cancel", or whatever
// h.Data("ui-confirm-cancel", "…") says on the same form. Without JavaScript
// the form submits straight away — when that is not good enough, ask on a page
// of its own, which works either way.
func Confirm(title, description string) h.Node {
	n := []h.Node{h.Data("ui-confirm", title)}
	if description != "" {
		n = append(n, h.Data("ui-confirm-description", description))
	}
	return h.Attrs(n...)
}

// ---- table -----------------------------------------------------------------

// Table wraps a <table class="ui-table"> in a horizontally scrollable box.
// Pass Cards() to turn each row into a card below 640px instead — a listing
// with many columns reads better than sideways scrolling on a phone; a wide
// numeric table (a comparison, a statement) is sometimes still better rolling.
func Table(children ...h.Node) h.Node {
	return h.Div(h.Class("ui-table-wrap"), h.Table(append([]h.Node{h.Class("ui-table")}, children...)...))
}

// Cards turns a Table's rows into cards below 640px: the header hides off
// screen (still reachable to a screen reader) and each cell's data-label
// takes its place beside the value. ui.DataTable writes data-label on every
// column already; a hand-written Table needs it added to its own <td>s.
func Cards() h.Node { return h.Class("ui-table-cards") }

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
//
//	ui.DialogTrigger("confirm", h.Text("Delete"))
//	ui.Dialog("confirm", "Delete the invoice?",
//		ui.DialogDescription("This cannot be undone."),
//		ui.DialogFooter(ui.DialogClose(ui.Outline(), h.Text("Cancel")), deleteForm))
func Dialog(id, title string, children ...h.Node) h.Node {
	n := []h.Node{h.Class("ui-dialog"), h.ID(id), h.Aria("labelledby", id+"-title"),
		h.Button(h.Class("ui-btn ui-btn-ghost ui-btn-icon ui-btn-sm ui-dialog-close"), h.Type("button"), h.Data("ui-dialog-close", ""), h.Aria("label", "Fechar"), Icon("x")),
		h.H2(h.Class("ui-dialog-title"), h.ID(id+"-title"), h.Text(title))}
	return h.Dialog(append(n, children...)...)
}

// DialogDescription is the line under a Dialog title, and DialogFooter is the
// row of buttons at the bottom.
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

// MenuItem is an entry that does something (a button) and MenuLink is one that
// goes somewhere (a link). Both belong inside a Menu.
func MenuItem(children ...h.Node) h.Node {
	return h.Button(append([]h.Node{h.Type("button"), h.Role("menuitem")}, children...)...)
}
func MenuLink(href string, children ...h.Node) h.Node {
	return h.A(append([]h.Node{h.Href(href), h.Role("menuitem")}, children...)...)
}

// ---- misc ------------------------------------------------------------------

// Separator is a horizontal rule between sections.
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

// Pages describes the page navigation Pagination renders.
type Pages struct {
	Page, Total int              // current page (1-based) and how many there are
	Href        func(int) string // address of a page
	Prev, Next  string           // labels; "Previous" and "Next" when empty
	Label       string           // aria-label of the <nav>; "Pagination" when empty
	Attrs       h.Node           // extra attributes for every page link (ui.Swap, for one)
}

// Pagination renders page navigation as real links, so a page can be shared,
// reloaded and indexed. The current page is a <span> with aria-current: it is
// where the visitor already is, and a link to here is a link to nowhere. The
// edges are not disabled links either — the first page has no previous, so
// nothing is rendered for it. A list with one page renders nothing at all.
//
// The window keeps the first and last page and the ones around the current
// one, with an ellipsis over each gap, so the footer does not grow with the
// table.
func Pagination(p Pages) h.Node {
	if p.Href == nil || p.Total < 2 {
		return h.Group()
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	if page > p.Total {
		page = p.Total
	}
	var items []h.Node
	if page > 1 {
		items = append(items, h.Li(h.A(h.Rel("prev"), h.Href(p.Href(page-1)), attrs(p.Attrs), h.Text(label(p.Prev, "Previous")))))
	}
	for _, n := range pageWindow(page, p.Total) {
		switch {
		case n == 0:
			items = append(items, h.Li(h.Aria("hidden", "true"), h.Text("…")))
		case n == page:
			items = append(items, h.Li(h.Span(h.Aria("current", "page"), h.Text(strconv.Itoa(n)))))
		default:
			items = append(items, h.Li(h.A(h.Href(p.Href(n)), attrs(p.Attrs), h.Text(strconv.Itoa(n)))))
		}
	}
	if page < p.Total {
		items = append(items, h.Li(h.A(h.Rel("next"), h.Href(p.Href(page+1)), attrs(p.Attrs), h.Text(label(p.Next, "Next")))))
	}
	return h.Nav(h.Class("ui-pagination"), h.Aria("label", label(p.Label, "Pagination")), h.Ul(items...))
}

// attrs is the optional extra attribute of a link, or nothing.
func attrs(n h.Node) h.Node {
	if n == nil {
		return h.Group()
	}
	return n
}

// pageWindow returns the page numbers to show, with 0 standing for a gap.
// Seven slots either way, so the footer is the same width on every page.
func pageWindow(page, total int) []int {
	if total <= 7 {
		all := make([]int, 0, total)
		for n := 1; n <= total; n++ {
			all = append(all, n)
		}
		return all
	}
	switch {
	case page <= 4:
		return []int{1, 2, 3, 4, 5, 0, total}
	case page >= total-3:
		return []int{1, 0, total - 4, total - 3, total - 2, total - 1, total}
	default:
		return []int{1, 0, page - 1, page, page + 1, 0, total}
	}
}

func label(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// Tooltip attaches a hint to what it wraps. The text is in title, so the hint
// works with ui.js off — that is the browser's own tooltip. With the script on
// the page, title is removed (two tooltips is worse than none), a bubble with
// role=tooltip takes its place and the target gets aria-describedby, so the
// hint is reachable by keyboard focus and by touch, and closes with Escape.
//
// The text is a string on purpose: a hint with a link inside is a popover, and
// that is what Menu is for.
func Tooltip(text string, children ...h.Node) h.Node {
	return h.Span(append([]h.Node{h.Class("ui-tooltip"), h.Data("ui-tooltip", text), h.Attr("title", text)}, children...)...)
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

// ---- form helpers for validation round-trips ------------------------------

// Errors is a Field option that shows the message for field from errs (a
// trilha.FieldErrors or any map); no-op when the field has no error.
func Errors(errs map[string]string, field string) FieldOpt {
	return func(f *fieldCfg) {
		if msg, ok := errs[field]; ok && msg != "" {
			f.err = msg
		}
	}
}

// InvalidIf marks a control invalid when errs has a message for field.
func InvalidIf(errs map[string]string, field string) h.Node {
	if msg, ok := errs[field]; ok && msg != "" {
		return Invalid()
	}
	return h.Nil
}

// Option is one <option> of SelectOptions.
type Option struct {
	Value, Label string
}

// SelectOptions renders <option>s, marking the one equal to selected. A
// leading placeholder can be given as Option{"", "Escolha…"}.
func SelectOptions(opts []Option, selected string) h.Node {
	found := false
	for _, o := range opts {
		if o.Value == selected && o.Value != "" {
			found = true
		}
	}
	return h.Map(opts, func(o Option) h.Node {
		n := []h.Node{h.Value(o.Value), h.Text(o.Label)}
		// The placeholder is selected when nothing else matches, so the
		// browser does not silently pick the first real option.
		if o.Value == selected || (!found && o.Value == "") {
			n = append(n, h.Selected())
		}
		if o.Value == "" {
			n = append(n, h.Attr("disabled", ""))
		}
		return h.Option(n...)
	})
}

// Checked returns h.Checked() when v is true (checkbox/switch round-trips).
func Checked(v bool) h.Node {
	if v {
		return h.Checked()
	}
	return h.Nil
}

// Status renders the badge of an enum value: the label a person reads, in the
// tone the enum declared.
//
//	ui.Status(docs.Status, doc.Status)   // <span class="ui-badge ui-badge-success">Processed</span>
//
// A value the enum does not know renders raw, in the muted tone: a row written
// before somebody retired that value must not take the screen down.
//
//	see: trilha.Enum, ui.Badge
func Status(e trilha.Enum, value string) h.Node {
	if value == "" {
		return h.Fragment()
	}
	return h.Span(h.Class("ui-badge ui-badge-"+e.Tone(value)), h.Text(e.Label(value)))
}
