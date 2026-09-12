// chat-demo.js — the model of the site's AI chat demo. It answers
// /_demo/ai-chat in the browser, in the contract ai.Serve writes: `text`
// events word by word and one `done` with the rendered answer and the
// history. ui.chat.js is the real client and does not know the difference;
// nothing else on the page is pretending.
(() => {
  "use strict";
  const endpoint = "/_demo/ai-chat";
  const realFetch = window.fetch.bind(window);
  const encoder = new TextEncoder();
  const pt = (document.documentElement.lang || "").toLowerCase().startsWith("pt");

  // Every answer is a list of runs; a run is [text, strong]. The HTML of the
  // done event is built from these, so nothing typed by the visitor is ever
  // rendered as markup.
  const answer = (message, context) => {
    if (!context || context.order_id !== "1043") {
      return pt
        ? [["Não encontrei um pedido aberto neste contexto."]]
        : [["I could not find an open order in this context."]];
    }
    const q = message.toLowerCase();
    if (/onde|where|status|situa|entreg|deliver|chega|arrive|quando|when/.test(q)) {
      return pt
        ? [["O pedido "], ["1043", 1], [" foi "], ["enviado", 1], [" e chega até sexta-feira."]]
        : [["Order "], ["1043", 1], [" has "], ["shipped", 1], [" and arrives by Friday."]];
    }
    if (/total|valor|quanto|price|cost|much|pag/.test(q)) {
      return pt
        ? [["O pedido "], ["1043", 1], [" custou "], ["R$ 32,90", 1], [", com frete incluído."]]
        : [["Order "], ["1043", 1], [" came to "], ["$32.90", 1], [", shipping included."]];
    }
    if (/troc|devolv|cancel|return|refund/.test(q)) {
      return pt
        ? [["Dá para pedir a devolução até 30 dias depois da entrega. Quer que eu abra o pedido de devolução do "], ["1043", 1], ["?"]]
        : [["You can ask for a return up to 30 days after delivery. Want me to start the return for "], ["1043", 1], ["?"]];
    }
    return pt
      ? [["Posso responder sobre a "], ["situação", 1], [", a "], ["entrega", 1], [", o "], ["total", 1], [" ou a "], ["devolução", 1], [" do pedido 1043."]]
      : [["I can answer about order 1043's "], ["status", 1], [", "], ["delivery", 1], [", "], ["total", 1], [" or "], ["return", 1], ["."]];
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
