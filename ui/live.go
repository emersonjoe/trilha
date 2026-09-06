package ui

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Poll asks the route for this fragment again every so often, and swaps it —
// the screen of a job that runs on the server without anybody clicking. The
// element needs an id: it is the id the fragment is asked for.
//
//	h.Div(h.ID("status"), ui.Poll("6s", "/docs/42/status"), status(doc))
//
// The interval is "6s", "500ms" or "2m". src is where to ask; the address of
// the page itself when it is empty. The client pauses while the tab is hidden
// and asks again as soon as it comes back, backs off to a minute after an
// error and returns to the interval on the first answer, and obeys what the
// route says: Ctx.PollStop ends it, Ctx.PollEvery changes the rhythm.
//
// The first render of the fragment is already on the page, so a visitor with
// no JavaScript sees the state as it was when the page loaded — old, never
// broken. Load the behavior with LiveScript.
func Poll(every, src string) h.Node {
	n := []h.Node{h.Data("trilha-poll", every)}
	if src != "" {
		n = append(n, h.Data("trilha-src", src))
	}
	return h.Attrs(n...)
}

// Live opens one Server-Sent Events connection for the page, to the route that
// says what changed. Put it once, in the layout or at the top of the page:
//
//	h.Body(ui.Live("/events"), ...)
//
// The connection carries names, never HTML: each name wakes up the fragments
// marked with On, which ask their route again. The route is an ordinary GET
// that calls Ctx.Stream and Stream.Notify.
func Live(src string) h.Node { return h.Data("trilha-live", src) }

// On swaps this fragment when the stream announces that name. The element
// needs an id, like Poll, and Poll is the fallback: a fragment with both waits
// for the event while the connection is up and goes back to the clock when it
// is down.
//
//	h.Div(h.ID("status"), ui.On("doc:42", "/docs/42/status"), ui.Poll("30s", ""), status(doc))
func On(event, src string) h.Node {
	n := []h.Node{h.Data("trilha-on", event)}
	if src != "" {
		n = append(n, h.Data("trilha-src", src))
	}
	return h.Attrs(n...)
}

// LiveScript loads ui.live.js, the behavior behind Poll, Live and On. Put it
// once, in the layout of the area that uses them — the kit's Head does not
// load it, so a page with nothing to watch does not download it.
func LiveScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.live.js")), h.Defer())
}
