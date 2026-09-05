// Dependent select: when a UF changes, load its cities from the API.
// (Everything else — conditional fields, toast, theme — comes from ui.js.)
document.addEventListener("change", async (e) => {
  const uf = e.target.closest("select[data-cidades]");
  if (!uf) return;
  const cidade = document.getElementById(uf.getAttribute("data-cidades"));
  cidade.innerHTML = "";
  cidade.disabled = true;
  if (!uf.value) {
    cidade.append(new Option("Escolha a UF primeiro", "", true, true));
    return;
  }
  try {
    const res = await fetch("/api/cidades?uf=" + encodeURIComponent(uf.value));
    const list = res.ok ? await res.json() : [];
    cidade.append(new Option("Escolha…", "", true, true));
    for (const c of list) cidade.append(new Option(c, c));
    cidade.disabled = false;
    cidade.focus();
  } catch {
    cidade.append(new Option("Falha ao carregar", "", true, true));
  }
});
