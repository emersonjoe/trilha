---
title: Live fragments
description: ui.Poll refreshes a fragment on a clock; ui.Live and ui.On refresh it when the server says so.
---

An app that processes things in the background — a document pipeline, a batch, a flow run
— shows state that changes with nobody clicking. Trilha already had the two halves:
`ui.Swap` replaces a fragment when the visitor clicks, and `c.Stream()` sends Server-Sent
Events to whoever writes the JavaScript. This is the glue, and it is the same glue every
screen was writing by hand.

Everything here needs `ui.LiveScript(c)` on the page — one `<script defer>` for
`ui.live.js`, which `ui.Head` does **not** load. A page with nothing to watch does not
download it.

## Polling

```go
h.Div(h.ID("status"), ui.Poll("6s", "/docs/42/status"), status(doc))
```

`ui.Poll(every, src)` are two attributes on an element that has an `id`: the id is the
fragment it asks for. `every` is `"6s"`, `"500ms"` or `"2m"`; `src` is where to ask, or
the address of the page itself when it is empty.

The client:

- **pauses while the tab is hidden** and asks again as soon as it comes back;
- **backs off** after an error — twice the interval each time, up to a minute — and goes
  back to the interval on the first answer;
- **respects `Retry-After`** on a 429 or a 5xx;
- **stops** on a 4xx that is not 429: a fragment that answers 403 or 404 is not going to
  start working on the next tick;
- **never replaces a fragment the visitor is typing into.**

The first render is already on the page, so a visitor with no JavaScript sees the state as
it was when the page loaded — old, never broken.

### The server owns the clock

| Call | Header | Effect |
|---|---|---|
| `c.PollStop()` | `Trilha-Poll: stop` | the polling ends; the answer is still the fragment, so the last state is what stays |
| `c.PollEvery(d)` | `Trilha-Poll: 30s` | the interval changes from here on; never under a second |

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() == "queue" {
		if queue.Done() {
			c.PollStop()
		}
		return queueBlock(), nil
	}
	...
}
```

`c.Fragment()` stays the only API on the server side: the route does not know whether the
request came from a click or from a tick.

## Loading later

A dashboard that needs seven queries to draw should not hold the whole page for the one that
takes two seconds.

```go
ui.Container(
	ui.Grid(stats...),
	ui.Defer(c, "insights", "/panel/insights", ui.DeferOpts{Height: "12rem"}),
)
```

`ui.Defer` renders a placeholder now and asks for the fragment as soon as the page has loaded —
once, with no clock, carrying the session and the headers of the page it sits in. `src` is an
ordinary route that answers `c.Fragment()`, the same one `ui.Poll` would ask:

```go
func Page(c *trilha.Ctx) (h.Node, error) {
	if c.Fragment() != "" {
		return block(c), nil
	}
	return h.Div(ui.H1(h.Text("Insights")), block(c)), nil
}
```

The id appears in both places for the same reason it does with `ui.Poll`: it is the element
being replaced, so the route's answer has to carry it. The route also answers as a whole page
when nobody sent the header — which is exactly where the placeholder's `<noscript>` link goes.
Without JavaScript the slow part is one click away instead of missing.

| Option | What it decides |
|---|---|
| `Height` | how tall the default skeleton is; the page must not jump when the content lands |
| `Placeholder` | something other than a skeleton — a card outline, a last known value |
| `Then` | what the fragment does after it arrives: `Then: ui.Poll("30s", src)` loads now and watches from then on |
| `Load`, `Error`, `Retry` | the three sentences, if the kit's own (in the app's language) are not what you want |

A fragment that fails shows a message and a **try again** in the hole instead of a skeleton
pulsing for ever. Both sentences are rendered on the server, so the behaviour never invents
text and never has to know a language.

:::note
`Defer` is `Poll`'s machinery with the clock left out, so `ui.LiveScript(c)` is what turns it
on — and one `Defer` per part of the page, not one per row of a list. A page that defers
twenty fragments made twenty requests to render itself; that is the SPA it was avoiding.
:::

## Events

One connection per page, opened by `ui.Live`:

```go
h.Body(ui.Live("/events"), ...)
```

and the fragments that care say what they are waiting for:

```go
h.Div(h.ID("status"), ui.On("doc:42", "/docs/42/status"), status(doc))
```

The route is an ordinary GET:

```go
func GET(c *trilha.Ctx) error {
	s := c.Stream()
	for ev := range bus.Subscribe(c.Context(), user) {
		if err := s.Notify(ev.Name); err != nil {
			return err
		}
	}
	return nil
}
```

`Stream.Notify(name)` sends the name and no data. **The event carries the name, never the
HTML**: the client asks the fragment's route again, so the authorization and the rendering
stay where they already are, and the connection never becomes a channel for data.

A fragment with both `ui.On` and `ui.Poll` waits for the event while the connection is up
and goes back to the clock when it is down — the fallback costs one extra attribute.

There is no bus in the framework: the route that answers `/events` is yours, and so is
whatever wakes it up. What the framework brings is the client's half of the protocol and
`Notify`.

## Security

The stream stays open for as long as the page does, which makes an unguarded one a
connection anyone can hold and read. Put the route behind the same guard as the pages it
serves — a `middleware.go` with `auth.Require()` over the branch — and `trilha audit`
will say so when there is none.
