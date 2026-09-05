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
| `ui.Asset(name) []byte` | embedded content of `ui.css`, `ui.theme.css` or `ui.js` |
| `ui.Files` | the three names, in the order `trilha ui` writes them |

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
| `Separator, Skeleton, Progress(value, max), Breadcrumb(Crumb{Label, Href}...), Avatar(initials, src), Collapsible(summary, ...)` | miscellaneous |
| `ThemeToggle()` | button that switches light/dark (`localStorage["ui-theme"]`) |
| `Icon(name, attrs...)`, `Icons()` | inline Lucide SVG; unknown name → panic (programming error) |

## ui.js

Everything by attribute, no initialization: `[data-ui-tabs]`, `[data-ui-dialog-open=id]`,
`[data-ui-dialog-close]`, `[data-ui-fade=ms]`, `[data-ui-show-when]`, `[data-ui-toast=text]`
(`data-ui-toast-kind`), `[data-ui-theme-toggle]`, `[popover].ui-menu`. It also exposes
`window.ui.toast(text, {kind, ms})`, `ui.fade(el)`, `ui.evalShowWhen(root)` and
`ui.applyTheme("dark"|"light")`. Elements inserted later (HTMX, fetch) need
`ui.evalShowWhen(el)`/`ui.fade(el)` if they use those attributes.

## Theme

`ui.theme.css` defines, in `:root` and `.dark`, exactly the shadcn/ui v4 variables:
`--background/--foreground`, `--card/--card-foreground`, `--popover/…`, `--primary/…`,
`--secondary/…`, `--muted/…`, `--accent/…`, `--destructive`, `--border`, `--input`, `--ring`,
`--chart-1…5`, `--sidebar…`, `--radius`. `ui.css` derives `--radius-sm/md/lg/xl`. Dark mode
is the `dark` class on `<html>` (the `ui.Head` script applies the saved or system
preference before the first paint).

## CLI

`trilha ui [--force] [--css-only|--js-only]` writes the three files in `public/`:
`ui.theme.css` is only created (never overwritten); `ui.css` and `ui.js` are updated when
they equal a previous version and, if you edited them, only with `--force`.
