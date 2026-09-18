// Kit ui do Trilha — outbox de formulários offline. Carregado por
// `ui.OfflineScript`, junto com o service worker de `trilha add pwa-offline`.
(() => {
  "use strict";
  const tag = document.currentScript || document.querySelector("script[data-ui-offline-routes]");
  const routes = (tag?.getAttribute("data-ui-offline-routes") || "").split(",").filter(Boolean);
  const version = tag?.getAttribute("data-ui-offline-version") || "";
  const DB = "trilha-outbox";
  const STORE = "queue";
  let lastError = "";

  // The worker only ever hears about the routes the server declared: a path
  // nobody marked is never cached, which is how the private area of another
  // audience stays off this disk.
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js").then((reg) => {
      const post = () => (reg.active || navigator.serviceWorker.controller)?.postMessage({ routes, version });
      if (reg.active) post();
      navigator.serviceWorker.ready.then(post);
    }).catch(() => {});
  }

  const open = () => new Promise((resolve, reject) => {
    const req = indexedDB.open(DB, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: "key" });
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });

  const tx = (mode, run) => open().then((db) => new Promise((resolve, reject) => {
    const t = db.transaction(STORE, mode);
    const out = run(t.objectStore(STORE));
    t.oncomplete = () => resolve(out?.result ?? out);
    t.onerror = () => reject(t.error);
  }));

  const all = () => tx("readonly", (s) => s.getAll());
  const put = (entry) => tx("readwrite", (s) => s.put(entry));
  const drop = (key) => tx("readwrite", (s) => s.delete(key));

  const draw = async () => {
    const boxes = [...document.querySelectorAll("[data-ui-outbox]")];
    if (!boxes.length) return;
    let items = [];
    try { items = await all(); } catch { return; }
    for (const box of boxes) {
      box.hidden = items.length === 0 && !lastError;
      const count = box.querySelector("[data-ui-outbox-count]");
      if (count) {
        const one = count.getAttribute("data-ui-outbox-one") || "1 pending";
        const many = count.getAttribute("data-ui-outbox-many") || "{n} pending";
        count.textContent = items.length === 1 ? one : many.replace("{n}", String(items.length));
      }
      const error = box.querySelector("[data-ui-outbox-error]");
      if (error) {
        error.textContent = lastError;
        error.hidden = !lastError;
      }
    }
  };

  // A file cannot wait in the outbox: the bytes do not survive the wait, and
  // pretending they do is worse than saying so on the form's own line.
  const hasFile = (form) => [...new FormData(form).values()].some((v) => typeof v !== "string");

  const queue = async (form, fields, key) => {
    const at = new Date().toISOString();
    const queued = fields.map(([n, v]) => (n === "_queued_at" ? [n, at] : [n, v]));
    if (!queued.some(([n]) => n === "_queued_at")) queued.push(["_queued_at", at]);
    await put({ key, at, action: form.action, method: form.method, fields: queued });
    await draw();
    form.el.dispatchEvent(new CustomEvent("trilha:queued", { bubbles: true, detail: { key } }));
  };

  let sending = false;
  const send = async () => {
    if (sending) return;
    sending = true;
    try {
      const items = (await all()).sort((a, b) => (a.at < b.at ? -1 : a.at > b.at ? 1 : 0));
      for (const item of items) {
        let res;
        try {
          res = await post(item.action, item.method, item.fields, item.key);
        } catch {
          return; // still no network: the queue keeps its order and waits
        }
        if (res.status >= 500) return; // the server is having a bad time; later
        if (res.status >= 400) {
          // Refused, and it will be refused again: keep the answer as the
          // message on the box and stop holding the submission.
          lastError = (await res.text().catch(() => "")).slice(0, 300) || String(res.status);
        } else {
          lastError = "";
        }
        await drop(item.key);
      }
    } catch {
      // No IndexedDB (private window, blocked storage): nothing to replay.
    } finally {
      sending = false;
      await draw();
    }
  };

  // post is the one place a queued submission leaves: the key travels in the
  // Idempotency-Key header, so the server answers the resend with what it
  // answered the first time instead of doing it twice.
  const post = (action, method, fields, key) => fetch(action, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/x-www-form-urlencoded", "Idempotency-Key": key },
    body: new URLSearchParams(fields),
  });

  const read = (form, submitter) => {
    const data = new FormData(form, submitter);
    const fields = [];
    for (const [name, value] of data.entries()) {
      if (typeof value === "string") fields.push([name, value]);
    }
    return fields;
  };

  // A form is one of ours when it carries the marker itself or when the
  // hidden field trilha.OfflineForm wrote is inside it.
  const nossa = (form) => form.matches("form[data-trilha-offline]") || !!form.querySelector("[data-trilha-offline]");

  document.addEventListener("submit", (e) => {
    const el = e.target.closest("form");
    if (!el || e.defaultPrevented || !nossa(el)) return;
    const method = (el.getAttribute("method") || "post").toUpperCase();
    if (method === "GET" || hasFile(el)) return; // not something that can wait
    const action = new URL(el.getAttribute("action") || location.href, location.href);
    if (action.origin !== location.origin) return;
    e.preventDefault();
    const fields = read(el, e.submitter);
    const key = fields.find(([n]) => n === "_idempotency_key")?.[1] ||
      String(Date.now()) + Math.random().toString(16).slice(2);
    const form = { el, action: action.href, method };
    if (!navigator.onLine) {
      queue(form, fields, key);
      return;
    }
    post(action.href, method, fields, key).then(async (res) => {
      if (res.redirected) {
        location.assign(res.url);
        return;
      }
      const text = await res.text();
      document.open();
      document.write(text);
      document.close();
    }).catch(() => queue(form, fields, key)); // the network went away mid-send
  });

  document.addEventListener("click", (e) => {
    if (e.target.closest("[data-ui-outbox-send]")) send();
  });

  addEventListener("online", send);
  draw();
  send();
})();
