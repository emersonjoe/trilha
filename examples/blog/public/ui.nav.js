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

  // The destination's layout drew its flashes (ui.Flashes) outside the
  // region; they are what the person has to see, so they come along as toasts
  // — or wait for the page that loads, when the swap gives up.
  const flashesOf = (doc, next) => [...doc.querySelectorAll(".ui-toast")].filter((t) => !next?.contains(t))
    .map((t) => ({ t: t.textContent.trim(), k: [...t.classList].find((c) => c.startsWith("ui-toast-"))?.slice(9) || "" }));

  // fetchInto resolves {r, res, doc}: r is "swapped", "skipped" (a newer click
  // owns the page) or "navigate", like ui.js's ask, and only "swapped" may
  // touch the history. init is the fetch of a form (#292); a write is never
  // aborted by a newer click, it only loses the region to it.
  let gen = 0;
  const fetchInto = async (url, id, y, trigger, init = {}) => {
    if (!document.getElementById(id)) return { r: "navigate" };
    ctl?.abort();
    const me = new AbortController(), mine = ++gen;
    ctl = (init.method || "GET") === "GET" ? me : null;
    const settle = pending(id, trigger);
    let res, doc;
    try {
      res = await fetch(url, { credentials: "same-origin", signal: me.signal, ...init, headers: { Accept: "text/html" } });
      const html = /text\/html/.test(res.headers.get("Content-Type")) && !/attachment/i.test(res.headers.get("Content-Disposition"));
      if (res.status >= 500 || !html || !sameOrigin(res.url)) return { r: "navigate", res };
      doc = new DOMParser().parseFromString(await res.text(), "text/html");
      const next = doc.getElementById(id);
      if (!next) return { r: "navigate", res, doc }; // the page is shaped differently: navigate for real
      const r = await update(() => {
        const cur = document.getElementById(id);
        if (me.signal.aborted || mine !== gen || !cur) return "skipped";
        const changed = doc.title && doc.title !== document.title;
        ui().beforeSwap?.(cur, id, res.url);
        cur.replaceWith(next);
        if (doc.title) document.title = doc.title;
        ui().hydrate?.(next);
        ui().activate?.(next);
        const bad = res.status >= 400 && next.querySelector("[aria-invalid='true']");
        if (bad) bad.focus();
        else arrive(next, document.title, changed);
        document.dispatchEvent(new CustomEvent("trilha:swap", { detail: { target: next, status: res.status, url: res.url } }));
        scrollTo(0, y || 0);
        return "swapped";
      }, trigger);
      if (r === "swapped") flashesOf(doc, next).forEach((f) => ui().toast?.(f.t, { kind: f.k, ms: 5000 }));
      return { r, res, doc };
    } catch (e) {
      return { r: e.name === "AbortError" ? "skipped" : "navigate", res, doc };
    } finally {
      settle();
    }
  };
  const sameOrigin = (v) => { try { return new URL(v, location.href).origin === location.origin; } catch { return false; } };

  // go is a GET into the region: the address the server ended on (a redirect
  // is followed in the same request) becomes the new entry. Giving up after a
  // redirect loads that address, with its flashes kept — never the one asked —
  // unless the caller (ui.js following a redirect) gives up on its own.
  const go = async (url, id, trigger, stay) => {
    history.replaceState({ trilhaNav: id, y: scrollY }, "");
    const { r, res, doc } = await fetchInto(url, id, 0, trigger);
    if (r === "swapped") history.pushState({ trilhaNav: id, y: 0 }, "", res.url);
    else if (r === "navigate" && !stay) leave(res, doc, url);
    return r;
  };
  const leave = (res, doc, url) => {
    if (res?.redirected && doc) ui().keepFlashes?.(flashesOf(doc));
    location.assign(res?.redirected ? res.url : url);
  };
  const regionOf = (el) => {
    const holder = el.closest("[data-trilha-nav]");
    const mark = holder?.getAttribute("data-trilha-nav");
    if (!holder || mark === "false") return null;
    return mark || holder.id;
  };

  document.addEventListener("click", (e) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = e.target.closest("a[href]");
    if (!a || a.hasAttribute("download") || a.hasAttribute("data-trilha-target")) return;
    if (a.target && a.target !== "_self") return;
    const id = regionOf(a);
    const url = new URL(a.href, location.href);
    if (!id || url.origin !== location.origin || !document.getElementById(id)) return;
    // Same page with a hash: that is the browser's job.
    if (url.hash && url.pathname === location.pathname && url.search === location.search) return;
    e.preventDefault();
    go(url.href, id, a);
  });

  // A form inside the region navigates in place too (#292): a GET is a link
  // with a query; a POST goes as the browser would send it, without the
  // fragment header, so the route answers its 303 or its 422 page unchanged.
  // Forms of ui.Swap and ui.UploadTo belong to their own scripts.
  const posting = new WeakSet();
  document.addEventListener("submit", (e) => {
    const f = e.target, sub = e.submitter;
    if (e.defaultPrevented || f.hasAttribute("data-trilha-target") || f.hasAttribute("data-trilha-upload")) return;
    const id = regionOf(f);
    const target = sub?.getAttribute("formtarget") || f.getAttribute("target");
    const action = new URL(sub?.getAttribute("formaction") || f.getAttribute("action") || location.href, location.href);
    const method = (sub?.getAttribute("formmethod") || f.getAttribute("method") || "get").toUpperCase();
    if (!id || (target && target !== "_self") || action.origin !== location.origin || !document.getElementById(id) || method === "DIALOG") return;
    e.preventDefault();
    const data = new FormData(f, sub);
    if (method === "GET") {
      action.search = new URLSearchParams(data);
      go(action.href, id, sub || f);
      return;
    }
    if (posting.has(f)) return; // the save is in the air: it does not go twice
    posting.add(f);
    const done = ui().formPending?.(f) || (() => {});
    history.replaceState({ trilhaNav: id, y: scrollY }, "");
    const body = f.enctype === "multipart/form-data" ? data : new URLSearchParams(data);
    fetchInto(action.href, id, 0, sub || f, { method, body }).then(({ r, res, doc }) => {
      // A redirect swapped in is a new address, and Back reaches the form by
      // GET, never by posting again. A 4xx swapped in has no address of its own.
      if (r === "swapped" && res.redirected) history.pushState({ trilhaNav: id, y: 0 }, "", res.url);
      else if (r === "navigate" && res?.redirected) leave(res, doc, res.url); // already accepted: load where it went
      else if (r === "navigate") f.submit();
    }).finally(() => { posting.delete(f); done(); });
  });

  // Back and forward rebuild the page from the server, at the scroll position
  // the entry was left in.
  addEventListener("popstate", (e) => {
    const id = e.state?.trilhaNav;
    if (!id) return;
    fetchInto(location.href, id, e.state.y).then(({ r }) => { if (r === "navigate") location.reload(); });
  });

  // ui.js follows a redirect through here when the page has a region (#291).
  const navRegion = () => document.querySelector('[data-trilha-nav]:not([data-trilha-nav="false"])');
  window.ui = Object.assign(window.ui || {}, {
    navRegion,
    navigate: (url, trigger) => { const el = navRegion(); return el ? go(url, el.getAttribute("data-trilha-nav") || el.id, trigger, true) : Promise.resolve("navigate"); },
  });
})();
