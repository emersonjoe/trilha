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

## What this is not

It is not a SPA. There is no client router, no shared state, no component hydration and no
DOM diffing — the swap is `outerHTML`, and the source of truth is still the server. A screen
that needs rich local state (an editor, a canvas) deserves its own JavaScript; the fragment
solves the common case, which is most screens.

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
