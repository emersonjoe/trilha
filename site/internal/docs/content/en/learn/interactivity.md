---
title: Interactivity
description: Swap one piece of the page and submit a form without a reload, from the same handler that serves the whole page.
---

A Trilha page is a whole document: the browser navigates, the server answers, the screen
blinks. That works well, but not on every screen — filtering a list or saving a form should
not cost a reload.

The way out here is the **fragment**: the same link and the same form as always, with one
extra attribute. With JavaScript on, the `ui` kit asks for the page, the server answers with
just that piece, and the browser swaps that element. With JavaScript off, the link navigates
and the form submits — the server answers with the whole page, because nobody asked for a
fragment. No new route, no new handler, no dependency.

## One more question in the handler

`c.Fragment()` returns the id the client wants to swap, or `""` on a normal navigation:

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Clientes")
	return tela(c, c.Query("q")), nil
}

// tela is the whole page when there is no fragment, and the piece when there is:
// the element being swapped must carry the same id.
func tela(c *trilha.Ctx, q string) h.Node {
	return h.Div(h.ID("lista"),
		h.Form(h.Method("get"), h.Action("/clientes"), ui.Swap("lista"),
			ui.Input(h.Name("q"), h.Value(q)),
			ui.Submit(h.Text("Buscar")),
		),
		lista(clientes.Buscar(q)),
	)
}
```

When the request carries the `Trilha-Fragment` header, Trilha:

- **skips the route's layouts** (no `<html>`, no `<head>`, no navigation bar);
- writes only the nodes you returned, with no document envelope and no dev server script;
- answers with `Vary: Trilha-Fragment`, so a cache does not keep the piece in place of the
  page.

Everything else stays the same: middleware runs, CSRF is checked, the status is the one you
sent. `c.Fragment()` is just a question.

## The link and the form

In the HTML, `ui.Swap("id")` marks who takes part:

```go
ui.ButtonLink("/clientes?pagina=2", ui.Swap("lista"), h.Text("Next"))

h.Form(h.Method("post"), h.Action("/clientes"), ui.Swap("tela"),
	trilha.CSRFInput(c),
	// fields…
)
```

`ui.js` intercepts the click (left button only, no Ctrl/Cmd, same origin) and the submit,
does a `fetch` with the header, and swaps the element for the HTML that came back. While it
waits, the target gets `aria-busy="true"` (the kit's CSS dims the block and shows the
progress cursor). `ui.NoPush()` on a link keeps history untouched.

## After the POST

A `POST` that redirects keeps redirecting — inside a fragment too. Since `fetch` would
follow the 303 on its own and bring the new page back as a piece, Trilha answers
**204 with the `Trilha-Location` header**, and `ui.js` navigates for real. Post/Redirect/Get
survives.

When staying on the same screen makes more sense, answer with the updated piece:

```go
func POST(c *trilha.Ctx) error {
	in, errs := ler(c)
	if len(errs) > 0 {
		return c.Render(422, tela(c, in, errs, "")) // the form with its errors
	}
	clientes.Criar(in)
	if c.Fragment() != "" {
		return c.Render(200, tela(c, clientes.Cliente{}, nil, "Cadastro salvo!"))
	}
	return c.Redirect("/clientes?ok=1")
}
```

On **422** `ui.js` focuses the first field with `aria-invalid="true"` — what the browser
would do by itself on a reload. Otherwise it gives focus (and the caret position) back to
the field in use, looking it up by `id` or by `name`.

## When the fragment does not work out

The kit **never leaves the screen stuck**: if the answer is 5xx, if the network drops or if
the piece comes back without the expected id, it gives up and does the real navigation — the
link becomes `location`, the form becomes `form.submit()`. The user sees the page reload;
they do not see a click that did nothing.

## After the swap

New elements arrive hydrated: `[data-ui-fade]` and `[data-ui-show-when]` work again on their
own. If you have behavior of your own, listen for the event:

```js
document.addEventListener("trilha:swap", (e) => {
  // e.detail.target = the new element, e.detail.status = the response status
});
```

`window.ui.swap(id, html, status)` and `window.ui.hydrate(el)` are exposed for whoever needs
to do the swap by hand.

## The island: what a fragment cannot do

A fragment always comes from the server. An editor with a live preview, a canvas, a map that
drags: the state is on the client and there is no round trip to make. That is an **island** —
a piece of the page that brings its own module, with everything around it staying plain HTML.

```go
c.Island("/editor.js", map[string]any{"wpm": 200},
	h.Class("editor"),
	ui.Textarea(h.Name("corpo")),               // the fallback: still a form field
	h.P(h.Data("info", ""), h.Hidden()),        // filled in by the module
)
```

```html
<div data-trilha-island="/editor.js?v=9c1f" data-trilha-props="{&quot;wpm&quot;:200}" class="editor">…</div>
```

The module is an ordinary ES module in `public/`, and its default export is the mount:

```js
export default function (el, props) {
  const area = el.querySelector("textarea");
  area.addEventListener("input", () => { /* … */ });
}
```

Four things fall out of that shape:

- **The children are the fallback, and the server renders them.** Script blocked, still on
  the way, or 404: the page is what it always was. The island adds, it does not carry.
- **The props are data.** They are escaped as an attribute and read back with `JSON.parse` —
  a value from the database cannot become markup. Anything `encoding/json` serializes goes;
  what does not serialize warns in the log and leaves the fallback alone.
- **No bundler and no global hydration.** The module is a file in `public/`, addressed
  through `Asset` (so the URL carries the content hash), and only the islands present on the
  page are mounted, each one once. The loader is a single inline script with the request
  nonce, which is why the default CSP accepts it without `unsafe-inline`.
- **An island that arrives inside a fragment mounts too**: the loader listens for
  `trilha:swap`. What it needs is to be on the page already — that is, the page rendered at
  least one island of its own.

### The escape hatch

The island is the boundary where another library is allowed in, and where its cost stops.
Web Components need nothing from here — `customElements.define` and the tag is the island.
For Alpine, htmx or anything else, drop the file in `public/` and import it from the island's
module; for React, an ESM build in `public/` and a `createRoot(el)` inside the mount. The
page around it is not asked to become a component, and nothing else in the project learns
about the choice.

The default CSP is `script-src 'self'`, so a module from a CDN is refused until you widen
it — a decision, not an accident.

## The whole page, without the reload

A fragment swaps a piece of the page a handler chose. Navigation is the other half: the
next page is a *different* page, and what should not blink is everything around it — the
header, the sidebar, the scroll position of a long list.

```go
// app/painel-/layout.go
return h.Section(h.Class("app"), ui.Navigate("conteudo"), ui.NavigateScript(c),
    ui.Sidebar(ui.Nav(
        ui.NavLink("/painel", "Dashboard", cur == "/painel"),
        ui.NavLink("/relatorio", "Report", cur == "/relatorio"),
    )),
    h.Div(h.Class("app-content"), children),
), nil
```

`ui.Navigate(id)` marks a region: a click on a same-origin link inside it fetches the next
page and replaces `#id` with the same element from it. `ui.NavigateScript(c)` loads the
behavior — a separate file from `ui.js`, so an app that does not navigate this way does not
download it. Nothing changes on the server: `/relatorio` is the same route, answering the
same document. Reloading, opening in another tab, or arriving with JavaScript off gives the
same page.

Off by default, and off per link:

```go
ui.ButtonLink("/relatorio.pdf", ui.NoNavigate(), h.Text("Download"))
```

The browser keeps its habits — Back and Forward work and restore the scroll position of the
entry they return to, `Cmd`-click opens a tab, `target` and `download` are untouched. The
kit adds `aria-busy` while it waits, moves focus to what came in, and fires `trilha:swap`,
so an island inside the new page mounts. A second click cancels the first request; a 5xx, a
redirect or a page without that id gives up and navigates for real.

The rule of thumb: **fragment** when a handler answers a piece, **navigation** when the
answer is a page and the frame around it should stay.

## What this is not

It is not a SPA. There is no client router, no shared state, no component hydration and no
DOM diffing — the swap is `outerHTML`, and the source of truth is still the server. A screen
that needs rich local state (an editor, a canvas) deserves its own JavaScript, and the island
above is where that JavaScript goes; the fragment solves the common case, which is most
screens.

Worth remembering the security boundary: `Trilha-Fragment` is a custom header, so a
third-party site cannot send it without a preflight — and Trilha answers no preflight. A
fragment only ever goes out to your own origin.

The `examples/cadastro` app uses both: a search that filters the list and a form that saves
without reloading, both working with JavaScript turned off.

## Challenge

Make the list swap as the user types, without waiting for the button — and without firing a
request per keystroke.

:::solution
```js
let t;
document.addEventListener("input", (e) => {
  const campo = e.target.closest("form[data-trilha-target] input[name=q]");
  if (!campo) return;
  clearTimeout(t);
  t = setTimeout(() => campo.form.requestSubmit(), 250);
});
```
`requestSubmit()` fires the same `submit` event the kit already listens for, so
`data-trilha-target` still applies — and the form keeps working on the button click for
whoever has no JavaScript.
:::
