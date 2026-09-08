---
title: From htmx and templ
description: What each piece of a Go + templ + htmx stack is called in Trilha, and what has no equivalent.
---

If you already write Go, render HTML on the server and swap pieces of the page, you have made
the two decisions this framework is built on. This page is not a tutorial — it is a
translation table, plus the honest list of what is missing.

## The table

| htmx + templ | Trilha |
|---|---|
| `.templ` files, compiled by `templ generate` | `h.Div(h.Class("card"), h.Text(title))` — plain Go functions, escaped by default, no build step and no second language |
| `templ.Component` as a parameter | `h.Node` as a parameter — same composition, same nesting |
| `@templ.Raw(...)` | `h.Raw(...)`, deliberately loud, same warning |
| `hx-get="/x" hx-target="#list"` | `ui.Swap("list")` on an ordinary `<a>` or `<form>`; the route answers the same URL and `c.Fragment()` tells the handler which piece was asked for |
| `hx-post` on a form | the form's own `method="post"` plus `ui.Swap` — the method stays where HTML puts it |
| `hx-indicator="#spinner"` | `ui.Indicator("list")`, with a threshold so a fast answer never blinks |
| `hx-disabled-elt` | automatic: a second trigger for a target already in flight is ignored |
| `hx-boost` | `ui.Navigate("id")` plus `ui.NavigateScript(c)` — opt-in per subtree, not per document |
| `hx-push-url="false"` | `ui.NoPush()` |
| `hx-confirm` | `ui.Confirm(title, description)` |
| `hx-trigger="every 2s"` | not yet — see below |
| `hx-swap-oob` | not yet — see below |
| Alpine `x-data` for local state | `c.Island("/thing.js", props, fallback)` — see [The ceiling](/learn/the-ceiling) |
| `templ.WithNonce` / CSP wiring by hand | on by default; `c.Nonce()` and `trilha.NonceAttr(c)` |
| CSRF middleware you chose and wired | on by default for form routes; `trilha.CSRFInput(c)` |
| `net/http` routing you wrote, or chi | folders under `app/`, checked by the compiler through a generated file |

## What has no equivalent, and why

**Out-of-band swaps (`hx-swap-oob`).** One response changing a second, unrelated element — the
cart badge in the header when you add an item — has no answer today. The workaround is to make
the swapped fragment contain both, or to swap the piece that holds them. It is a real gap, and
it is tracked rather than papered over.

**Polling triggers (`hx-trigger="every 2s"`).** There is no declarative poller. For live data
the framework's answer is server-sent events, which are already what the dev reload rides on;
for the general case you write the `setInterval` yourself in an island.

**A generic event trigger.** htmx will fire a request on any DOM event with any modifier
(`keyup changed delay:500ms`). Trilha triggers on what HTML already triggers on: a link click
and a form submit. Anything else is an island.

**Template hot reload without a rebuild.** `templ` can reload a template without recompiling
Go. Trilha rebuilds the binary — usually about a second on an example app — because the pages
*are* Go. You trade a little dev latency for the compiler checking every route.

## What you get that the stack did not have

- **Routes from folders, checked by the compiler.** `app/blog/[slug]/page.go` becomes a
  registered route in generated Go. A page whose function has the wrong signature is a
  compile error, not a 404 you find in staging.
- **`trilha check`** — one command that runs generate, `gofmt`, `vet`, tests, a security audit
  and the OpenAPI check, in that order, stopping at the first failure.
- **Validation and field errors in the handler that renders HTML.** `c.Bind(&in)` returns
  `trilha.FieldErrors`; `ui.Errors(errs, "email")` puts the message beside the input, and the
  same handler answers 422 with the form re-rendered.
- **A `ui` kit with no dependency** — around forty typed components over prefixed CSS, copied
  into your project so you can edit them.
- **One binary at the end**, with `public/` embedded, that does not need the CLI to run.

## What stays exactly the same

The instinct. Render on the server, send HTML, let the browser do what browsers do, and reach
for the client only where the client is genuinely the one who knows. Trilha does not ask you
to change that — it asks you to stop wiring it together by hand.

If you are migrating a real application, the [migration recipe](/cookbook/migration) covers a
Next.js project; the shape of this move is smaller, because the hard decision is already made.

## Challenge

You are porting a screen where htmx did `hx-get="/search" hx-target="#results"
hx-trigger="keyup changed delay:300ms"`. Trilha has no event trigger. Port it anyway, keeping
the search working for whoever has no JavaScript.

:::solution
The form is the trigger, and the debounce is the only thing left to write:

```go
h.Form(h.Method("get"), h.Action("/search"), ui.Swap("results"),
    ui.Input(h.Name("q"), h.Value(q)),
    ui.Submit(h.Text("Search")),
    ui.Spinner(ui.Indicator("results")),
)
h.Div(h.ID("results"), rows())
```

```js
let t;
document.addEventListener("input", (e) => {
  const field = e.target.closest('form[data-trilha-target] input[name=q]');
  if (!field) return;
  clearTimeout(t);
  t = setTimeout(() => field.form.requestSubmit(), 300);
});
```

Three things came for free. The URL still carries `?q=`, because it is a `GET` form, so the
result is linkable and Back works. The submit button is still there, so the screen works with
the script blocked. And `ui.Indicator` only shows the spinner if the answer takes longer than
the threshold, which on a local search is never — the flicker htmx users tune `delay:` to
avoid.
:::
