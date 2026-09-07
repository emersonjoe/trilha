/* trilha ui b4c922f01185f691 */
// Kit ui do Trilha — envio de arquivo com progresso. Carregado por `ui.UploadScript`.
(() => {
  // The browser is the only one that knows how many bytes have left the
  // machine, so the progress comes from XHR (fetch has no upload progress).
  // Everything else is the fragment of spec 018: same route, same answer.
  document.addEventListener("submit", (e) => {
    const f = e.target.closest("form[data-trilha-upload]");
    if (!f || e.defaultPrevented) return;
    const id = f.getAttribute("data-trilha-upload");
    const action = new URL(f.getAttribute("action") || location.href, location.href);
    if (action.origin !== location.origin || !document.getElementById(id)) return;
    e.preventDefault();

    const bar = f.querySelector("[data-trilha-progress]");
    const target = document.getElementById(id);
    const data = new FormData(f, e.submitter);
    const xhr = new XMLHttpRequest();
    const give = () => { f.removeAttribute("data-trilha-sending"); f.submit(); };

    xhr.upload.addEventListener("progress", (p) => {
      if (bar) {
        bar.hidden = false;
        if (p.lengthComputable) { bar.max = p.total; bar.value = p.loaded; }
        else bar.removeAttribute("value"); // unknown size: indeterminate
      }
      f.dispatchEvent(new CustomEvent("trilha:upload", {
        bubbles: true,
        detail: { loaded: p.loaded, total: p.lengthComputable ? p.total : 0, form: f },
      }));
    });
    xhr.addEventListener("load", () => {
      if (bar) bar.hidden = true;
      target.removeAttribute("aria-busy");
      f.removeAttribute("data-trilha-sending");
      if (xhr.status >= 500 || !window.ui?.swap?.(id, xhr.responseText, xhr.status)) give();
    });
    xhr.addEventListener("error", give);
    xhr.addEventListener("abort", give);

    xhr.open((f.getAttribute("method") || "post").toUpperCase(), action.href);
    xhr.setRequestHeader("Trilha-Fragment", id);
    f.setAttribute("data-trilha-sending", "");
    target.setAttribute("aria-busy", "true");
    if (bar) { bar.hidden = false; bar.value = 0; }
    xhr.send(data);
  });

  // Dropzone: [data-ui-dropzone] takes files by dragging or by clicking, and
  // sends them one request each — one line, one bar, one message per file. A
  // multipart of fifty files that dies on the forty-ninth cannot say which one
  // failed, and retrying it would start from zero.
  const human = (n) => {
    const u = ["B", "KB", "MB", "GB"];
    let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return `${n < 10 && i ? n.toFixed(1) : Math.round(n)} ${u[i]}`;
  };
  const queues = new WeakMap();

  const line = (zone, file) => {
    const li = document.createElement("li");
    li.innerHTML = `<div class="ui-queue-head"><span class="ui-queue-name"></span><span class="ui-queue-size"></span></div><progress class="ui-upload" value="0" max="100"></progress><p class="ui-queue-error" hidden></p>`;
    li.querySelector(".ui-queue-name").textContent = file.name;
    li.querySelector(".ui-queue-size").textContent = human(file.size);
    zone.querySelector("[data-ui-queue]").appendChild(li);
    return li;
  };
  const fail = (li, msg) => {
    li.setAttribute("data-state", "error");
    li.querySelector("progress").hidden = true;
    const p = li.querySelector(".ui-queue-error");
    p.hidden = false;
    p.textContent = msg;
  };

  const send = (zone, file, li) => new Promise((done) => {
    const form = zone.closest("form");
    const input = zone.querySelector('input[type="file"]');
    const max = Number(zone.getAttribute("data-ui-dropzone-max") || 0);
    if (max && file.size > max) { fail(li, `> ${human(max)}`); return done(); }

    const id = form.getAttribute("data-trilha-upload");
    const data = new FormData(form);
    data.delete(input.name);
    data.append(input.name, file);
    const xhr = new XMLHttpRequest();
    const bar = li.querySelector("progress");
    xhr.upload.addEventListener("progress", (p) => {
      if (!p.lengthComputable) return bar.removeAttribute("value");
      bar.max = p.total;
      bar.value = p.loaded;
    });
    xhr.addEventListener("load", () => {
      if (xhr.status >= 400) {
        // The answer belongs to this file, so it is shown on this line: the
        // route says it in plain text, or the line says the status.
        const kind = xhr.getResponseHeader("Content-Type") || "";
        fail(li, kind.startsWith("text/plain") ? xhr.responseText.slice(0, 200) : `HTTP ${xhr.status}`);
      } else {
        window.ui?.swap?.(id, xhr.responseText, xhr.status);
        li.remove();
      }
      done();
    });
    xhr.addEventListener("error", () => { fail(li, "network"); done(); });
    xhr.open((form.getAttribute("method") || "post").toUpperCase(), new URL(form.getAttribute("action") || location.href, location.href).href);
    xhr.setRequestHeader("Trilha-Fragment", id);
    xhr.send(data);
  });

  const pump = async (zone) => {
    if (zone.hasAttribute("data-sending")) return;
    zone.setAttribute("data-sending", "");
    const q = queues.get(zone);
    while (q.length) {
      const { file, li } = q.shift();
      await send(zone, file, li);
    }
    zone.removeAttribute("data-sending");
  };

  // The queue only exists when the form says where the answer goes; without
  // that the input keeps the files and the form's own button sends them.
  const takes = (zone) => zone.closest("form")?.hasAttribute("data-trilha-upload") && document.getElementById(zone.closest("form").getAttribute("data-trilha-upload"));

  const enqueue = (zone, files) => {
    if (!files.length) return;
    const q = queues.get(zone) || [];
    for (const file of files) q.push({ file, li: line(zone, file) });
    queues.set(zone, q);
    pump(zone);
  };

  document.addEventListener("change", (e) => {
    const zone = e.target.closest?.("[data-ui-dropzone]");
    if (!zone || e.target.type !== "file" || !takes(zone)) return;
    enqueue(zone, e.target.files);
    e.target.value = ""; // the queue has them now
  });

  const over = (e, on) => {
    const zone = e.target.closest?.("[data-ui-dropzone]");
    if (!zone) return;
    e.preventDefault();
    if (on) zone.setAttribute("data-over", "");
    else zone.removeAttribute("data-over");
  };
  document.addEventListener("dragover", (e) => over(e, true));
  document.addEventListener("dragleave", (e) => over(e, false));
  document.addEventListener("drop", (e) => {
    const zone = e.target.closest?.("[data-ui-dropzone]");
    if (!zone) return;
    over(e, false);
    const files = e.dataTransfer?.files;
    if (!files?.length) return;
    if (takes(zone)) return enqueue(zone, files);
    // No queue: hand the files to the input, so the plain form still sends them.
    const input = zone.querySelector('input[type="file"]');
    const dt = new DataTransfer();
    for (const f of files) dt.items.add(f);
    input.files = dt.files;
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
})();
