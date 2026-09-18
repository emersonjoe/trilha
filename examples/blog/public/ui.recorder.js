/* trilha ui 7baa695bd2cfb0c6 */
// Kit ui do Trilha — captura de áudio no balcão. Carregado por
// `ui.RecorderScript`. Sem ele o formulário continua funcionando: o campo de
// arquivo com `capture` abre o gravador do sistema no celular e o botão de
// enviar manda o arquivo. O que o script acrescenta é gravar na própria
// página, com o tempo correndo, e entregar a gravação ao mesmo campo — daí
// para a frente quem envia é o `ui.upload.js`.
(() => {
  const REC = "[data-ui-recorder]";
  const supported = () => !!(navigator.mediaDevices?.getUserMedia && window.MediaRecorder);

  const parts = (form) => ({
    toggle: form.querySelector("[data-ui-recorder-toggle]"),
    file: form.querySelector("[data-ui-recorder-file]"),
    time: form.querySelector("[data-ui-recorder-time]"),
    note: form.querySelector("[data-ui-recorder-note]"),
  });

  const clock = (s) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

  const say = (form, text) => {
    const { note } = parts(form);
    if (!note) return;
    note.textContent = text || "";
    note.hidden = !text;
  };

  // The microphone was refused, or there is none: the file field comes back,
  // which is the way that always worked.
  const fallback = (form, text) => {
    const { toggle, file } = parts(form);
    if (toggle) toggle.hidden = true;
    if (file) file.hidden = false;
    say(form, text);
  };

  const ready = (form) => {
    const { toggle, file } = parts(form);
    if (!toggle) return;
    toggle.hidden = false;
    if (file) file.hidden = true; // the recording fills it; nobody types there
  };

  const state = new WeakMap();

  const stop = (form) => {
    const s = state.get(form);
    if (!s) return;
    clearInterval(s.tick);
    s.recorder.stop();
    s.stream.getTracks().forEach((t) => t.stop());
  };

  const start = async (form) => {
    const { toggle, file, time } = parts(form);
    let stream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch (e) {
      // NotAllowedError is the person saying no; the rest is a device that is
      // not there. Both end in the same place: the file field.
      fallback(form, form.getAttribute("data-ui-recorder-denied"));
      return;
    }
    say(form, "");
    const wanted = form.getAttribute("data-ui-recorder-mime") || "";
    const opts = wanted && window.MediaRecorder.isTypeSupported?.(wanted) ? { mimeType: wanted } : {};
    const recorder = new MediaRecorder(stream, opts);
    const chunks = [];
    recorder.addEventListener("dataavailable", (e) => { if (e.data.size) chunks.push(e.data); });
    recorder.addEventListener("stop", () => {
      state.delete(form);
      form.removeAttribute("data-recording");
      toggle.textContent = form.getAttribute("data-ui-recorder-record") || toggle.textContent;
      toggle.setAttribute("aria-pressed", "false");
      const type = recorder.mimeType || wanted || "audio/webm";
      const blob = new Blob(chunks, { type });
      if (!blob.size || !file) return;
      const ext = (type.split(";")[0].split("/")[1] || "webm").replace("mpeg", "mp3");
      const dt = new DataTransfer();
      dt.items.add(new File([blob], `recording.${ext}`, { type }));
      file.files = dt.files;
      // The form is the one that knows where the answer goes: submitting it is
      // what puts ui.upload.js — progress and swap — in front of the request.
      if (typeof form.requestSubmit === "function") form.requestSubmit();
      else form.submit();
    });

    const max = Number(form.getAttribute("data-ui-recorder-max") || 0);
    let seconds = 0;
    if (time) time.textContent = clock(0);
    const tick = setInterval(() => {
      seconds++;
      if (time) time.textContent = clock(seconds);
      if (max && seconds >= max) stop(form);
    }, 1000);

    state.set(form, { recorder, stream, tick });
    form.setAttribute("data-recording", "");
    toggle.textContent = form.getAttribute("data-ui-recorder-stop") || "Stop";
    toggle.setAttribute("aria-pressed", "true");
    recorder.start();
  };

  document.addEventListener("click", (e) => {
    const toggle = e.target.closest?.("[data-ui-recorder-toggle]");
    const form = toggle?.closest(REC);
    if (!form) return;
    e.preventDefault();
    if (state.has(form)) stop(form);
    else start(form);
  });

  const enable = () => {
    for (const form of document.querySelectorAll(REC)) {
      if (form.hasAttribute("data-ui-recorder-on")) continue;
      form.setAttribute("data-ui-recorder-on", "");
      if (supported()) ready(form);
    }
  };
  enable();
  // A recorder that arrived in a swapped fragment gets the button too.
  document.addEventListener("trilha:swap", enable);
})();
