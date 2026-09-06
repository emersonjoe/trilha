/* trilha ui 7003d0b52a86efe2 */
// Kit ui do Trilha — fragmentos que se atualizam sozinhos. Carregado só por
// `ui.LiveScript`.
(() => {
  const POLL = "data-trilha-poll", SRC = "data-trilha-src";
  const ON = "data-trilha-on", LIVE = "data-trilha-live";
  const MAX = 60000; // the slowest a fragment ever gets, after failing

  // ms reads "6s", "500ms" or "2m"; 0 when it is not a duration.
  const ms = (s) => {
    const m = /^\s*(\d+(?:\.\d+)?)\s*(ms|s|m)?\s*$/.exec(s || "");
    if (!m) return 0;
    const n = parseFloat(m[1]);
    return m[2] === "ms" ? n : m[2] === "m" ? n * 60000 : n * 1000;
  };

  // The state is keyed by the id of the fragment, not by the element: the
  // element is replaced at every swap, the id is what stays.
  const state = new Map();
  const of = (el) => {
    let s = state.get(el.id);
    if (!s) {
      s = { every: ms(el.getAttribute(POLL)), fails: 0, next: 0 };
      s.base = s.every;
      s.next = Date.now() + s.every;
      state.set(el.id, s);
    }
    return s;
  };

  const swap = (id, html) => {
    const old = document.getElementById(id);
    if (!old || old.contains(document.activeElement)) return; // never type into a fragment that is being replaced
    old.outerHTML = html;
    const el = document.getElementById(id);
    if (el) document.dispatchEvent(new CustomEvent("trilha:hydrate", { detail: { target: el } }));
    arm();
  };

  // ask fetches the fragment and applies what the route said about the clock.
  const ask = async (el) => {
    const id = el.id, s = of(el);
    if (!id || s.busy) return;
    s.busy = true;
    try {
      const res = await fetch(el.getAttribute(SRC) || location.href, {
        headers: { "Trilha-Fragment": id }, credentials: "same-origin",
      });
      const say = res.headers.get("Trilha-Poll");
      if (!res.ok) {
        if (res.status >= 400 && res.status < 500 && res.status !== 429) { s.stop = true; return; }
        const after = ms((res.headers.get("Retry-After") || "") + "s");
        s.fails++;
        s.next = Date.now() + (after || Math.min(s.base * 2 ** s.fails, MAX));
        return;
      }
      swap(id, await res.text());
      s.fails = 0;
      if (say === "stop") { s.stop = true; return; }
      const other = ms(say);
      if (other) s.base = other;
      s.next = Date.now() + s.base;
    } catch {
      s.fails++;
      s.next = Date.now() + Math.min(s.base * 2 ** s.fails, MAX);
    } finally {
      s.busy = false;
    }
  };

  // ---- the clock ----------------------------------------------------------

  let sse = null, up = false;

  const tick = () => {
    if (document.hidden) return; // a tab nobody is looking at asks nothing
    const now = Date.now();
    for (const el of document.querySelectorAll(`[${POLL}]`)) {
      if (!el.id) continue;
      const s = of(el);
      if (s.stop || !s.base) continue;
      if (up && el.hasAttribute(ON)) continue; // the stream is telling it; the clock is the fallback
      if (now >= s.next) { s.next = now + s.base; ask(el); }
    }
  };

  document.addEventListener("visibilitychange", () => {
    if (document.hidden) return;
    for (const s of state.values()) s.next = 0; // came back: show what happened while away
    tick();
  });

  // ---- the stream ---------------------------------------------------------

  const armed = new Set();

  // arm connects the page's stream and subscribes to the names the fragments
  // on screen are waiting for. New fragments arrive with every swap, so it
  // runs again after each one.
  const arm = () => {
    const live = document.querySelector(`[${LIVE}]`);
    if (live && !sse) {
      sse = new EventSource(live.getAttribute(LIVE), { withCredentials: true });
      sse.onopen = () => { up = true; };
      sse.onerror = () => { up = false; }; // EventSource reconnects on its own
    }
    if (!sse) return;
    for (const el of document.querySelectorAll(`[${ON}]`)) {
      const name = el.getAttribute(ON);
      if (!name || armed.has(name)) continue;
      armed.add(name);
      sse.addEventListener(name, () => {
        for (const t of document.querySelectorAll(`[${ON}]`)) {
          if (t.getAttribute(ON) === name && t.id) ask(t);
        }
      });
    }
  };

  const start = () => { arm(); tick(); };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
  document.addEventListener("trilha:swap", arm);
  setInterval(tick, 500);
})();
