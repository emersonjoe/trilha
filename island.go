package trilha

import (
	"encoding/json"

	"github.com/emersonjoe/trilha/h"
)

// Island renders an interactive region of a page that stays static: the server
// sends the fallback HTML, and a module in public/ takes over on the client.
// There is no global hydration and no bundler — src is a file in public/,
// addressed through Asset so it gets the content hash:
//
//	c.Island("/editor.js", map[string]any{"wpm": 200},
//		h.Class("editor"), ui.Textarea(h.Name("corpo")))
//
// The module's default export is the mount function, called once with the
// element, the props already parsed, and the island object:
//
//	export default function (el, props, island) { ... }
//
// island is the way back to the server: island.post(url, data) sends JSON with
// the CSRF token already on it and gives back what the route answered, island.get
// reads, island.swap(url, id) replaces a fragment the way a link with a target
// does, island.csrf() is the token, and island.signal is aborted when the
// element leaves the page — so an island inside a fragment that gets swapped
// stops writing to what is no longer there.
//
// props is anything encoding/json can serialize, or nil. What the server sends
// is data, never markup: it is escaped as an attribute and read back with
// JSON.parse. The children are the fallback, so the page works with the script
// blocked, failing to load, or still on its way.
func (c *Ctx) Island(src string, props any, children ...h.Node) h.Node {
	// The double-submit cookie is HttpOnly on purpose, so the token reaches the
	// island the way it reaches a form: written into the page. It is the same
	// token CSRFInput puts in every form of this response.
	attrs := []h.Node{h.Data("trilha-island", c.Asset(src)), h.Data("trilha-csrf", c.CSRFToken())}
	if props != nil {
		data, err := json.Marshal(props)
		if err != nil {
			// Props that do not serialize are a mistake in the page, and the
			// page is not the place to die for it: the fallback is already
			// good HTML.
			c.app.warnOnce("island:"+src, "trilha: island props are not JSON; the island was not mounted",
				"island", src, "path", c.r.URL.Path, "error", err)
			return h.Div(children...)
		}
		attrs = append(attrs, h.Data("trilha-props", string(data)))
	}
	el := h.Div(append(attrs, children...)...)
	if c.islandLoader {
		return el
	}
	// One runtime per response. It is a file and not an inline script for two
	// reasons: a <script src> needs no nonce (script-src 'self' already covers
	// it), and a file is reachable from the kit as well — which is what keeps
	// the mounting and the island object from existing twice and drifting
	// apart, as they did (spec 060).
	c.islandLoader = true
	return h.Fragment(el, h.Script(h.Data("trilha-islands", ""), h.Src(c.Asset(IslandRuntime)), h.Defer()))
}

// IslandRuntime is the kit file that mounts islands and builds the island
// object. Ctx.Island points a script tag at it; `trilha ui` writes it into
// public/ along with the rest of the kit, and `trilha check` says so when a
// project uses an island without it.
const IslandRuntime = "/ui.island.js"
