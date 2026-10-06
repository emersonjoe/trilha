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
**204 with the `Trilha-Location` header**. Post/Redirect/Get survives.

What the kit does with it depends on the page. On a page with a `ui.Navigate` region — an app
that already navigates in place — it **follows the redirect in place**: it fetches the
destination, swaps the region, puts its address in the bar as a new entry, and shows the
`c.Flash` in a toast, once. Back returns to the form by `GET`; it never posts again. Elsewhere
it loads the destination for real, as before. `ui.Follow()` on a trigger (or on a form around
several) turns following on without a region — the same target is asked again at the new
address — and `ui.NoFollow()` turns it off. `ui.UploadTo` follows the same way.

When the destination has another frame — login, logout, switching organization or language,
anything that changes the header and the menu outside the region — answer
`c.RedirectReload("/…")`: the same 303 without JavaScript, and a real load with it. A
destination that does not have the region loads whole too, and the message comes along.

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

A fragment answered in the `POST` saves the second round trip, and leaves the bar where it
was. Say the address of what you drew, and the kit puts it there:

```go
doc := documentos.Criar(in)
c.PushURL("/documentos/" + doc.ID)        // a new history entry
return c.Render(200, painel(c, doc))
```

`c.ReplaceURL(path)` puts it on the entry the trigger made instead of a new one — a link to
`?aba=marcos&x=1` whose canonical address is `?aba=marcos` — and `c.ReplaceURL("")` leaves the
bar alone. The contract is the PRG's: the address is a `GET` that draws this screen, so
reload, Back and a shared link show it again. Both take a path of this site, like `Redirect`,
and write nothing on a full page. A `GET` form with `ui.Swap` replaces its entry by default
(ten letters typed are not ten pages to go back through); `ui.PushHistory()` gives it one
entry per search, so Back undoes the filter.

## While the server answers

A swap that takes 40 ms should leave the page exactly as it was; one that takes two seconds
should say so. `ui.Indicator` marks the element that appears while a target is waiting, and it
only appears once the request has been in the air past a threshold — 120 ms by default:

```go
h.Form(h.Method("get"), ui.Swap("lista"),
    ui.Input(h.Name("q")),
    ui.Submit(h.Text("Search")),
    ui.Spinner(ui.Indicator("lista")),   // hidden until the wait is worth mentioning
)
h.Div(h.ID("lista"), rows())
```

The threshold is the point. Showing a spinner for every answer is the flicker people write
CSS to hide; showing it only for the slow ones is information. `ui.PendingAfter(300)` on the
trigger changes it, and several indicators may watch the same target — a spinner beside the
button, a bar in the header.

While a target waits, three elements carry `data-trilha-pending`: the target, the trigger and
every indicator of that target. The target also gets `aria-busy`, so a screen reader is told
without any styling of yours. `trilha:pending` and `trilha:settled` fire on `document` for
anything the CSS cannot do.

A second write — `POST`, `PUT`, `PATCH`, `DELETE` — on a target that already has one in flight
is ignored, so the save does not go out twice. A second *read* is the newer intent and wins:
the `GET` in the air is aborted, so clicking "2" and then "3" on a slow pager shows page 3,
with page 3 in the address bar and one new history entry. The waiting marks stay on until the
last request lands. Nothing to wire for either.

Where the browser has `startViewTransition`, the replacement crossfades instead of jumping;
where it does not, or where the system asks for less motion, nothing changes.
`ui.NoTransition()` turns it off on one trigger.

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

Just before the old element goes, `trilha:before-swap` fires with `e.detail.target` (still on
the page), `e.detail.id` and, for a navigation, `e.detail.url` — the place to stop a timer or
an observer your script started. Mount in `trilha:swap`, filtering by `detail.target`; take
down in `trilha:before-swap`; anything with state of its own is an island.

The scripts that come inside the new element run, the way they would on a full page: each
same-origin `<script src>` once per URL per document — an island's runtime, `ui.LiveScript`,
a file of your app. An inline `<script>` that came in a response never runs: executing HTML
from a response is how XSS happens, and the CSP would refuse it anyway. Put the code in a
file under `public/`.

`window.ui.swap(id, html, status)`, `window.ui.hydrate(el)` and `window.ui.activate(el)` (the
scripts above) are exposed for whoever needs to do the swap by hand.

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
  page are mounted, each one once. What mounts them is `public/ui.island.js`, a kit file
  linked with a `<script src>` — there is no inline island script, so `script-src 'self'` is
  all the CSP needs, and the runtime is cached like any other asset. A project that uses an
  island without that file gets a critical from `trilha check`, because the failure is
  otherwise silent: the fallback shows and nothing else happens.
- **An island that arrives inside a fragment or a client navigation mounts too.** The
  runtime listens for `trilha:swap`; when it is not on the page yet, the kit runs the tag
  that came with the new content, because a `<script>` written by `outerHTML` never runs.

### The escape hatch

The island is the boundary where another library is allowed in, and where its cost stops.
Web Components need nothing from here — `customElements.define` and the tag is the island.
For Alpine, htmx or anything else, drop the file in `public/` and import it from the island's
module; for React, an ESM build in `public/` and a `createRoot(el)` inside the mount. The
page around it is not asked to become a component, and nothing else in the project learns
about the choice.

The default CSP is `script-src 'self'`, so a module from a CDN is refused until you widen
it — a decision, not an accident.

An island is also where you go when the swap model runs out — dragging, collaborative
editing, anything whose truth lives in the browser while the person is acting.
[The ceiling](/learn/the-ceiling) is about recognising that moment, and about the rules that
keep an island from quietly growing into a SPA.

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

A form inside the region navigates in place too, with no change to its route. A `GET` (a
filter, a search) is a link with a query: a new address, and Back returns to the previous
filter. A `POST` goes as the browser would send it — no fragment header — so the route answers
what it answers without JavaScript: its 303 is followed in the same request, the destination
is swapped in, its address goes to the bar and its `c.Flash` becomes a toast; its 422 page is
swapped in with the focus on the first invalid field and the address left alone. Back never
posts again. A response that is not HTML, or is a download, or a destination without the
region, is the browser's: the form submits for real. `ui.NoNavigate()` on a form keeps it
out; forms with `ui.Swap` or `ui.UploadTo` are their own scripts' business. A link to an
address that redirects is one `GET` as well, and the message of the destination is shown.

The browser keeps its habits — Back and Forward work and restore the scroll position of the
entry they return to, `Cmd`-click opens a tab, `target` and `download` are untouched. The
kit adds `aria-busy` while it waits, runs the scripts the new region brings and fires
`trilha:swap`, so an island, a `ui.Defer` or a `ui.Poll` inside the new page works. A second
click cancels the first request. A redirect is followed in the same request and its
destination's address goes to the bar; a 5xx, a response that is not HTML or a page without
that id gives up and navigates for real.

A full page load is announced by the screen reader; a swap is not, so the kit says it: the
focus goes to the first `h1` of the new region, and a visually hidden live region,
`#trilha-route-announcer`, reads the new `<title>` (or the heading, when the title did not
change) — and stays quiet when the heading the focus reached already said it. Back and
Forward announce the same way. `ui.NavigateFocus("region")` beside `ui.Navigate` keeps the
focus on the region instead, and `"none"` leaves it where it was.

### Asking before the click

A round trip of 200 ms still shows when the click starts it. `ui.Prefetch()` beside
`ui.Navigate` (or on one link) asks for the next page as soon as the person shows the intent
of opening it — the pointer resting on the link for 80 ms, the keyboard's focus reaching it, a
finger touching it — and the click finds the answer there, or still on its way and shared.
Never by viewport, which is where useless traffic lives; never on a `ui.NoPrefetch()` link, a
download, another origin, or a connection in data-saver or 2G. An answer waits 10 seconds for
its click (`ui.PrefetchTTL(ms)` changes it), eight at most; a redirect or anything but a `200`
page is not kept, and the click asks for itself. `trilha:swap` says `detail.prefetched`, so
the app can measure what it gains.

The request says `Purpose: prefetch`, and `c.IsPrefetch()` reads it — and the browser's own
`Sec-Purpose`. A route whose reading has a side effect that may be skipped (a view counter)
checks it. The click uses the prefetched answer without asking again, so a reading that must
be recorded every time — an access log the law asks for — keeps its links out with
`ui.NoPrefetch()`.

### A panel beside the page

"The document open beside the list, the next one a click away" is a `ui.Sheet`: a panel whose
body loads on demand and that stays open while the region navigates.

```go
// app/layout.go — outside the ui.Navigate region
ui.Sheet(c, "leitor", ui.SheetOpts{Title: "Document"}), ui.SheetScript(c),

// the list, inside it
h.A(h.Href("/documentos/"+d.ID), ui.SheetOpen("leitor"), h.Text(d.Nome))

// app/documentos/id_/page.go — one route, the piece and the page
if c.Fragment() == "leitor-body" {
	return ui.SheetBody("leitor", leitor(c, doc)), nil
}
return h.Div(ui.H1(h.Text(doc.Nome)), leitor(c, doc)), nil
```

The link opens the panel, marks its body pending and asks its own address for the fragment
`leitor-body`; another link swaps only the body, and the address does not change — a panel is a
moment (`SheetOpts.Push` makes it an entry, and Back closes it). The panel is outside the
region, so a `ui.Preview` inside it does not reload while the list beside it navigates.
Closing it empties the body, which is what frees a heavy frame. The focus goes to the panel's
title; Escape and the close button give it back to the link, whose `aria-expanded` follows.
Not modal, it does not trap the focus — it is complementary content, an `<aside>`;
`SheetOpts.Modal` makes it a `<dialog>`. Below 768px it covers the screen and closes the
shell's drawer. Without JavaScript the link goes to the document's page; `SheetOpts.Open`
renders the panel open, for a list that reads `?painel=…`.

A dialog loads its body the same way: `ui.DialogTrigger("ver", ui.Swap("ver-body"),
h.Href("/documentos/7"))` is a link that opens `ui.Dialog("ver", …)` and swaps `#ver-body`
in, without touching the address.

The rule of thumb: **fragment** when a handler answers a piece, **navigation** when the
answer is a page and the frame around it should stay.

## The file, and the bar that says how far it got

Sending a file is the one place where "the screen blinks" is not the problem — the problem is
that nothing happens for thirty seconds. The browser knows how far the upload got; it just
has no way to say so from a plain form submit.

```go
// app/anexos/page.go
h.Form(h.Method("post"), h.Action("/anexos"), h.Enctype("multipart/form-data"),
	ui.UploadTo("lista"),
	trilha.CSRFInput(c),
	ui.Field("arquivo", "File", ui.Input(h.ID("arquivo"), h.Name("arquivo"), h.Type("file"), h.Required())),
	ui.UploadBar(),
	ui.Submit(h.Text("Send")),
)
```

`ui.UploadTo(id)` sends the form with XHR and swaps `#id` with the answer; `ui.UploadBar()`
is the `<progress>` the kit fills in from the browser's own progress event; and
`ui.UploadScript(c)` loads the behavior — its own file again, so a page without an upload
does not download it. With JavaScript off, none of that exists and the form is what it always
was: it posts, the server answers, the page reloads.

On the server there is no new API. The request carries `Trilha-Fragment`, so the same handler
that renders the page answers the piece:

```go
func POST(c *trilha.Ctx) error {
	if err := c.FormErr(); err != nil {
		return err
	}
	f, hdr, err := c.Request().FormFile("arquivo")
	if err != nil {
		return err
	}
	defer f.Close()
	anexos.Add(hdr.Filename, hdr.Size)
	if c.Fragment() != "" {
		return c.Render(200, lista()) // the piece, with the same id
	}
	return c.Redirect("/anexos") // no JavaScript: Post/Redirect/Get
}
```

### The limit is the app's; the exception is the route's

A body is capped at `Config.MaxBodyBytes` (1 MiB by default) — that cap is what keeps one
request from eating the server's memory, and it should stay where it is for every route that
receives a form. The route that receives files says so for itself, in its `middleware.go`:

```go
// app/anexos/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	if c.Request().Method == "POST" {
		c.AllowBody(8 << 20)
		c.NoReadDeadline() // a slow connection is not an error
	}
	return next()
}
```

In the middleware, not in the handler: CSRF parses the form before the handler runs, so by
then the body has already been read under the old limit. Everything else in the app keeps the
1 MiB, and going over 8 MiB is still a 413 with the usual message.

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
