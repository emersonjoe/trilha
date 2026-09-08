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
	// One loader per response, with the request nonce, so the default CSP
	// (script-src 'self' 'nonce-…') accepts it without unsafe-inline.
	c.islandLoader = true
	return h.Fragment(el, h.Script(NonceAttr(c), h.Raw(islandLoader)))
}

// The loader is what mounts islands on a page that does not use the ui kit.
// It cannot be the only one: a fragment applied with outerHTML does not run the
// <script> it carries, so on a page that had no island the first one to arrive
// through a swap would sit there unmounted and silent. ui.js mounts what a swap
// brings in, and both sides skip an element already marked data-trilha-mounted,
// so an island mounts exactly once when both are present (spec 057, #82).
//
// islandLoaderMark is what identifies the loader in a rendered page.
const islandLoaderMark = "/*trilha-islands*/"

// islandLoader imports each island's module and mounts it once. It runs after
// the document is parsed, and again after a fragment swap (spec 018), so an
// island that arrives inside a fragment is not left dead.
const islandLoader = "/*trilha-islands*/" + `(()=>{
const tok=el=>el.getAttribute("data-trilha-csrf")||"";
class IslandError extends Error{constructor(status,detail,body){super(detail||("HTTP "+status));this.name="IslandError";this.status=status;this.detail=detail;this.body=body}}
class IslandInvalid extends IslandError{constructor(status,detail,body,fields){super(status,detail,body);this.name="IslandInvalid";this.fields=fields||{}}}
const api=(el,ac)=>{
const send=async(method,url,data)=>{
const h={"Accept":"application/json"};
if(data!==undefined)h["Content-Type"]="application/json";
const t=tok(el);if(t)h["X-CSRF-Token"]=t;
const res=await fetch(url,{method:method,headers:h,body:data===undefined?undefined:JSON.stringify(data),credentials:"same-origin",signal:ac.signal});
const loc=res.headers.get("Trilha-Location");if(loc){location.assign(loc);return null}
const ct=res.headers.get("Content-Type")||"";
const body=ct.includes("json")?await res.json().catch(()=>null):await res.text();
if(res.ok)return body;
const d=body&&typeof body==="object"?(body.detail||body.title||""):"";
if(res.status===422)throw new IslandInvalid(res.status,d,body,(body&&body.fields)||{});
throw new IslandError(res.status,d,body)};
return{csrf:()=>tok(el),signal:ac.signal,
get:url=>send("GET",url),post:(url,data)=>send("POST",url,data===undefined?{}:data),send:send,
swap:async(url,id)=>{const target=id||el.id;if(!target)throw new IslandError(0,"island.swap needs the id of the fragment");
const res=await fetch(url,{headers:{"Trilha-Fragment":target},credentials:"same-origin",signal:ac.signal});
const loc=res.headers.get("Trilha-Location");if(loc){location.assign(loc);return false}
const old=document.getElementById(target);if(!old)return false;
old.outerHTML=await res.text();const now=document.getElementById(target);if(!now)return false;
document.dispatchEvent(new CustomEvent("trilha:swap",{detail:{target:now,status:res.status}}));return true}}};
const m=r=>{r.querySelectorAll("[data-trilha-island]").forEach(el=>{
if(el.hasAttribute("data-trilha-mounted"))return;el.setAttribute("data-trilha-mounted","");
const s=el.getAttribute("data-trilha-island");let p=null;
try{p=JSON.parse(el.getAttribute("data-trilha-props")||"null")}catch(e){console.error("trilha: island props",s,e);return}
const ac=new AbortController();
const gone=new MutationObserver(()=>{if(!el.isConnected){ac.abort();gone.disconnect()}});
gone.observe(document,{childList:true,subtree:true});
import(s).then(mod=>{const f=mod.default;
if(typeof f!=="function"){console.error("trilha: island without a default export:",s);return}
f(el,p,api(el,ac))}).catch(e=>console.error("trilha: island",s,e))})};
const run=()=>m(document);
if(document.readyState==="loading")document.addEventListener("DOMContentLoaded",run);else run();
document.addEventListener("trilha:swap",e=>m((e.detail&&e.detail.target)||document))})()`
