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

  // Flashes (spec 053): a fragment answer carries the messages of c.Flash in a
  // header, because there is no redirect for a cookie to survive.
  const showFlashes = (v) => {
    try {
      const bin = atob(v.replace(/-/g, "+").replace(/_/g, "/"));
      const txt = new TextDecoder().decode(Uint8Array.from(bin, (ch) => ch.charCodeAt(0)));
      for (const f of JSON.parse(txt) || []) toast(f.t, { kind: f.k || "", ms: 5000 });
    } catch {}
  };

  // Confirm (spec 053): a form with [data-ui-confirm] asks before it submits.
  // The dialog is built here, with the kit's own classes, so no page needs a
  // <dialog> per button and no app needs inline script the CSP would block.
  const confirm = (f, btn) => new Promise((resolve) => {
    const d = document.createElement("dialog");
    d.className = "ui-dialog";
    const el = (tag, cls, txt) => { const n = document.createElement(tag); n.className = cls; n.textContent = txt; return n; };
    d.appendChild(el("h2", "ui-dialog-title", f.getAttribute("data-ui-confirm") || ""));
    const desc = f.getAttribute("data-ui-confirm-description");
    if (desc) d.appendChild(el("p", "ui-dialog-description", desc));
    const cancel = el("button", "ui-btn ui-btn-outline", f.getAttribute("data-ui-confirm-cancel") || "Cancel");
    const ok = el("button", "ui-btn" + (btn?.classList.contains("ui-btn-destructive") ? " ui-btn-destructive" : ""), btn?.textContent.trim() || "OK");
    cancel.type = ok.type = "button";
    const foot = el("div", "ui-dialog-footer", "");
    foot.append(cancel, ok);
    d.appendChild(foot);
    document.body.appendChild(d);
    let done = false;
    const finish = (v) => { if (done) return; done = true; d.close(); d.remove(); resolve(v); };
    cancel.addEventListener("click", () => finish(false));
    ok.addEventListener("click", () => finish(true));
    d.addEventListener("cancel", () => finish(false)); // Escape
    d.addEventListener("click", (ev) => { if (ev.target === d) finish(false); });
    d.showModal();
    cancel.focus(); // the safe answer is the one under the finger
  });

  // Capture, so the confirmation happens before the fragment listener below
  // decides what to do with the same submit.
  document.addEventListener("submit", (e) => {
    const f = e.target;
    if (!(f instanceof HTMLFormElement) || !f.hasAttribute("data-ui-confirm") || f.dataset.uiConfirmed) return;
    const btn = e.submitter;
    e.preventDefault();
    e.stopPropagation();
    confirm(f, btn).then((yes) => {
      if (!yes) return;
      f.dataset.uiConfirmed = "1"; // the second submit is the confirmed one
      if (f.requestSubmit) f.requestSubmit(btn); else f.submit();
      delete f.dataset.uiConfirmed;
    });
  }, true);

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
  const hydrate = (root) => { armFades(root); evalShowWhen(root); initTooltips(root); };

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
      const flash = res.headers.get("Trilha-Flash");
      if (flash) showFlashes(flash);
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

  // Tooltips: [data-ui-tooltip] also carries a title, so the hint exists with
  // this script off. Here the title goes away — two tooltips is worse than none
  // — and a bubble takes its place, reachable by focus and by touch and closed
  // by Escape (WCAG 1.4.13).
  let tipN = 0;
  const tipOf = (el) => {
    let tip = el.lastElementChild;
    if (tip && tip.className === "ui-tooltip-bubble") return tip;
    tip = document.createElement("span");
    tip.className = "ui-tooltip-bubble";
    tip.setAttribute("role", "tooltip");
    tip.id = "ui-tip-" + ++tipN;
    tip.textContent = el.dataset.uiTooltip; // never innerHTML: it is app text
    tip.hidden = true;
    el.removeAttribute("title");
    const target = el.querySelector("a,button,input,select,textarea,[tabindex]") || el;
    if (target === el) el.tabIndex = 0;
    target.setAttribute("aria-describedby", tip.id);
    el.appendChild(tip);
    return tip;
  };
  const initTooltips = (root) => $("[data-ui-tooltip]", root).forEach(tipOf);
  const hideTips = () => $(".ui-tooltip-bubble").forEach((t) => (t.hidden = true));
  // A touch has no hover, so the tap that reaches the control shows the hint.
  const showTip = (e) => {
    const el = e.target.closest?.("[data-ui-tooltip]");
    $(".ui-tooltip-bubble").forEach((t) => (t.hidden = t.parentElement !== el));
    if (!el) return;
    const tip = tipOf(el);
    tip.style.transform = "translateX(-50%)";
    const r = tip.getBoundingClientRect(); // and keep it inside the window
    const dx = r.left < 8 ? 8 - r.left : Math.min(0, innerWidth - 8 - r.right);
    if (dx) tip.style.transform = `translateX(calc(-50% + ${Math.round(dx)}px))`;
  };
  ["pointerover", "focusin", "click"].forEach((ev) => document.addEventListener(ev, showTip));
  document.addEventListener("keydown", (e) => e.key === "Escape" && hideTips());

  const init = () => { armFades(document); evalShowWhen(document); initTooltips(document); };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init); else init();
  window.ui = Object.assign(window.ui || {}, { toast, fade, confirm, evalShowWhen, applyTheme, swap, hydrate, initTooltips });
})();
