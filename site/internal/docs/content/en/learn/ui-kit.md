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
	ui.Flashes(c),              // where toasts show up, c.Flash included
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
"Saved!", 4000)` inside the layout's toaster. The `examples/blog` app does both in
`app/blog/novo/page.go`.

## Saying what happened, and asking before destroying

A `POST` that works ends in a redirect, and the redirect eats the news. `c.Flash` writes it
in a signed cookie, and the `ui.Flashes(c)` in the layout shows it on the page that follows:

```go
c.Flash(ui.FlashSuccess, "Post deleted")
return c.Redirect("/blog")
```

`ui.FlashInfo`, `ui.FlashSuccess` and `ui.FlashError` are the kinds. On a fragment answer
there is no redirect to survive, so the messages travel in a header and `ui.js` shows them —
the call in the handler is the same. Without `TRILHA_SECRET` nothing is written, and the app
says so once in the log.

Before something irreversible, `ui.Confirm` puts the question on the form itself:

```go
h.Form(h.Method("post"), h.Action("/blog/"+p.Slug), trilha.CSRFInput(c),
	ui.Confirm("Delete this post?", "There is no undo."),
	ui.Submit(ui.Destructive(), h.Text("Delete")))
```

`ui.js` holds the submit, opens the kit's dialog and only then lets it through. Without
JavaScript the form submits straight away; when that is not good enough, ask on a page of
its own (`GET /blog/{slug}/delete` rendering the same form), which works either way.

@demo ui-confirmar

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

## The frame of an internal app

`ui.Shell` is the sidebar, the top bar and the user's menu, written once. The item whose
`Href` is the longest prefix of `Current` gets `aria-current="page"` — an exact match always
wins, so `/items/42/edit` lights up `/items`, not `/`. `ui.PageHeader` is the title of the
screen inside it, with the way back and the actions of the screen. See
[Shell](/reference/shell) for `Hide`, the collapsing sidebar and `IconNode`.

@demo ui-shell

## Where you are, and who is signed in

`ui.Breadcrumb` renders the trail as real links, with the current page as a `<span
aria-current="page">` instead of a link to itself. `ui.Avatar` falls back to initials when
there is no picture.

@demo ui-breadcrumb

A menu that opens a small list of actions is `ui.MenuTrigger` and `ui.Menu` sharing an `id`,
on the browser's own `popover` attribute — no script of its own.

@demo ui-menu

## More content behind one click

`ui.Collapsible` is a styled `<details>`: no script decides whether it is open, the browser
already does.

@demo ui-colapsavel

`ui.Tabs` works the same wherever it appears — arrows and Home/End move the selection, the
first tab starts open:

@demo ui-abas

## The boxes layout is built from

`ui.Row`, `ui.Stack` and `ui.Grid` are `<div>`s with one class each — a row, a column, a
responsive grid — and `ui.Separator` is the rule between sections of a screen.

@demo ui-grade

## Before the data arrives

`ui.Skeleton` is the placeholder shape; `ui.Progress` is a bar at a known position. Neither
needs a script — `ui.Defer`, further down, is what swaps a skeleton for the real thing.

@demo ui-carregamento

## A key, and a snippet

`ui.Kbd` and `ui.Code` sit inline with the sentence around them.

@demo ui-tipografia

## One value of an enum, as a badge

`ui.Status(enum, value)` reads the label and the tone a `trilha.Enum` declared once — the
same declaration a `<select>`'s options and a form's validation already use. A value the
enum no longer knows renders muted, not blank.

@demo ui-status

## Nothing to show, and what could not load

`ui.Empty` is the screen with nothing on it: an icon, a title, a hint that says what to do
next, and a way out. `ui.EmptyError` is the same shape for a screen that failed to load — the
error itself only appears in development, never on the page a visitor sees.

@demo ui-vazio

## A form in several screens

`ui.Steps` draws where somebody is: what is behind them links back, what is ahead is plain
text, and the current step carries `aria-current="step"`. The state between screens is
`Ctx.Draft` — see [the wizard recipe](/cookbook/wizard).

@demo ui-etapas

## The slow part, a moment later

A dashboard that needs seven queries should not hold the whole page for the one that takes
two seconds. `ui.Defer` renders a placeholder now and asks for the fragment once the page has
loaded; see [Live fragments](/reference/live) for the route it expects on the other end.

@demo ui-atraso

## A file next to its metadata

`ui.Preview` shows an image as an `<img>` you can open at full size, a document a browser
renders inline, or — when the type cannot be shown in place — a card with a download button
instead of a frame that renders blank.

@demo ui-preview

## What happens during a swap

`ui.Indicator(id)` marks anything — a badge, a spinner — to appear only while the target with
that `id` has been waiting past `ui.PendingAfter`'s threshold (120 ms by default), and
`ui.NoTransition()` turns off the crossfade for a trigger that fires often, like a live
search.

@demo ui-espera

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
