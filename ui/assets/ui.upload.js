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
})();
