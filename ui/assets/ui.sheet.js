// Kit ui do Trilha — painel lateral. Carregado só por `ui.SheetScript`.
(() => {
  // A [data-ui-sheet-open=id] link opens the Sheet #id and asks its address
  // for the fragment "<id>-body", through ui.js — the same pending marks, the
  // same redirect handling, the same scripts run on arrival. The panel lives
  // outside the client-navigation region, so it stays open while the page
  // beside it navigates (#296).
  const ui = () => window.ui || {};
  let opener = null;
  const shown = (s) => (s.tagName === "DIALOG" ? s.open : !s.hidden);
  const expand = (a, v) => a?.setAttribute("aria-expanded", String(v));

  const open = (s, a) => {
    if (opener !== a) expand(opener, false);
    opener = a;
    expand(a, true);
    // On a phone the panel covers the screen, and the shell's drawer would
    // fight it for the same edge: one moment at a time.
    document.documentElement.classList.remove("ui-drawer-open");
    if (!shown(s)) {
      if (s.tagName === "DIALOG") s.showModal();
      else s.hidden = false;
      if (s.getAttribute("data-ui-sheet-history") === "push") history.pushState({ uiSheet: s.id }, "");
    }
    s.querySelector(".ui-sheet-title")?.focus({ preventScroll: true });
  };

  // close empties the body — a PDF frame left alive in a hidden panel is
  // memory nobody sees — and gives the focus back to the link that opened it.
  const close = (s, fromHistory) => {
    if (!s || !shown(s)) return;
    if (s.tagName === "DIALOG") return s.close(); // its close event comes back to done
    s.hidden = true;
    done(s, fromHistory);
  };
  const done = (s, fromHistory) => {
    s.querySelector(".ui-sheet-body")?.replaceChildren();
    expand(opener, false);
    opener?.focus({ preventScroll: true });
    opener = null;
    if (!fromHistory && history.state?.uiSheet === s.id) history.back();
  };

  document.addEventListener("click", (e) => {
    const a = e.target.closest("[data-ui-sheet-open]");
    if (a && !e.defaultPrevented && e.button === 0 && !(e.metaKey || e.ctrlKey || e.shiftKey || e.altKey)) {
      const s = document.getElementById(a.getAttribute("data-ui-sheet-open"));
      if (!s || !a.href || !ui().fragment) return; // nothing to open into: the link is a link
      e.preventDefault();
      open(s, a);
      ui().fragment(a.href, s.id + "-body", a);
      return;
    }
    const c = e.target.closest("[data-ui-sheet-close]");
    if (c) close(c.closest("[data-ui-sheet]"));
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    const s = [...document.querySelectorAll("aside[data-ui-sheet]")].find(shown);
    if (s) close(s);
  });
  // A modal panel is a native <dialog>: Escape closes it on its own, and the
  // rest of the cleanup follows its close event.
  document.addEventListener("close", (e) => {
    const s = e.target;
    if (s.matches?.("dialog[data-ui-sheet]")) { done(s, s.__fromHistory); s.__fromHistory = false; }
  }, true);

  // Back closes a panel that pushed its entry.
  addEventListener("popstate", (e) => {
    document.querySelectorAll("[data-ui-sheet]").forEach((s) => {
      if (!shown(s) || e.state?.uiSheet === s.id) return;
      if (s.tagName === "DIALOG") { s.__fromHistory = true; s.close(); } else close(s, true);
    });
  });
})();
