// Sends messages to /api/chat and renders the SSE stream.
(() => {
  const form = document.getElementById("form");
  const input = document.getElementById("entrada");
  const list = document.getElementById("mensagens");
  const estado = document.getElementById("estado");
  let history = [];

  const add = (cls, text) => {
    const el = document.createElement("div");
    el.className = "msg " + cls;
    el.textContent = text;
    list.appendChild(el);
    list.scrollTop = list.scrollHeight;
    return el;
  };

  let bubble = null;
  const handle = (ev, data) => {
    if (ev === "text") {
      if (!bubble) bubble = add("assistente", "");
      bubble.textContent += data;
    } else if (ev === "tool_call") {
      const d = JSON.parse(data);
      add("passo", `🔧 ${d.agent} chamou ${d.tool}(${d.arguments})`);
    } else if (ev === "tool_result") {
      const d = JSON.parse(data);
      add("passo", `↩ ${d.tool}: ${d.output}`);
    } else if (ev === "handoff") {
      const d = JSON.parse(data);
      add("passo", `🤝 transferido para ${d.to}`);
    } else if (ev === "done") {
      const d = JSON.parse(data);
      history = d.history || [];
      estado.textContent = `${d.agent} · ${d.usage.total_tokens || 0} tokens`;
    } else if (ev === "error") {
      const d = JSON.parse(data);
      add("erro", "Erro: " + d.message);
    }
    list.scrollTop = list.scrollHeight;
  };

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const message = input.value.trim();
    if (!message) return;
    input.value = "";
    add("usuario", message);
    bubble = null;
    estado.textContent = "pensando…";
    form.querySelector("button").disabled = true;
    try {
      const res = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message, history }),
      });
      if (!res.ok) throw new Error("HTTP " + res.status);
      const reader = res.body.getReader();
      const dec = new TextDecoder();
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += dec.decode(value, { stream: true });
        let idx;
        while ((idx = buf.indexOf("\n\n")) >= 0) {
          const block = buf.slice(0, idx);
          buf = buf.slice(idx + 2);
          let ev = "message";
          const data = [];
          for (const line of block.split("\n")) {
            if (line.startsWith("event: ")) ev = line.slice(7);
            else if (line.startsWith("data: ")) data.push(line.slice(6));
            else if (line.startsWith("data:")) data.push(line.slice(5));
          }
          handle(ev, data.join("\n"));
        }
      }
    } catch (err) {
      add("erro", "Falha: " + err.message);
    } finally {
      form.querySelector("button").disabled = false;
      input.focus();
    }
  });
})();
