/* trilha ui 451b0ab9de7581e5 */
// Trilha ui kit — the combobox. ui.js loads it on the page that has a
// [data-ui-combo] (spec 177); without it the field is still text posted as
// Name_q, which is the path without JavaScript.
(() => {
  const $ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  // Combobox: [data-ui-combo] wraps a text input (role=combobox), a hidden
  // input with the value and a <ul role=listbox>. The server does the search
  // and answers the options as HTML (ui.ComboboxOptions); a short list is
  // already inside the <ul> and is filtered here.
  const comboParts = (box) => ({
    text: box.querySelector('input[role="combobox"]'),
    hidden: box.querySelector('input[type="hidden"]'),
    list: box.querySelector('[role="listbox"]'),
  });
  const comboOpts = (list) => $('[role="option"]', list).filter((o) => !o.hidden);
  const comboClose = (box) => {
    const { text, list } = comboParts(box);
    list.hidden = true;
    text.setAttribute("aria-expanded", "false");
    text.removeAttribute("aria-activedescendant");
  };
  const comboMark = (box, opt) => {
    const { text, list } = comboParts(box);
    $('[role="option"]', list).forEach((o) => o.setAttribute("aria-selected", String(o === opt)));
    if (!opt) return text.removeAttribute("aria-activedescendant");
    if (!opt.id) opt.id = `${list.id}-o${$('[role="option"]', list).indexOf(opt)}`;
    text.setAttribute("aria-activedescendant", opt.id);
    opt.scrollIntoView({ block: "nearest" });
  };
  const comboOpen = (box) => {
    const { text, list } = comboParts(box);
    if (!comboOpts(list).length) return comboClose(box);
    list.hidden = false;
    text.setAttribute("aria-expanded", "true");
  };
  const comboPick = (box, opt) => {
    const { text, hidden } = comboParts(box);
    text.value = opt.textContent.trim();
    hidden.value = opt.getAttribute("data-value") || "";
    comboClose(box);
    hidden.dispatchEvent(new Event("change", { bubbles: true }));
  };
  const comboSearch = async (box) => {
    const { text, list } = comboParts(box);
    const src = box.getAttribute("data-ui-combo-src");
    const q = text.value.trim();
    if (q.length < Number(box.getAttribute("data-ui-combo-min") || 1)) return comboClose(box);
    if (!src) { // static list: the filter is here, there is nothing to ask
      const needle = q.toLowerCase();
      $('[role="option"]', list).forEach((o) => (o.hidden = !o.textContent.toLowerCase().includes(needle)));
      return comboOpen(box);
    }
    const url = new URL(src, location.href);
    url.searchParams.set("q", q);
    for (const name of (box.getAttribute("data-ui-combo-with") || "").split(/\s+/).filter(Boolean)) {
      const other = box.closest("form")?.elements[name];
      if (other) url.searchParams.set(name, other.value);
    }
    box.setAttribute("aria-busy", "true");
    try {
      const res = await fetch(url, { headers: { "Trilha-Fragment": list.id } });
      list.innerHTML = res.ok ? await res.text() : "";
    } catch { list.innerHTML = ""; }
    box.removeAttribute("aria-busy");
    comboOpen(box);
  };
  const comboTimers = new WeakMap();
  document.addEventListener("input", (e) => {
    const box = e.target.closest?.("[data-ui-combo]");
    if (!box || e.target.getAttribute("role") !== "combobox") return;
    // Typing throws the choice away: the text and the value cannot disagree.
    comboParts(box).hidden.value = "";
    clearTimeout(comboTimers.get(box));
    comboTimers.set(box, setTimeout(() => comboSearch(box), Number(box.getAttribute("data-ui-combo-wait") || 250)));
  });
  document.addEventListener("keydown", (e) => {
    const box = e.target.closest?.("[data-ui-combo]");
    if (!box || e.target.getAttribute("role") !== "combobox") return;
    const { list } = comboParts(box);
    const opts = comboOpts(list);
    const at = opts.findIndex((o) => o.getAttribute("aria-selected") === "true");
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (list.hidden) comboOpen(box);
      const step = e.key === "ArrowDown" ? 1 : -1;
      const from = at < 0 ? (step > 0 ? -1 : 0) : at;
      const next = opts[(from + step + opts.length) % (opts.length || 1)];
      if (next) comboMark(box, next);
    } else if (e.key === "Enter" && !list.hidden && at >= 0) {
      e.preventDefault(); // the choice is not the submit
      comboPick(box, opts[at]);
    } else if (e.key === "Escape" && !list.hidden) {
      e.preventDefault();
      comboClose(box);
    }
  });
  document.addEventListener("click", (e) => {
    const opt = e.target.closest?.('[data-ui-combo] [role="option"]');
    if (opt) return comboPick(opt.closest("[data-ui-combo]"), opt);
    $("[data-ui-combo]").forEach((box) => { if (!box.contains(e.target)) comboClose(box); });
  });
  document.addEventListener("focusout", (e) => {
    const box = e.target.closest?.("[data-ui-combo]");
    // A click on an option is a focusout too, so let it land first.
    if (box) setTimeout(() => { if (!box.contains(document.activeElement)) comboClose(box); }, 0);
  });
})();
