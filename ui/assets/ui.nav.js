// Kit ui do Trilha — navegação no cliente. Carregado só por `ui.NavigateScript`.
(() => {
  if ("scrollRestoration" in history) history.scrollRestoration = "manual";
  let ctl;

  // fetchInto asks the server for the whole page and swaps one element of it.
  // The address is the same one a normal navigation would use, so reloading it
  // gives the same page — this is a shortcut, not a second truth.
  //
  // The waiting marks, the crossfade and the scripts a region brings come from
  // ui.js, which the kit always loads before this file: one threshold, one
  // transition, one behaviour — a second copy here would drift.
  const ui = () => window.ui || {};
  const pending = (id, trigger) => ui().pending?.(id, trigger) || (() => {});
  const update = (fn, trigger) => ui().update?.(fn, trigger) || Promise.resolve(fn());

  // A full page load is announced by the screen reader; a swap is not, so the
  // new title goes to a live region born here (#297). The focus goes to the
  // heading — or where data-trilha-nav-focus says — and the announcer stays
  // quiet when the heading the focus reached already says the same.
  const announce = (text) => {
    let el = document.getElementById("trilha-route-announcer");
    if (!el) {
      el = document.createElement("div");
      el.id = "trilha-route-announcer";
      el.className = "ui-sr";
      el.setAttribute("aria-live", "assertive");
      el.setAttribute("aria-atomic", "true");
      document.body.appendChild(el);
    }
    el.textContent = text;
  };
  const arrive = (next, title, changed) => {
    const mode = next.closest("[data-trilha-nav-focus]")?.getAttribute("data-trilha-nav-focus") || "h1";
    const h1 = next.querySelector("h1");
    const say = (changed ? title : h1?.textContent || title).trim();
    const to = mode === "none" ? null : mode === "h1" && h1 ? h1 : next;
    if (to) {
      if (!to.hasAttribute("tabindex")) to.setAttribute("tabindex", "-1");
      to.focus({ preventScroll: true });
    }
    announce(to === h1 && h1.textContent.trim() === say ? "" : say);
  };

  // fetchInto resolves "swapped", "skipped" (a newer click owns the page) or
  // "navigate", like ui.js's ask: only "swapped" may touch the history.
  const fetchInto = async (url, id, y, trigger) => {
    const old = document.getElementById(id);
    if (!old) return "navigate";
    ctl?.abort();
    const me = (ctl = new AbortController());
    const settle = pending(id, trigger);
    try {
      const res = await fetch(url, { credentials: "same-origin", signal: me.signal, headers: { Accept: "text/html" } });
      if (res.redirected) { location.assign(res.url); return "skipped"; }
      if (res.status >= 500) return "navigate";
      const doc = new DOMParser().parseFromString(await res.text(), "text/html");
      const next = doc.getElementById(id);
      if (!next) return "navigate"; // the page is shaped differently: navigate for real
      return await update(() => {
        const cur = document.getElementById(id);
        if (me.signal.aborted || !cur) return "skipped";
        const changed = doc.title && doc.title !== document.title;
        ui().beforeSwap?.(cur, id, url);
        cur.replaceWith(next);
        if (doc.title) document.title = doc.title;
        ui().hydrate?.(next);
        ui().activate?.(next);
        arrive(next, document.title, changed);
        document.dispatchEvent(new CustomEvent("trilha:swap", { detail: { target: next, status: res.status, url } }));
        scrollTo(0, y || 0);
        return "swapped";
      }, trigger);
    } catch (e) {
      return e.name === "AbortError" ? "skipped" : "navigate";
    } finally {
      settle();
    }
  };

  document.addEventListener("click", (e) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = e.target.closest("a[href]");
    if (!a || a.hasAttribute("download") || a.hasAttribute("data-trilha-target")) return;
    if (a.target && a.target !== "_self") return;
    const holder = a.closest("[data-trilha-nav]");
    const mark = holder?.getAttribute("data-trilha-nav");
    if (!holder || mark === "false") return;
    const id = mark || holder.id;
    const url = new URL(a.href, location.href);
    if (url.origin !== location.origin || !document.getElementById(id)) return;
    // Same page with a hash: that is the browser's job.
    if (url.hash && url.pathname === location.pathname && url.search === location.search) return;
    e.preventDefault();
    // Mark the entry we are leaving, so Back knows how to rebuild it.
    history.replaceState({ trilhaNav: id, y: scrollY }, "");
    fetchInto(url.href, id, 0, a).then((r) => {
      if (r === "navigate") location.assign(url.href);
      else if (r === "swapped") history.pushState({ trilhaNav: id, y: 0 }, "", url.href);
    });
  });

  // Back and forward rebuild the page from the server, at the scroll position
  // the entry was left in.
  addEventListener("popstate", (e) => {
    const id = e.state?.trilhaNav;
    if (!id) return;
    fetchInto(location.href, id, e.state.y).then((r) => { if (r === "navigate") location.reload(); });
  });
})();
