---
title: The ceiling
description: Where server-rendered swaps stop being the right tool, and how to cross without turning the app into a SPA.
---

Every framework has a ceiling. The ones worth trusting say where it is.

Trilha's model — the server renders HTML, a link or a form asks for one piece, the browser
swaps that element — covers most of an application. Not all of it. This page is about the
part it does not cover, how to recognise it before you are three days into fighting it, and
what to do when you get there.

## What the swap model does cover

If the screen changes because **something happened on the server**, a fragment is the whole
answer: a filtered list, a form that validates, a row that gets deleted, a panel that opens
with data in it, a table that pages. The state lives in one place, the URL still means what
it says, and there is no store to keep in step with the DOM.

That is the 90%, and it is worth defending: a swap has no hydration step, so it has no
hydration bugs.

## Where the ceiling is

The ceiling is not about how *complicated* a screen looks. It is about **who owns the truth
while the person is interacting**.

You are above the ceiling when, even for a moment, the browser is the only one who knows what
is on screen:

- **A continuous gesture.** Dragging to reorder, resizing a pane, drawing on a canvas,
  cropping an image. Between pointer-down and pointer-up there is no round trip that fits.
- **State that has to survive a swap.** A half-typed comment, a scroll position inside a
  virtualised list, an open combobox with a filter typed into it.
- **Two people at once.** Collaborative editing, live cursors, anything where a second
  client's change has to land without stepping on what you are doing.
- **A rhythm the network cannot keep.** Sixty frames a second, audio, a game loop.

A short test: *if I disabled the network right now, would the screen still have to respond?*
If yes, that part belongs to the client.

## Crossing it: an island

An island is a piece of the page that a JavaScript module takes over, with props the server
computed and HTML the server already rendered as the fallback. Everything around it stays
server-rendered.

```go
c.Island("/order.js", map[string]any{"order": slugs},
    h.Input(h.Type("hidden"), h.Name("order"), h.Value(strings.Join(slugs, ","))),
    h.Ol(h.Data("list", ""), rows...),
)
```

```js
// public/order.js — a module, no bundler, no build step
export default function (el, props) {
  const list = el.querySelector("[data-list]");
  const field = el.querySelector('input[name="order"]');
  // dragging happens here, in the browser, where it has to
  const save = () => { field.value = rows().map((li) => li.dataset.slug).join(","); };
}
```

The whole example — dragging, the keyboard path, the handler that saves — is
[`examples/blog`, route `/blog/ordem`](https://github.com/emersonjoe/trilha/tree/main/examples/blog/app/blog/ordem).

## The rules that keep an island from growing into a SPA

**The server still owns the data.** The island writes into a form field and stops. What saves
the order is the same `POST → redirect → GET` the rest of the app uses — not a private
`fetch` to a private endpoint with a private shape. The moment an island starts doing its own
persistence you have two sources of truth and a synchronisation bug waiting.

**The props are the starting truth.** The server renders the order; the island receives it.
A module that loads late, or not at all, cannot leave the page showing something that was
never true.

**The fallback is real HTML, not a spinner.** The children of `c.Island` are what the page
looks like without the module. If that is a blank box, the island is not an enhancement, it
is a requirement with extra steps.

**Do not take the keyboard away.** Dragging is not reachable from a keyboard. The example
keeps ↑ ↓ buttons that post one step through the same handler, and the island leaves them
alone. An island that trades accessibility for polish is a downgrade.

**One island, one job.** Three islands that each own a small thing are easier to delete than
one that owns the screen.

## Islands and swaps compose

An island mounts when it enters the document, whether it came with the whole page or arrived
inside a swapped fragment. You can swap a panel that contains an island and it will start;
you can swap it away and the module goes with it.

> Before 0.40.0 this was only true when the page already had an island. An island that
> arrived in a fragment on a page that had none stayed unmounted and silent — the loader
> travelled with it as a `<script>`, and the DOM does not run a script inserted that way.

## When an island is not enough either

Be honest with the shape of your app. If **most** screens are above the ceiling — a diagram
editor, a spreadsheet, a DAW — then you are not writing an app with a few interactive parts,
you are writing a client application, and a client framework will fit it better than a pile
of islands.

Trilha is still useful there: serve the shell, the authentication, the settings pages and the
API from Trilha, and give the client app its own route. What you should not do is grow an
island until it is a framework nobody chose.

## What this costs, honestly

An island is JavaScript you own: no types shared with the server, no compiler checking that
`props.order` exists, and a second place where a bug can live. That is the price of crossing
the ceiling, and it is why the ceiling is worth naming — so you pay it on the screens that
need it and not on the other ninety.

## Challenge

The reorder island writes the whole order into a hidden field, so nothing is saved until the
person presses the button — and if they wander off, the dragging is lost. Make the order save
by itself, a moment after the dragging stops, without giving the island its own endpoint and
without breaking the button for whoever has no JavaScript.

:::solution
```js
let t;
const salvaSozinho = () => {
  clearTimeout(t);
  t = setTimeout(() => campo.form.requestSubmit(), 800);
};
lista.addEventListener("dragend", salvaSozinho);
```
`requestSubmit()` sends the same form, through the same handler, with the CSRF token that is
already in it — so there is no second endpoint and no second shape to keep in step. Add
`ui.Swap("lista")` to the form and the answer comes back as a fragment instead of a redirect,
which is what keeps the page from jumping under the pointer.

The delay matters for the same reason the pending threshold does: `dragend` fires on every
drop, and a person reordering five rows drops five times. Waiting until they stop turns five
requests into one.
:::
