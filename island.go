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
// element and the props already parsed:
//
//	export default function (el, props) { ... }
//
// props is anything encoding/json can serialize, or nil. What the server sends
// is data, never markup: it is escaped as an attribute and read back with
// JSON.parse. The children are the fallback, so the page works with the script
// blocked, failing to load, or still on its way.
func (c *Ctx) Island(src string, props any, children ...h.Node) h.Node {
	attrs := []h.Node{h.Data("trilha-island", c.Asset(src))}
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
	// One loader per response, with the request nonce, so the default CSP
	// (script-src 'self' 'nonce-…') accepts it without unsafe-inline.
	c.islandLoader = true
	return h.Fragment(el, h.Script(NonceAttr(c), h.Raw(islandLoader)))
}

// islandLoaderMark is what identifies the loader in a rendered page.
const islandLoaderMark = "/*trilha-islands*/"

// islandLoader imports each island's module and mounts it once. It runs after
// the document is parsed, and again after a fragment swap (spec 018), so an
// island that arrives inside a fragment is not left dead.
const islandLoader = `/*trilha-islands*/(()=>{const m=r=>{r.querySelectorAll("[data-trilha-island]").forEach(el=>{` +
	`if(el.hasAttribute("data-trilha-mounted"))return;el.setAttribute("data-trilha-mounted","");` +
	`const s=el.getAttribute("data-trilha-island");let p=null;` +
	`try{p=JSON.parse(el.getAttribute("data-trilha-props")||"null")}catch(e){console.error("trilha: island props",s,e);return}` +
	`import(s).then(mod=>{const f=mod.default;` +
	`if(typeof f!=="function"){console.error("trilha: island without a default export:",s);return}` +
	`f(el,p)}).catch(e=>console.error("trilha: island",s,e))})};` +
	`const run=()=>m(document);` +
	`if(document.readyState==="loading")document.addEventListener("DOMContentLoaded",run);else run();` +
	`document.addEventListener("trilha:swap",e=>m((e.detail&&e.detail.target)||document))})()`
