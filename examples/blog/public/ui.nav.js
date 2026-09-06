// Kit ui do Trilha — navegação no cliente. Carregado só por `ui.NavigateScript`.
(() => {
  if ("scrollRestoration" in history) history.scrollRestoration = "manual";
  let ctl;

  // fetchInto asks the server for the whole page and swaps one element of it.
  // The address is the same one a normal navigation would use, so reloading it
  // gives the same page — this is a shortcut, not a second truth.
  const fetchInto = async (url, id, y) => {
    const old = document.getElementById(id);
    if (!old) return false;
    ctl?.abort();
    ctl = new AbortController();
    old.setAttribute("aria-busy", "true");
    try {
      const res = await fetch(url, { credentials: "same-origin", signal: ctl.signal, headers: { Accept: "text/html" } });
      if (res.redirected) { location.assign(res.url); return true; }
      if (res.status >= 500) return false;
      const doc = new DOMParser().parseFromString(await res.text(), "text/html");
      const next = doc.getElementById(id);
      if (!next) return false; // the page is shaped differently: navigate for real
      old.replaceWith(next);
      if (doc.title) document.title = doc.title;
      next.setAttribute("tabindex", "-1");
      next.focus({ preventScroll: true });
      window.ui?.hydrate?.(next);
      document.dispatchEvent(new CustomEvent("trilha:swap", { detail: { target: next, status: res.status, url } }));
      scrollTo(0, y || 0);
      return true;
    } catch (e) {
      return e.name === "AbortError"; // a newer click owns the page now
    } finally {
      old.removeAttribute("aria-busy");
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
    fetchInto(url.href, id, 0).then((ok) => {
      if (!ok) { location.assign(url.href); return; }
      history.pushState({ trilhaNav: id, y: 0 }, "", url.href);
    });
  });

  // Back and forward rebuild the page from the server, at the scroll position
  // the entry was left in.
  addEventListener("popstate", (e) => {
    const id = e.state?.trilhaNav;
    if (!id) return;
    fetchInto(location.href, id, e.state.y).then((ok) => { if (!ok) location.reload(); });
  });
})();
