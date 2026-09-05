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

  const init = () => { armFades(document); evalShowWhen(document); };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init); else init();
  window.ui = Object.assign(window.ui || {}, { toast, fade, evalShowWhen, applyTheme });
})();
