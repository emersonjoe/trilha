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
| `ui.Asset(name) []byte` | embedded content of `ui.css`, `ui.theme.css`, `ui.js`, `ui.nav.js` or `ui.upload.js` |
| `ui.Files` | the five names, in the order `trilha ui` writes them |

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
| `Badge`, `Alert(title, ...)`, `AlertDescription(...)` | badge and alert (`role=alert`) |
| `Toaster(...)`, `Toast(kind, text, fadeMs)` | toast stack; `kind` = `""`, `success`, `error`; `fadeMs > 0` disappears on its own |
| `Table(...)`, `Num()`, `Depth(n)` | scrollable table; numeric cell; row indentation (tree) |
| `Tabs(id, Tab{Label, Content}...)` | accessible tabs (arrows, Home/End); the first starts open |
| `Dialog(id, title, ...)`, `DialogDescription(s)`, `DialogFooter(...)`, `DialogTrigger(id, ...)`, `DialogClose(...)` | native `<dialog>` with `showModal` |
| `Menu(id, ...)`, `MenuItem(...)`, `MenuLink(href, ...)`, `MenuTrigger(id, ...)` | menu with the native `popover` attribute |
| `Pagination(Pages{Page, Total, Href, Prev, Next, Label})` | page navigation as links; the current page is a `<span>` with `aria-current`, the edges are absent instead of disabled, and a window of seven slots keeps the first and last page with `…` over each gap; one page renders nothing |
| `Tooltip(text, ...)` | hint on what it wraps: `title` plus `data-ui-tooltip`, upgraded by `ui.js` into a bubble with `role=tooltip` and `aria-describedby` |
| `Separator, Skeleton, Progress(value, max), Breadcrumb(Crumb{Label, Href}...), Avatar(initials, src), Collapsible(summary, ...)` | miscellaneous |
| `ThemeToggle()` | button that switches light/dark (`localStorage["ui-theme"]`) |
| `Swap(id)` | `data-trilha-target`: the `<a>` or `<form>` asks for element `#id` only and swaps it (fragments) |
| `NoPush()` | `data-trilha-push="false"`: the swap leaves history alone |
| `Icon(name, attrs...)`, `Icons()` | inline Lucide SVG; unknown name → panic (programming error) |

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
`ui.swap(id, html, status)` and `ui.hydrate(el)` do the swap by hand. See
[Interactivity](/learn/interactivity).

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

## Theme

`ui.theme.css` defines, in `:root` and `.dark`, exactly the shadcn/ui v4 variables:
`--background/--foreground`, `--card/--card-foreground`, `--popover/…`, `--primary/…`,
`--secondary/…`, `--muted/…`, `--accent/…`, `--destructive`, `--border`, `--input`, `--ring`,
`--chart-1…5`, `--sidebar…`, `--radius`. `ui.css` derives `--radius-sm/md/lg/xl`. Dark mode
is the `dark` class on `<html>` (the `ui.Head` script applies the saved or system
preference before the first paint).

## CLI

`trilha ui [--force] [--css-only|--js-only]` writes the five files in `public/`:
`ui.theme.css` is only created (never overwritten); `ui.css`, `ui.js`, `ui.nav.js` and
`ui.upload.js` are updated when they equal a previous version and, if you edited them, only
with `--force`.
