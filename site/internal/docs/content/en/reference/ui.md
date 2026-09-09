---
title: ui
description: The kit's components, variants, assets and the theme contract.
---

`import "github.com/emersonjoe/trilha/ui"` — stdlib only. Components return `h.Node` with
`ui-*` classes from `public/ui.css`; behaviors live in `public/ui.js`.

## Assets

| Symbol | Role |
|---|---|
| `ui.Head(c) h.Node` | `<link>` for `ui.theme.css` and `ui.css`, inline script (with nonce) that applies the saved theme, `<script defer src=ui.js>`; honors `c.Base()` |
| `ui.Body() h.Node` | `ui-body` class for the `<body>` |
| `ui.Asset(name) []byte` | embedded content of `ui.css`, `ui.theme.css`, `ui.js`, `ui.nav.js`, `ui.upload.js`, `ui.live.js`, `ui.chat.js` or `ui.island.js` |
| `ui.Files` | the six names, in the order `trilha ui` writes them |

## Variants and sizes

`ui.Secondary()`, `ui.Outline()`, `ui.Ghost()`, `ui.Destructive()`, `ui.LinkStyle()`,
`ui.Sm()`, `ui.Lg()`, `ui.IconSize()`. They are class attributes: valid on `Button`,
`Submit`, `ButtonLink`, `Badge` and `Alert` (each one translates to its own class, e.g.
`ui-btn-outline`, `ui-badge-outline`).

## Components

| Function | Renders |
|---|---|
| `Container, Stack, Row, Grid, Spacer` | layout: max width, column, row, responsive grid |
| `Header(children...)`, `Brand(href, name)`, `Nav(...)`, `NavLink(href, label, current)`, `Sidebar(...)` | sticky top bar, brand, navigation (with `aria-current`), side column |
| `H1, H2, H3, Lead, Muted, Code(s), Kbd(s)` | typography |
| `Button, Submit, ButtonLink(href, ...)` | `<button type=button>`, `<button type=submit>`, `<a>` styled as a button |
| `Card, CardHeader, CardTitle(s), CardDescription(s), CardContent, CardFooter` | card |
| `Input, Textarea, Select, Checkbox, Radio, Switch, Label` | controls (`Switch` has `role=switch`) |
| `Field(id, label, control, opts...)` | label + control + `Help(s)` + `Error(s)`; `With(nodes...)` puts attributes on the group |
| `CheckRow(control, label, id)` | checkbox/switch next to its label |
| `Invalid()` | `aria-invalid="true"` (red ring) |
| `Errors(errs, field)` | `Field` option: shows the message from `errs[field]` (a `trilha.FieldErrors`) if any |
| `InvalidIf(errs, field)` | `Invalid()` only when there is an error for the field |
| `SelectOptions([]Option{{Value, Label}}, selected)` | `<option>`s marking the selected one; `Value: ""` is a placeholder (disabled) and is selected when nothing matches |
| `Checked(bool)` | conditional `checked` (round trip of checkbox/switch/radio) |
| `ShowWhen(field, values...)` | `data-ui-show-when`: shows the element only with the value (or any non-empty value); hidden controls are disabled |
| `Combobox(ComboboxOpts{...}, attrs...)`, `ComboboxOptions(items, of)` | a text field that searches a list — see [Combobox](#combobox) |
| `Dropzone(DropzoneOpts{...}, children...)` | drag-and-drop area over a file input — see [Upload with progress](#upload-with-progress) |
| `SchemaForm(schema, values, errs, ...)` | a form defined by data: one field per `trilha.SchemaField` — see [Validation](/reference/validation) |
| `Badge`, `Alert(title, ...)`, `AlertDescription(...)` | badge and alert (`role=alert`) |
| `Toaster(...)`, `Toast(kind, text, fadeMs)` | toast stack; `kind` = `""`, `success`, `error`; `fadeMs > 0` disappears on its own |
| `Flashes(c)` | the toaster with the messages of [`c.Flash`](/reference/ctx) — put it in the layout; `FlashInfo`, `FlashSuccess` and `FlashError` are the kinds |
| `Table(...)`, `Num()`, `Depth(n)` | scrollable table; numeric cell; row indentation (tree) |
| `Tabs(id, Tab{Label, Content}...)` | accessible tabs (arrows, Home/End); the first starts open |
| `Dialog(id, title, ...)`, `DialogDescription(s)`, `DialogFooter(...)`, `DialogTrigger(id, ...)`, `DialogClose(...)` | native `<dialog>` with `showModal` |
| `Confirm(title, description)` | attributes for a `<form>`: `ui.js` asks in a dialog before submitting, fragment forms included. The confirming button repeats the pressed button's label; the other says `Cancel`, or what `h.Data("ui-confirm-cancel", "…")` says. Without JavaScript the form submits straight away |
| `Menu(id, ...)`, `MenuItem(...)`, `MenuLink(href, ...)`, `MenuTrigger(id, ...)` | menu with the native `popover` attribute |
| `Pagination(Pages{Page, Total, Href, Prev, Next, Label, Attrs})` | page navigation as links; the current page is a `<span>` with `aria-current`, the edges are absent instead of disabled, and a window of seven slots keeps the first and last page with `…` over each gap; one page renders nothing |
| `Tooltip(text, ...)` | hint on what it wraps: `title` plus `data-ui-tooltip`, upgraded by `ui.js` into a bubble with `role=tooltip` and `aria-describedby` |
| `Separator, Skeleton, Progress(value, max), Breadcrumb(Crumb{Label, Href}...), Avatar(initials, src), Collapsible(summary, ...)` | miscellaneous |
| `ThemeToggle()` | button that switches light/dark (`localStorage["ui-theme"]`) |
| `CSVErrors(c, res, CSVErrorsOpts{...})` | what `trilha.BindCSV` rejected, by line and column — see [CSV](/cookbook/csv) |
| `DataTable(c, Columns[T], rows, ListState)` | the listing: filter form, sortable headers, pagination and empty state, all in the URL — see [Listings](/reference/listings) |
| `Swap(id)` | `data-trilha-target`: the `<a>` or `<form>` asks for element `#id` only and swaps it (fragments) |
| `SettingsForm(c, section, errs)` | the administration screen of a `trilha.Settings` section, drawn from the struct — see [App](/reference/app) |
| `Tree(TreeOpts{...})`, `TreePicker(TreePickerOpts{...})`, `TreeItems`, `TreeScript(c)` | a hierarchy that opens node by node, and the field that picks one — see [Trees](#trees) |
| `AuditTable(c, records, AuditOpts{...})` | the trail c.Audit writes, with filter, pagination and CSV export — see [Observability](/reference/observability) |
| `Steps([]Step{Label, Href}, current)` | the indicator of a form in several screens — see [A form in steps](/cookbook/wizard) |
| `Preview(c, src, PreviewOpts{...})` | a file shown beside its metadata: bar, frame, image or "cannot be previewed" — see [Ctx](/reference/ctx) and [Uploads](/cookbook/uploads) |
| `Defer(c, id, src, DeferOpts{...})` | serves the page now and fills this part a moment later — see [Live fragments](/reference/live) |
| `Poll(every, src)`, `Live(src)`, `On(event, src)`, `LiveScript(c)` | a fragment that refreshes on a clock or on an event from the server — see [Live fragments](/reference/live) |
| `NoPush()` | `data-trilha-push="false"`: the swap leaves history alone |
| `Markdown(text, MarkdownOpts{...})` | model or visitor text as HTML, escaped by construction — see [Markdown](#markdown) |
| `Chat(c, ChatOpts{...})`, `ChatScript(c)`, `ChatHTML(text)` | a conversation with an agent — see [Chat](#chat) |
| `Icon(name, attrs...)`, `Icons()` | inline Lucide SVG; unknown name → panic (programming error) |

## Trees

A hierarchy with thousands of nodes is the component people go to npm for: expanding, searching
and the keyboard are each easy and together are three hundred lines. `ui.Tree` is the server's
version of it — the server already knows the tree, so the browser never has to.

```go
ui.Tree(ui.TreeOpts{
	Nodes:   roots,                   // with the path down to Current already inside
	Source:  "/classification/nodes", // GET ?parent=100.1 answers the children
	Current: doc.Code,
	Label:   "Classification plan",
})
```

| Symbol | Role |
|---|---|
| `Tree(TreeOpts{...})` | the hierarchy; each node is a `<details>`, so it opens with no script at all |
| `TreeNode{Value, Label, Leaf, Href, Children, Open, Path}` | one node; `Children` travel with it when they are already known |
| `TreeItems(nodes, TreeOpts{...})` / `TreeNodes(items, of)` | what a source route answers: the children of one node, as HTML |
| `TreePicker(TreePickerOpts{...})` | the same tree as a form field: **a radio per node** |
| `TreeScript(c)` | loads `ui.tree.js`; a page with no tree does not download it |

**A node is `<details>`, and that is the whole no-JavaScript story.** What the script adds is
fetching the children the first time a branch opens, instead of asking the server for a whole
page. A node whose `Children` are already in `Nodes` never asks for anything — which is how the
path down to the current node arrives open and complete on the first render, including after a
422 brought the form back.

The roles are the real ones (`tree`, `treeitem`, `group`, `aria-expanded`), the arrows move
through what is visible, `Home` and `End` jump to the ends, and `*` expands everything. Only the
first node is in the tab order: the tree is one stop, and the arrows move inside it.

### The picker

```go
ui.Field("code", "Classification", ui.TreePicker(ui.TreePickerOpts{
	Name:   "code",
	Value:  form.Code,
	Nodes:  plan.Roots(form.Code),
	Source: "/classification/nodes",
	Search: "/classification/search", // GET ?q= answers flattened nodes, each with its Path
}))
```

**What posts is a radio**, which is the whole reason this works with no script: somebody browses
the same `<details>` and picks the same radio, and the form posts the same field. There is no
hidden input to keep in sync and nothing to resolve on the server.

With the script, typing asks `Search` and puts the matches where the tree was, each with the
ancestry it came from — a code found out of context does not say where it lives. Clearing the box
brings the tree back from memory, without asking again.

A tree of radios announces itself as a **group of choices**, not as a navigation: it is a form
field, and that is one role and not two.

:::warning
The value arriving at the server did not come from the tree — it came from a request. Check it
against the hierarchy (`validate:"required,..."` plus a rule of your own, as
[`examples/cadastro`](https://github.com/emersonjoe/trilha/tree/main/examples/cadastro) does):
the radio is what a person uses, not what an attacker is limited to.
:::

## ui.js

Everything by attribute, no initialization: `[data-ui-tabs]`, `[data-ui-dialog-open=id]`,
`[data-ui-dialog-close]`, `[data-ui-fade=ms]`, `[data-ui-show-when]`, `[data-ui-toast=text]`
(`data-ui-toast-kind`), `[data-ui-theme-toggle]`, `[data-ui-tooltip=text]`, `[popover].ui-menu`. It also exposes
`window.ui.toast(text, {kind, ms})`, `ui.fade(el)`, `ui.evalShowWhen(root)` and
`ui.applyTheme("dark"|"light")`. Elements inserted later (HTMX, fetch) need
`ui.evalShowWhen(el)`/`ui.fade(el)`/`ui.initTooltips(el)` if they use those attributes —
`ui.hydrate(el)` does the three at once.

## Fragments

`[data-trilha-target=id]` on an `<a>` or `<form>` (see `ui.Swap`) makes the kit request the
same URL with the `Trilha-Fragment` header and swap element `#id` for the HTML that comes
back. Details: the target gets `aria-busy` while it waits; **204 with `Trilha-Location`**
becomes a real navigation; **422** focuses the first `[aria-invalid=true]`, otherwise focus
(and the caret) return to the field in use; what came in is hydrated (`fade`, `show-when`)
and fires `trilha:swap` (`detail.target`, `detail.status`). On 5xx, a network error or a
fragment without the id, the kit gives up and navigates/submits normally.
`ui.swap(id, html, status)` and `ui.hydrate(el)` do the swap by hand (`ui.swap` returns a
promise: the replacement may be running inside a view transition). See
[Interactivity](/learn/interactivity).

### Waiting

| Symbol | What it does |
|---|---|
| `ui.Indicator(id)` | this element appears only while target `id` is waiting past the threshold |
| `ui.PendingAfter(ms)` | the threshold on the trigger; default 120 ms, zero or less means the default |
| `ui.NoTransition()` | no crossfade on this trigger |
| `ui.Spinner(attrs…)` | a turning ring sized by the font it sits in, hidden from assistive technology |

While a target waits, `data-trilha-pending` is on the target, the trigger and every indicator
of that target, and `aria-busy` is on the target; `trilha:pending` and `trilha:settled` fire
on `document` with `detail.target` and `detail.id`. A second trigger for a target already in
flight is ignored. The replacement runs inside `document.startViewTransition` where it exists
and where the system does not ask for less motion.

## Navigation

Client navigation is off until you ask for it, in two places:

| Symbol | Role |
|---|---|
| `ui.Navigate(id) h.Node` | marks a region: a click on a same-origin link inside it replaces element `#id` with the same element from the next page. An empty `id` means the marked element itself |
| `ui.NoNavigate() h.Node` | keeps one link out of it (a download, another app, a route that must reload) |
| `ui.NavigateScript(c) h.Node` | `<script defer src=ui.nav.js>`; put it once, in the layout of the area that uses it |

What the browser keeps doing: the address in the bar is the one a normal navigation would
use, Back and Forward work (and restore the scroll position of the entry they return to),
`Cmd`/`Ctrl`-click and middle click open a tab, `target`, `download` and links to another
origin are untouched. What the kit adds: `aria-busy` on the region while it waits, focus
moved to what came in, `ui.hydrate` and the `trilha:swap` event, and one request at a time —
a second click aborts the first. On 5xx, a network error, a redirect or a page that does not
contain the id, it gives up and navigates for real.

The behavior is a separate file so an app that does not use it does not download it, and
`ui.Head` does not load it. A link marked with `ui.Swap` stays with fragments: it asks for a
piece of the page, not for the next page.

## Upload with progress

A form that sends a file is a form: `method="post"`, `enctype="multipart/form-data"`, the
CSRF field. Three symbols add the progress bar on top of it, and it is off until you ask:

| Symbol | Role |
|---|---|
| `ui.UploadTo(id) h.Node` | on the `<form>`: send it with XHR and swap `#id` with what comes back |
| `ui.UploadBar(attrs…) h.Node` | the `<progress>` the kit fills in; hidden until the send starts |
| `ui.UploadScript(c) h.Node` | `<script defer src=ui.upload.js>`, once per page that uploads |

The request carries `Trilha-Fragment: id`, so the handler answers the piece with the same
`c.Fragment()` it already uses. While it uploads, the bar gets `value`/`max` from the
browser's own progress event (and loses `value` — an indeterminate bar — when the total is
not known), and a `trilha:upload` event bubbles with `detail: {loaded, total, form}`. On a
5xx, a network error or a piece without the id, the form submits for real: the user sees the
page reload, not a button that did nothing.

The attribute is `data-trilha-upload`, not `data-trilha-target`, so the fragment handler in
`ui.js` does not submit the same form a second time. The body limit is the server's business
— see [`AllowBody`](/reference/ctx).

`ui.Dropzone(ui.DropzoneOpts{Name, Accept, MaxSize, Single, Attrs})` puts a drop area over the
file input: a `<label>` that takes the drop and a `<ul class="ui-queue">` with one line per
file. With `ui.UploadTo` on the form the queue sends **one file per request**, so each line
gets its own progress and its own answer — a message that names `files[2]` has nowhere to go
when three files travel in one body. The swapped element must be outside the dropzone, or the
queue is destroyed halfway through.

`Accept` and `MaxSize` in the options only spare the user a round trip: the browser can be
told anything. What decides is [`c.Files`](/reference/ctx) with its `FileRules` — the same
rules, applied to bytes that already arrived. Without JavaScript the input is a plain
`multiple` field and the form posts every file at once, into the same handler.

## Combobox

A text field that searches a list is two inputs: the one the person types in, and the one the
form sends. `ui.Combobox` renders both — a visible `<input role=combobox name="<name>_q">` and
an `<input type=hidden name="<name>">` with the chosen value — plus the `<ul role=listbox>` of
options.

| Field of `ComboboxOpts` | Role |
|---|---|
| `Name` | name of the hidden field; the visible one is `Name + "_q"` |
| `Value` / `Label` | what was chosen and what is written for it (the round trip) |
| `Options []Option` | a short list: `ui.js` filters it in the browser, no request |
| `Source string` | the URL that searches; the answer is `ui.ComboboxOptions(...)` |
| `With []string` | other fields of the same form to carry in the query (`?uf=SP&q=camp`) |
| `MinChars`, `Debounce` | when to search (default 1 character, 200 ms) |
| `Placeholder`, `Required`, `Attrs` | as in `Input` |

`ComboboxOptions(items []T, of func(T) (value, label string))` is the answer of the search
route: only the `<li>`s, no envelope. The request carries `Trilha-Fragment`, so the route
answers with `c.HTML` and the page's layout stays out of it.

```go
func GET(c *trilha.Ctx) error {
	return c.HTML(200, ui.ComboboxOptions(
		Search(c.Query("uf"), c.Query("q")),
		func(city string) (string, string) { return city, city },
	))
}
```

Without JavaScript nothing breaks: the visible field is a normal text input, so the form still
arrives with `cidade_q` filled in and `cidade` empty. Resolving the typed text against the
list is the server's job — the same list the search route reads.

## Markdown

```go
func Markdown(src string, opt MarkdownOpts) h.Node
```

A model writes Markdown, and so does whoever types in a form. `Markdown` turns it into nodes:
paragraphs, emphasis, headings, lists, quotes, inline and fenced code, GFM tables, links and
line breaks.

```go
h.Div(ui.Markdown(doc.Summary, ui.MarkdownOpts{}))
```

**There is no raw HTML, and no way to turn it on.** A `<script>` in the text is a `<script>` on
the screen, as text — the return is a tree, not a string, so the escaping is not a rule anybody
has to remember. That is the whole reason this exists instead of an `h.Raw` around a converter.

| Field of `MarkdownOpts` | What it does |
|---|---|
| `HeadingBase` | the level `#` becomes (default 3, so it does not compete with the page's `<h1>`); `######` never goes past `<h6>` |
| `Images` | `![alt](url)` renders an `<img>`; off by default, and the URL is validated either way |
| `Class` | an extra class on the wrapper, which is always `ui-md` |

Links only survive as links when the address is `http`, `https`, `mailto`, or relative (`/`,
`#`, `./`); anything else — `javascript:`, `data:` — stays as the text it was. An external link
gets `rel="noopener nofollow ugc"`.

The site's own pages use a different renderer (`site/internal/md`): another dialect, over text
from this repository. `ui.Markdown` is for text nobody in this repository wrote.

## Chat

```go
func Chat(c *trilha.Ctx, o ChatOpts) h.Node
func ChatScript(c *trilha.Ctx) h.Node
func ChatHTML(text string) string
```

The conversation with an agent: the bubbles, the field and the button. The route on the other
side is [`ai.Serve`](/reference/ai#chat-over-http).

```go
ui.Chat(c, ui.ChatOpts{Action: "/api/chat", History: msgs, Greeting: "Ask me anything."})
ui.ChatScript(c)   // once, in the layout
```

| Field of `ChatOpts` | What it does |
|---|---|
| `Action` | the route that answers — the only one that has to be filled in |
| `History` | what was said before, oldest first. It is the app's: the framework keeps no session |
| `Greeting` | Markdown shown while the history is empty |
| `ID` | the element id and the prefix of the ids inside (default `chat`) |
| `Placeholder`, `Submit`, `Label` | the words on the screen |
| `MaxLength` | caps the field (default 4000; negative lifts it) |
| `Steps` | shows the tool the agent called and what came back |
| `Markdown` | the `MarkdownOpts` for the answers |

`ChatMessage{Role, Text}` is one turn. `Role: "assistant"` is rendered as Markdown;
`Role: "user"` is rendered as text — what somebody typed is never markup.

With `ChatScript` the answer arrives word by word and the Markdown is rendered when the message
ends: pass `ui.ChatHTML` to `ai.ServeOpts.HTML` and the finished bubble looks exactly like a
reloaded page. Without the script the form still submits and the route answers the whole thing
at once, so nothing on the screen depends on the script running.

## Formatting

A date, a size, a duration and a count are not domain: they are the same in every
application, and every application writes them again — usually four times, slightly differently
each time, and usually ignoring the time zone.

```go
// app/setup.go
func Config(cfg *trilha.Config) {
	cfg.Locale = "pt-BR"                  // "en" is the zero value
	cfg.TimeZone = "America/Sao_Paulo"    // empty means UTC
}
```

```go
ui.Date(c, doc.CreatedAt)                 // <time datetime="…">Sep 8, 2026 3:04 PM</time>
ui.Date(c, doc.CreatedAt, ui.Relative())  // 3min ago, absolute in the title
ui.Date(c, doc.CreatedAt, ui.DateOnly())  // Sep 8, 2026
ui.Bytes(c, doc.Size)                     // 1.4 MB      (pt-BR: 1,4 MB)
ui.Duration(c, job.Elapsed)               // 2 min 13 s
ui.Number(c, total)                       // 12,345      (pt-BR: 12.345)
ui.Number(c, price, ui.Decimals(2))       // 1,234.56
```

### The rules worth knowing

**A missing value is a dash.** The zero `time.Time`, a nil `*time.Time`, a zero size: all
render `—` in muted text. A zero date printing as `01/01/0001` is the bug this removes.

**`datetime` is always the instant.** The text is local and translated; the machine-readable
attribute is RFC 3339 in UTC, so a copy-paste, a sort or a screen reader gets the fact and not
the presentation.

**`Relative` does not move.** It writes "3min ago" and keeps the absolute time in the `title`.
Nothing updates it: the kit has no clock and does not want one. A screen that needs the number
to keep moving puts the piece in a `ui.Poll` — a decision the page makes and pays for, once.

**`Bytes` is base 10.** kB, MB, GB: what the file manager of whoever is reading already shows
them. The exact byte count stays in the `title`.

**An unknown time zone falls back to UTC and says so in the log.** Falling back in silence
would shift every timestamp on the screen and nothing would look broken.

### They take a Ctx, and that is on purpose

`ui.Date(c, t)` and not `ui.Date(t)`. A package-level language would be shared by two
applications running in one process — which is exactly what `trilha.Provide` and the embedded
app exist to support — and the second one to boot would silently change the first. `ui.Head`,
`ui.Flashes` and `ui.DataTable` take a `Ctx` for the same kind of reason.

### Money is not here

The currency, where the symbol goes, how a negative reads: those are the application's to
decide, and a framework that guessed would be wrong in somebody's country.
`ui.Number(c, v, ui.Decimals(2))` with the symbol written beside it is the whole recipe.

`trilha audit` warns about a `time.Format("02/01/2006")` inside `app/`: a layout in the page
ignores `Config.TimeZone`, which is how a date shown to somebody in another country ends up
simply wrong.

## Theme

`ui.theme.css` defines, in `:root` and `.dark`, exactly the shadcn/ui v4 variables:
`--background/--foreground`, `--card/--card-foreground`, `--popover/…`, `--primary/…`,
`--secondary/…`, `--muted/…`, `--accent/…`, `--destructive`, `--border`, `--input`, `--ring`,
`--chart-1…5`, `--sidebar…`, `--radius`. `ui.css` derives `--radius-sm/md/lg/xl`. Dark mode
is the `dark` class on `<html>` (the `ui.Head` script applies the saved or system
preference before the first paint).

## CLI

`trilha ui [--force] [--css-only|--js-only]` writes the six files in `public/`:
`ui.theme.css` is only created (never overwritten); `ui.css`, `ui.js`, `ui.nav.js`,
`ui.upload.js`, `ui.live.js`, `ui.chat.js` and `ui.island.js` are updated when they equal a previous version and, if you edited them, only
with `--force`.
