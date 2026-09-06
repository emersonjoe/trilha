---
title: UI kit
description: Trilha's default component kit, compatible with shadcn/ui themes, and how it becomes yours to customize.
---

Every project created with `trilha new` ships with the `ui` kit: typed components in Go
(`ui.Button`, `ui.Card`, `ui.Field`...) that render classes from a small, prefixed CSS
(`ui-*`), plus 200 lines of JavaScript for what HTML does not do on its own (tabs,
disappearing toasts, conditional fields, light/dark theme). No dependencies: the three files
live in `public/` and are yours.

```text
public/ui.theme.css   ← colors and radius: edit it or paste a ready-made theme
public/ui.css         ← the components; `trilha ui` updates it
public/ui.js          ← behaviors; `trilha ui` updates it
```

The theme contract is the one from [shadcn/ui](https://ui.shadcn.com) (MIT): the same
variables, `--background`, `--primary`, `--radius`, in `oklch`. Generate a theme at
ui.shadcn.com/themes or tweakcn.com, paste the `:root { … } .dark { … }` block into
`ui.theme.css` and you are done: nothing in Go changes. Trilha uses neither React nor
Tailwind; only the theme is compatible.

## Wiring the kit

The generated layout already does this; in an existing project, run `trilha ui` and add:

```go
h.Head(…, ui.Head(c)),          // ui.theme.css, ui.css, saved theme, ui.js
h.Body(ui.Body(),               // theme font and colors
	ui.Header(ui.Brand("/", "My app"), ui.Nav(ui.NavLink("/", "Home", true)), ui.Spacer(), ui.ThemeToggle()),
	h.Main(ui.Container(children)),
	ui.Toaster(),               // where toasts show up
)
```

## Variants are attributes

A component is a function returning `h.Node`; variants and sizes are class attributes you
mix with any `h` attribute, in any order. `h` merges repeated `class` attributes into one.

@demo ui-botoes

## Forms

`ui.Field` joins label, control, help and error with the right `id`/`for` and `aria-*`.
`ui.ShowWhen("field", "value")` shows the group only while the field has that value and
**disables the hidden controls**, so they do not travel in the `POST`. Without JavaScript,
all fields simply appear.

@demo ui-formulario

After a `POST`, render the error in the field itself (`ui.Error("Title is required")` +
`ui.Invalid()` on the control) and a toast that disappears on its own: `ui.Toast("success",
"Saved!", 4000)` inside the layout's `ui.Toaster()`. The `examples/blog` app does both in
`app/blog/novo/page.go`.

## Cards, tabs, progress

@demo ui-card

## Dialog and toasts

`ui.Dialog` is a native `<dialog>`: it closes with Esc, a click outside or `ui.DialogClose`;
the form inside it does a normal `POST`.

@demo ui-dialogo

## Tables with hierarchy

`ui.Depth(n)` indents the first cell: it serves charts of accounts, category trees and any
server-rendered *drill-down*. `ui.Num()` aligns numbers to the right.

@demo ui-tabela

## Pagination and hints

`ui.Pagination` renders page navigation as real links, so a page can be shared, reloaded and
indexed. The current page is a `<span>` with `aria-current` — a link to where you already are
is a link to nowhere — and the first page has no *previous*, so nothing is rendered for it.
The window keeps the first page, the last one and the ones around the current, with an
ellipsis over each gap, so the footer does not grow with the table.

`ui.Tooltip` writes the hint into `title`, which is the browser's own tooltip and works with
`ui.js` off. With the script on the page the `title` is removed — two tooltips is worse than
none — a bubble with `role="tooltip"` takes its place, the target gets `aria-describedby`,
and the hint answers to hover, keyboard focus and touch, closing with Escape.

@demo ui-paginacao

:::note
The hint is a string on purpose. A hint with a link inside is a popover, and that is what
`ui.Menu` is for.
:::

## Updating and customizing

- `trilha ui` rewrites `ui.css` and `ui.js` when you update Trilha; it never touches
  `ui.theme.css`. If you edited `ui.css`, it warns and only overwrites with `--force`.
- To change a component, edit `ui.css` (it is yours) or override it in `style.css`. For a new
  component, write the function in your own package: `func Price(v int) h.Node { return
  h.Span(h.Class("ui-badge price"), …) }`.
- Icons: `ui.Icon("check")`, a small set from [Lucide](https://lucide.dev) (ISC).
  `ui.Icons()` lists the names. For others, paste the SVG into your own `h.Raw`.

## Challenge

Build a sign-up form where the "Company" field only appears when "Type" is "Company" and,
when submitted empty, the error shows in the field and a toast disappears after 3 s.

:::solution
```go
func Page(c *trilha.Ctx) (h.Node, error) {
	msg := c.Query("error")
	return h.Form(h.Method("post"), h.Class("ui-stack"), trilha.CSRFInput(c),
		ui.Field("type", "Type", ui.Select(h.ID("type"), h.Name("type"),
			h.Option(h.Value("individual"), h.Text("Individual")), h.Option(h.Value("company"), h.Text("Company")))),
		ui.Field("company", "Company", ui.Input(h.ID("company"), h.Name("company"), h.If(msg != "", ui.Invalid())),
			ui.Error(msg), ui.With(ui.ShowWhen("type", "company"))),
		ui.Submit(h.Text("Sign up")),
		h.If(msg != "", ui.Toaster(ui.Toast("error", msg, 3000))),
	), nil
}

func POST(c *trilha.Ctx) error {
	if c.Form("type") == "company" && strings.TrimSpace(c.Form("company")) == "" {
		return c.Redirect("/signup?error=Company+is+required")
	}
	return c.Redirect("/signup/done")
}
```
:::
