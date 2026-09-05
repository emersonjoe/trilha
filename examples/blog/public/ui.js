// Kit ui do Trilha — comportamentos (sem dependências). Atualizado por `trilha ui`.
(() => {
  const $ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  // Theme: [data-ui-theme-toggle] alternates light/dark; persisted in localStorage("ui-theme").
  const applyTheme = (t) => {
    document.documentElement.classList.toggle("dark", t === "dark");
    document.documentElement.classList.toggle("light", t === "light");
  };
  const currentTheme = () => document.documentElement.classList.contains("dark") ? "dark" : "light";
  document.addEventListener("click", (e) => {
    const b = e.target.closest("[data-ui-theme-toggle]");
    if (!b) return;
    const next = currentTheme() === "dark" ? "light" : "dark";
    applyTheme(next);
    try { localStorage.setItem("ui-theme", next); } catch {}
  });

  // Tabs: [data-ui-tabs] > .ui-tabs-list > .ui-tab[aria-controls] + panels.
  const selectTab = (tab) => {
    const tabs = tab.closest("[data-ui-tabs]");
    $(".ui-tab", tabs).forEach((t) => {
      const on = t === tab;
      t.setAttribute("aria-selected", on);
      t.tabIndex = on ? 0 : -1;
      const p = document.getElementById(t.getAttribute("aria-controls"));
      if (p) p.hidden = !on;
    });
  };
  document.addEventListener("click", (e) => {
    const t = e.target.closest(".ui-tab");
    if (t) selectTab(t);
  });
  document.addEventListener("keydown", (e) => {
    const t = e.target.closest(".ui-tab");
    if (!t || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) return;
    const list = $(".ui-tab", t.closest("[data-ui-tabs]"));
    let i = list.indexOf(t);
    if (e.key === "ArrowLeft") i = (i - 1 + list.length) % list.length;
    if (e.key === "ArrowRight") i = (i + 1) % list.length;
    if (e.key === "Home") i = 0;
    if (e.key === "End") i = list.length - 1;
    list[i].focus();
    selectTab(list[i]);
    e.preventDefault();
  });

  // Dialog: [data-ui-dialog-open="id"] opens <dialog id>; [data-ui-dialog-close] closes the nearest.
  document.addEventListener("click", (e) => {
    const open = e.target.closest("[data-ui-dialog-open]");
    if (open) {
      const d = document.getElementById(open.getAttribute("data-ui-dialog-open"));
      if (d && typeof d.showModal === "function") d.showModal();
      return;
    }
    const close = e.target.closest("[data-ui-dialog-close]");
    if (close) close.closest("dialog")?.close();
    const dlg = e.target.closest("dialog.ui-dialog");
    if (dlg && e.target === dlg) dlg.close(); // click on backdrop
  });

  // Fade: [data-ui-fade="ms"] fades out and removes the element after ms.
  const fade = (el) => {
    const ms = parseInt(el.getAttribute("data-ui-fade"), 10) || 4000;
    setTimeout(() => {
      el.classList.add("ui-fading");
      el.addEventListener("transitionend", () => el.remove(), { once: true });
      setTimeout(() => el.remove(), 600);
    }, ms);
  };
  const armFades = (root) => $("[data-ui-fade]:not([data-ui-armed])", root).forEach((el) => { el.setAttribute("data-ui-armed", ""); fade(el); });

  // Toast: window.ui.toast(text, {kind, ms}) appends to .ui-toaster (created on demand).
  const toast = (text, { kind = "", ms = 4000 } = {}) => {
    let box = document.querySelector(".ui-toaster");
    if (!box) { box = document.createElement("div"); box.className = "ui-toaster"; box.setAttribute("aria-live", "polite"); document.body.appendChild(box); }
    const el = document.createElement("div");
    el.className = "ui-toast" + (kind ? " ui-toast-" + kind : "");
    el.setAttribute("role", "status");
    el.setAttribute("data-ui-fade", String(ms));
    el.textContent = text;
    box.appendChild(el);
    armFades(box);
    return el;
  };

  // [data-ui-toast="texto"] shows a toast on click (kind in data-ui-toast-kind).
  document.addEventListener("click", (e) => {
    const b = e.target.closest("[data-ui-toast]");
    if (b) toast(b.getAttribute("data-ui-toast"), { kind: b.getAttribute("data-ui-toast-kind") || "success" });
  });

  // Conditional fields: [data-ui-show-when="campo=valor"] (or "campo=a|b", "campo" for any truthy).
  // Hidden groups also get their controls disabled so they are not submitted.
  const evalShowWhen = (root) => {
    $("[data-ui-show-when]", root).forEach((el) => {
      const [field, values] = el.getAttribute("data-ui-show-when").split("=");
      const form = el.closest("form") || document;
      const controls = $(`[name="${field}"]`, form);
      let val = "";
      for (const c of controls) {
        if ((c.type === "checkbox" || c.type === "radio")) { if (c.checked) val = c.value || "on"; }
        else val = c.value;
      }
      const show = values === undefined ? val !== "" : values.split("|").includes(val);
      el.hidden = !show;
      $("input,select,textarea,button", el).forEach((c) => { c.disabled = !show; });
    });
  };
  document.addEventListener("input", (e) => { if (e.target.closest("form")) evalShowWhen(e.target.closest("form")); });
  document.addEventListener("change", (e) => { if (e.target.closest("form")) evalShowWhen(e.target.closest("form")); });

  // Popover menus: position [popover].ui-menu under its invoker.
  document.addEventListener("toggle", (e) => {
    const m = e.target;
    if (!(m instanceof HTMLElement) || !m.classList.contains("ui-menu") || e.newState !== "open") return;
    const btn = document.querySelector(`[popovertarget="${m.id}"]`);
    if (!btn) return;
    const r = btn.getBoundingClientRect();
    m.style.top = r.bottom + 4 + "px";
    m.style.left = Math.min(r.left, window.innerWidth - m.offsetWidth - 8) + "px";
  }, true);

  // Fragments (spec 018): [data-trilha-target="id"] on an <a> or <form> asks
  // for just that piece of the page and swaps element #id. Without JavaScript
  // the same link navigates and the same form submits — the server answers with
  // the whole page, because nobody sent the header.
  const hydrate = (root) => { armFades(root); evalShowWhen(root); };

  const swap = (id, html, status) => {
    const old = document.getElementById(id);
    if (!old) return false;
    const act = document.activeElement;
    const key = act && old.contains(act) ? (act.id || act.name || "") : "";
    const sel = key && act.selectionStart != null ? [act.selectionStart, act.selectionEnd] : null;
    old.outerHTML = html;
    const el = document.getElementById(id);
    if (!el) return false; // the fragment came back without the id: navigate instead
    const invalid = status === 422 ? el.querySelector("[aria-invalid='true']") : null;
    if (invalid) invalid.focus();
    else if (key) {
      const back = el.querySelector(`#${CSS.escape(key)}, [name="${CSS.escape(key)}"]`);
      if (back) {
        back.focus();
        if (sel && back.setSelectionRange) { try { back.setSelectionRange(sel[0], sel[1]); } catch {} }
      }
    }
    hydrate(el);
    document.dispatchEvent(new CustomEvent("trilha:swap", { detail: { target: el, status } }));
    return true;
  };

  // ask returns false when the right thing to do is a real navigation.
  const ask = async (url, opts, id) => {
    const target = document.getElementById(id);
    target?.setAttribute("aria-busy", "true");
    try {
      const res = await fetch(url, { ...opts, headers: { "Trilha-Fragment": id }, credentials: "same-origin" });
      const loc = res.headers.get("Trilha-Location");
      if (loc) { location.assign(loc); return true; }
      if (res.redirected) { location.assign(res.url); return true; }
      if (res.status >= 500) return false;
      return swap(id, await res.text(), res.status);
    } catch {
      return false; // network is down: a normal navigation may still work
    } finally {
      target?.removeAttribute("aria-busy");
    }
  };

  const pushable = (el) => el.getAttribute("data-trilha-push") !== "false";

  document.addEventListener("click", (e) => {
    const a = e.target.closest("a[data-trilha-target]");
    if (!a || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    if (a.target && a.target !== "_self") return;
    const url = new URL(a.href, location.href);
    if (url.origin !== location.origin) return;
    const id = a.getAttribute("data-trilha-target");
    e.preventDefault();
    ask(url.href, { method: "GET" }, id).then((ok) => {
      if (!ok) { location.assign(url.href); return; }
      if (pushable(a)) history.pushState({ trilhaFragment: id }, "", url.href);
    });
  });

  document.addEventListener("submit", (e) => {
    const f = e.target.closest("form[data-trilha-target]");
    if (!f || e.defaultPrevented) return;
    const action = new URL(f.getAttribute("action") || location.href, location.href);
    if (action.origin !== location.origin) return;
    const id = f.getAttribute("data-trilha-target");
    const method = (f.getAttribute("method") || "get").toUpperCase();
    const data = new FormData(f, e.submitter);
    const btn = e.submitter;
    e.preventDefault();
    let url = action.href, opts = { method };
    if (method === "GET") {
      action.search = new URLSearchParams(data).toString();
      url = action.href;
    } else if (f.enctype === "multipart/form-data") {
      opts.body = data; // files: let the browser build the multipart body
    } else {
      opts.body = new URLSearchParams(data);
    }
    if (btn) btn.disabled = true;
    ask(url, opts, id).then((ok) => {
      if (btn) btn.disabled = false;
      if (!ok) { f.submit(); return; }
      if (method === "GET" && pushable(f)) history.replaceState({ trilhaFragment: id }, "", url);
    });
  });

  // Back undoes a swap made by a link.
  window.addEventListener("popstate", (e) => {
    const id = e.state?.trilhaFragment;
    if (!id) return;
    ask(location.href, { method: "GET" }, id).then((ok) => { if (!ok) location.reload(); });
  });

  const init = () => { armFades(document); evalShowWhen(document); };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init); else init();
  window.ui = Object.assign(window.ui || {}, { toast, fade, evalShowWhen, applyTheme, swap, hydrate });
})();
