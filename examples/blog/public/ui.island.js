/* trilha ui 3b481f6fd4665ad2 */
// Trilha ui kit — the island runtime. Loaded by Ctx.Island, once per page.
//
// This is the only implementation of mounting an island and of the island
// object handed to it. It used to exist twice — a minified copy inline in
// island.go for the page without the kit, and a readable one in ui.js for the
// island that arrives inside a swapped fragment — and the two had already
// drifted: only the ui.js copy restored focus and hydrated what came back.
// A file loaded with <script src> is reachable from both places, needs no
// nonce (script-src 'self' covers it) and is cached like any other asset.
(() => {
  if (window.__trilhaIslands) return; // one runtime per page, however it arrived
  window.__trilhaIslands = true;

  const token = (el) => el.getAttribute("data-trilha-csrf") || "";

  class IslandError extends Error {
    constructor(status, detail, body) {
      super(detail || "HTTP " + status);
      this.name = "IslandError";
      this.status = status;
      this.body = body;
    }
  }
  class IslandInvalid extends IslandError {
    constructor(status, detail, body, fields) {
      super(status, detail, body);
      this.name = "IslandInvalid";
      this.fields = fields;
    }
  }

  // api is the way back to the server. It carries the CSRF token the response
  // wrote on the element, because the double-submit cookie is HttpOnly on
  // purpose, and it turns a 422 into the same field errors a form would show.
  const api = (el, ac) => {
    const send = async (method, url, data) => {
      const h = { Accept: "application/json" };
      if (data !== undefined) h["Content-Type"] = "application/json";
      const t = token(el);
      if (t) h["X-CSRF-Token"] = t;
      const res = await fetch(url, {
        method, headers: h, credentials: "same-origin", signal: ac.signal,
        body: data === undefined ? undefined : JSON.stringify(data),
      });
      const loc = res.headers.get("Trilha-Location");
      if (loc) { location.assign(loc); return null; }
      const ct = res.headers.get("Content-Type") || "";
      const body = ct.includes("json") ? await res.json().catch(() => null) : await res.text();
      if (res.ok) return body;
      const detail = body && typeof body === "object" ? (body.detail || body.title || "") : "";
      if (res.status === 422) throw new IslandInvalid(res.status, detail, body, (body && body.fields) || {});
      throw new IslandError(res.status, detail, body);
    };
    return {
      csrf: () => token(el),
      signal: ac.signal,
      send,
      get: (url) => send("GET", url),
      post: (url, data) => send("POST", url, data === undefined ? {} : data),
      // swap goes through the kit when the page has it, so the fragment is
      // hydrated and the focus kept the way every other swap is. Without the
      // kit the element is replaced and the event still fires, which is what
      // mounts whatever island came inside it.
      swap: async (url, id) => {
        const target = id || el.id;
        if (!target) throw new IslandError(0, "island.swap needs the id of the fragment");
        const res = await fetch(url, {
          headers: { "Trilha-Fragment": target }, credentials: "same-origin", signal: ac.signal,
        });
        const loc = res.headers.get("Trilha-Location");
        if (loc) { location.assign(loc); return false; }
        const html = await res.text();
        if (window.ui && window.ui.swap) return window.ui.swap(target, html, res.status);
        const old = document.getElementById(target);
        if (!old) return false;
        old.outerHTML = html;
        const now = document.getElementById(target);
        if (!now) return false;
        document.dispatchEvent(new CustomEvent("trilha:swap", { detail: { target: now, status: res.status } }));
        return true;
      },
    };
  };

  // mount imports each island's module once. The mark is what keeps an element
  // from mounting twice when this runs again after a swap.
  const mount = (root) => {
    const found = Array.from(root.querySelectorAll("[data-trilha-island]"));
    if (root.matches && root.matches("[data-trilha-island]")) found.push(root);
    for (const el of found) {
      if (el.hasAttribute("data-trilha-mounted")) continue;
      el.setAttribute("data-trilha-mounted", "");
      const src = el.getAttribute("data-trilha-island");
      let props = null;
      try { props = JSON.parse(el.getAttribute("data-trilha-props") || "null"); }
      catch (e) { console.error("trilha: island props", src, e); continue; }
      // The signal is aborted when the element leaves the page, so an island
      // inside a fragment that gets swapped stops writing to what is gone.
      const ac = new AbortController();
      const gone = new MutationObserver(() => {
        if (!el.isConnected) { ac.abort(); gone.disconnect(); }
      });
      gone.observe(document, { childList: true, subtree: true });
      import(src)
        .then((mod) => {
          if (typeof mod.default !== "function") {
            console.error("trilha: island without a default export:", src);
            return;
          }
          mod.default(el, props, api(el, ac));
        })
        .catch((e) => console.error("trilha: island", src, e));
    }
  };

  window.trilhaIslands = mount; // the kit calls this after it swaps

  const run = () => mount(document);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", run);
  else run();
  document.addEventListener("trilha:swap", (e) => mount((e.detail && e.detail.target) || document));
})();
