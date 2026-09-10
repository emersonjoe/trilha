// assistant-demo.js — the model of the site's assistant demo. It answers
// /_demo/assistant in the browser, in the contract ai.Serve writes: `text`
// events word by word and one `done` with the rendered answer and the
// history. ui.chat.js is the real client and does not know the difference;
// nothing else on the page is pretending.
(() => {
  "use strict";
  const endpoint = "/_demo/assistant";
  const realFetch = window.fetch.bind(window);
  const encoder = new TextEncoder();
  const pt = (document.documentElement.lang || "").toLowerCase().startsWith("pt");

  // Every answer is a list of runs; a run is [text, strong]. The HTML of the
  // done event is built from these, so nothing typed by the visitor is ever
  // rendered as markup.
  const answer = (message, context) => {
    if (!context || context.invoice_id !== "42") {
      return pt
        ? [["Não encontrei uma fatura aberta neste contexto."]]
        : [["I could not find an open invoice in this context."]];
    }
    const q = message.toLowerCase();
    if (/total|valor|amount|much/.test(q)) {
      return pt
        ? [["A "], ["fatura 42", 1], [" totaliza "], ["R$ 1.284,50", 1], ["."]]
        : [[""], ["Invoice 42", 1], [" totals "], ["$1,284.50", 1], ["."]];
    }
    if (/venc|due|date|data|when|quando/.test(q)) {
      return pt
        ? [["O vencimento da "], ["fatura 42", 1], [" é "], ["30 de setembro de 2026", 1], ["."]]
        : [[""], ["Invoice 42", 1], [" is due on "], ["September 30, 2026", 1], ["."]];
    }
    if (/status|situa|open|aberta|paid|paga/.test(q)) {
      return pt
        ? [["A "], ["fatura 42", 1], [" está "], ["em aberto", 1], ["."]]
        : [[""], ["Invoice 42", 1], [" is currently "], ["open", 1], ["."]];
    }
    return pt
      ? [["Posso responder sobre o "], ["total", 1], [", a "], ["situação", 1], [" ou o "], ["vencimento", 1], [" da fatura 42."]]
      : [["I can answer about invoice 42's "], ["total", 1], [", "], ["status", 1], [" or "], ["due date", 1], ["."]];
  };

  const escape = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  const text = (runs) => runs.map((r) => r[0]).join("");
  const html = (runs) =>
    '<div class="ui-md"><p>' + runs.map((r) => (r[1] ? "<strong>" + escape(r[0]) + "</strong>" : escape(r[0]))).join("") + "</p></div>";
  const event = (name, data) => encoder.encode("event: " + name + "\ndata: " + data + "\n\n");

  window.fetch = (input, init) => {
    const url = typeof input === "string" ? input : input.url;
    if (!url.endsWith(endpoint)) return realFetch(input, init);
    let payload = {};
    try {
      payload = JSON.parse((init && init.body) || "{}");
    } catch (e) {}
    const runs = answer(String(payload.message || ""), payload.context || {});
    const output = text(runs);
    const history = (payload.history || []).concat([
      { role: "user", content: payload.message || "" },
      { role: "assistant", content: output },
    ]);
    // Word by word, with a pause between them: it is the streaming the real
    // route does, slowed down enough to be seen.
    const words = output.split(/(?<=\s)/);
    let i = 0;
    const stream = new ReadableStream({
      start(controller) {
        const next = () => {
          if (i < words.length) {
            controller.enqueue(event("text", words[i++]));
            window.setTimeout(next, 40);
            return;
          }
          controller.enqueue(event("done", JSON.stringify({ output: output, html: html(runs), history: history })));
          controller.close();
        };
        window.setTimeout(next, 250);
      },
    });
    return Promise.resolve(new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } }));
  };
})();
