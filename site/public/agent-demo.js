// agent-demo.js — the model of the site's AI agent demo. It answers
// /_demo/ai-agent in the browser, in the contract ai.Serve writes: `handoff`,
// `tool_call` and `tool_result` events for the steps, `text` word by word and
// one `done` with the rendered answer. ui.chat.js is the real client and does
// not know the difference; the queue below the chat is the real ui.Inbox, and
// the only thing pretending is the model.
(() => {
  "use strict";
  const endpoint = "/_demo/ai-agent";
  const realFetch = window.fetch.bind(window);
  const encoder = new TextEncoder();
  const pt = (document.documentElement.lang || "").toLowerCase().startsWith("pt");

  const order = '{"id":"1043","status":"shipped","total":"$32.90","placed":"2026-09-01"}';

  // A run is a list of steps and an answer. A step is one event: the tool it
  // called and what came back, or the agent it handed the conversation to.
  const run = (message, context) => {
    if (!context || context.order_id !== "1043") {
      return { steps: [], runs: [[pt ? "Não encontrei um pedido aberto neste contexto." : "I could not find an open order in this context."]] };
    }
    const q = message.toLowerCase();
    if (/cancel|devolv|troc|refund|estorn|dinheiro back|money/.test(q)) {
      return {
        steps: [
          { event: "handoff", to: "billing" },
          { event: "tool_call", tool: "get_order", arguments: '{"id":"1043"}' },
          { event: "tool_result", tool: "get_order", output: order },
          { event: "tool_call", tool: "cancel_order", arguments: '{"order_id":"1043","reason":"' + (pt ? "cliente pediu" : "customer asked") + '"}' },
          { event: "tool_result", tool: "cancel_order", output: "request apr_1043 is waiting for a person to decide; the order has not changed", queue: 1 },
        ],
        runs: pt
          ? [["Pedi o cancelamento do pedido "], ["1043", 1], [". Ele está "], ["esperando alguém do atendimento", 1], [" — nada mudou no pedido ainda, e você recebe um aviso quando a decisão sair."]]
          : [["I asked for order "], ["1043", 1], [" to be cancelled. It is "], ["waiting for someone in support", 1], [" — nothing has changed on the order yet, and you will hear back when it is decided."]],
      };
    }
    if (/total|valor|quanto|price|cost|much|pag/.test(q)) {
      return {
        steps: [
          { event: "tool_call", tool: "search_orders", arguments: '{"query":"1043"}' },
          { event: "tool_result", tool: "search_orders", output: "order 1043: Order 1043 — shipped (/orders/1043)" },
        ],
        runs: pt
          ? [["O pedido "], ["1043", 1], [" custou "], ["R$ 32,90", 1], [", com frete incluído."]]
          : [["Order "], ["1043", 1], [" came to "], ["$32.90", 1], [", shipping included."]],
      };
    }
    if (/onde|where|status|situa|entreg|deliver|chega|arrive|quando|when/.test(q)) {
      return {
        steps: [
          { event: "tool_call", tool: "get_order", arguments: '{"id":"1043"}' },
          { event: "tool_result", tool: "get_order", output: order },
        ],
        runs: pt
          ? [["O pedido "], ["1043", 1], [" foi "], ["enviado", 1], [" no dia 1º e chega até sexta-feira."]]
          : [["Order "], ["1043", 1], [" "], ["shipped", 1], [" on the 1st and arrives by Friday."]],
      };
    }
    return {
      steps: [
        { event: "tool_call", tool: "search_orders", arguments: '{"query":"' + (pt ? "pedidos" : "orders") + '"}' },
        { event: "tool_result", tool: "search_orders", output: "order 1043: Order 1043 — shipped (/orders/1043)" },
      ],
      runs: pt
        ? [["Posso responder sobre a "], ["situação", 1], [", o "], ["total", 1], [" ou o "], ["cancelamento", 1], [" do pedido 1043."]]
        : [["I can answer about order 1043's "], ["status", 1], [", "], ["total", 1], [", or "], ["cancel", 1], [" it for you."]],
    };
  };

  const escape = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  const text = (runs) => runs.map((r) => r[0]).join("");
  const html = (runs) =>
    '<div class="ui-md"><p>' + runs.map((r) => (r[1] ? "<strong>" + escape(r[0]) + "</strong>" : escape(r[0]))).join("") + "</p></div>";
  const event = (name, data) => encoder.encode("event: " + name + "\ndata: " + data + "\n\n");

  // The queue is on the page already, hidden: a request the agent has not
  // asked for yet is a request nobody opened.
  const showQueue = () => {
    const el = document.getElementById("agent-demo-queue");
    if (el) el.removeAttribute("hidden");
  };

  // Deciding is a form post in the app; here it is the last step of the story,
  // so the row says who decided and the order changes on the card.
  const wireQueue = () => {
    const queue = document.getElementById("agent-demo-queue");
    if (!queue) return;
    queue.addEventListener("submit", (e) => {
      const form = e.target.closest("form");
      if (!form) return;
      e.preventDefault();
      const approved = (document.activeElement && document.activeElement.value) === "approved";
      const cell = form.parentNode;
      cell.textContent = approved
        ? pt ? "aprovado por você — o pedido foi cancelado" : "approved by you — the order was cancelled"
        : pt ? "recusado por você — o pedido continua" : "rejected by you — the order stands";
      const state = queue.querySelector(".ui-badge");
      if (state) state.textContent = approved ? (pt ? "Aprovado" : "Approved") : pt ? "Recusado" : "Rejected";
    });
  };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", wireQueue);
  else wireQueue();

  window.fetch = (input, init) => {
    const url = typeof input === "string" ? input : input.url;
    if (!url.endsWith(endpoint)) return realFetch(input, init);
    let payload = {};
    try {
      payload = JSON.parse((init && init.body) || "{}");
    } catch (e) {}
    const answer = run(String(payload.message || ""), payload.context || {});
    const output = text(answer.runs);
    const history = (payload.history || []).concat([
      { role: "user", content: payload.message || "" },
      { role: "assistant", content: output },
    ]);
    const steps = answer.steps.slice();
    const words = output.split(/(?<=\s)/);
    let i = 0;
    const stream = new ReadableStream({
      start(controller) {
        // The steps first, one every third of a second — the pace of a tool
        // that really ran — and then the answer, word by word.
        const next = () => {
          if (steps.length) {
            const s = steps.shift();
            if (s.queue) showQueue();
            controller.enqueue(event(s.event, JSON.stringify(s)));
            window.setTimeout(next, 350);
            return;
          }
          if (i < words.length) {
            controller.enqueue(event("text", words[i++]));
            window.setTimeout(next, 40);
            return;
          }
          controller.enqueue(event("done", JSON.stringify({ output: output, html: html(answer.runs), history: history })));
          controller.close();
        };
        window.setTimeout(next, 250);
      },
    });
    return Promise.resolve(new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } }));
  };
})();
