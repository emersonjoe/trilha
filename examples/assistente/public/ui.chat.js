/* trilha ui 254825a44313b871 */
// ui.chat.js — the behavior behind ui.Chat: send the message, read the events
// from ai.Serve and render the answer as it arrives. Without this file the
// form still submits and the page comes back with the answer; this only makes
// it arrive word by word.
(() => {
  "use strict";

  const el = (cls, text) => {
    const d = document.createElement("div");
    d.className = cls;
    if (text !== undefined) d.textContent = text;
    return d;
  };

  const setup = (root) => {
    const action = root.getAttribute("data-trilha-chat");
    const steps = root.hasAttribute("data-trilha-chat-steps");
    const id = root.id || "chat";
    const log = document.getElementById(id + "-log");
    const form = document.getElementById(id + "-form");
    const input = document.getElementById(id + "-input");
    const state = document.getElementById(id + "-state");
    if (!action || !log || !form || !input) return;

    let history = [];
    let busy = false;

    const add = (node) => {
      log.appendChild(node);
      log.scrollTop = log.scrollHeight;
      return node;
    };
    const say = (s) => {
      if (state) state.textContent = s;
    };
    const token = () => {
      const f = form.querySelector('input[name="_csrf"]');
      return f ? f.value : "";
    };

    // The page's context: the hidden ctx.* fields the server wrote. They ride
    // with the message here and as plain form fields when this script is not
    // running, so the route reads one thing either way.
    const context = () => {
      const out = {};
      form.querySelectorAll('input[name^="ctx."]').forEach((i) => {
        out[i.name.slice(4)] = i.value;
      });
      return out;
    };

    // bubble keeps the assistant's turn: plain text while it streams, the
    // rendered Markdown when the message ends. It is text/textContent until
    // the very last step, and the HTML that replaces it came from ui.Markdown.
    const bubble = () => {
      const wrap = add(el("ui-msg ui-msg-assistant"));
      const body = el("ui-md");
      wrap.appendChild(body);
      return body;
    };

    const handle = (body, name, data) => {
      if (name === "text") {
        body.textContent += data;
      } else if (name === "tool_call" || name === "tool_result" || name === "handoff") {
        if (!steps) return;
        const d = JSON.parse(data);
        if (name === "tool_call") add(el("ui-chat-step", d.tool + "(" + (d.arguments || "") + ")"));
        else if (name === "tool_result") add(el("ui-chat-step", d.tool + " → " + (d.output || "")));
        else add(el("ui-chat-step", "→ " + (d.to || "")));
      } else if (name === "done") {
        const d = JSON.parse(data);
        // outerHTML, not innerHTML: the rendered answer is already a .ui-md
        // div, the same one the server writes for the history, so the bubble
        // ends up identical to a reloaded page.
        if (d.html) body.outerHTML = d.html;
        else if (d.output) body.textContent = d.output;
        history = d.history || history;
        say("");
      } else if (name === "error") {
        let msg = data;
        try {
          msg = JSON.parse(data).message || data;
        } catch (e) {}
        add(el("ui-msg ui-msg-note", msg));
        say("");
      }
      log.scrollTop = log.scrollHeight;
    };

    form.addEventListener("submit", async (e) => {
      const message = input.value.trim();
      if (!message || busy) return;
      e.preventDefault();
      busy = true;
      input.value = "";
      add(el("ui-msg ui-msg-user", message));
      say("…");
      const button = form.querySelector("button");
      if (button) button.disabled = true;
      const body = bubble();
      try {
        const res = await fetch(action, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Accept: "text/event-stream",
            "X-CSRF-Token": token(),
          },
          body: JSON.stringify({ message: message, history: history, context: context() }),
        });
        if (!res.ok || !res.body) throw new Error("HTTP " + res.status);
        const reader = res.body.getReader();
        const dec = new TextDecoder();
        let buf = "";
        for (;;) {
          const chunk = await reader.read();
          if (chunk.done) break;
          buf += dec.decode(chunk.value, { stream: true });
          let i;
          while ((i = buf.indexOf("\n\n")) >= 0) {
            const block = buf.slice(0, i);
            buf = buf.slice(i + 2);
            let name = "message";
            const data = [];
            for (const line of block.split("\n")) {
              if (line.startsWith("event: ")) name = line.slice(7);
              else if (line.startsWith("data: ")) data.push(line.slice(6));
              else if (line.startsWith("data:")) data.push(line.slice(5));
            }
            handle(body, name, data.join("\n"));
          }
        }
      } catch (err) {
        // The message goes back into the field: the answer was lost, the
        // question was not, and sending it again is one key away.
        if (!body.textContent) body.parentNode.remove();
        add(el("ui-msg ui-msg-note", String(err.message || err)));
        if (!input.value) input.value = message;
        say("");
      } finally {
        busy = false;
        if (button) button.disabled = false;
        input.focus();
      }
    });
  };

  const start = () => document.querySelectorAll("[data-trilha-chat]").forEach(setup);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
})();
