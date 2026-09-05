// Tema claro/escuro (a preferência do sistema vale quando não há escolha) e
// botão "copiar" nos blocos de código. Tudo funciona sem este arquivo.
(function () {
  var root = document.documentElement;
  var btn = document.querySelector("[data-tema-toggle]");
  function atual() {
    var t = root.getAttribute("data-tema");
    if (t) return t;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "escuro" : "claro";
  }
  if (btn) btn.addEventListener("click", function () {
    var novo = atual() === "escuro" ? "claro" : "escuro";
    root.setAttribute("data-tema", novo);
    root.classList.toggle("dark", novo === "escuro");
    try { localStorage.setItem("tema", novo); } catch (e) {}
  });
  document.querySelectorAll(".codigo").forEach(function (bloco) {
    var b = document.createElement("button");
    b.type = "button"; b.className = "copiar"; b.textContent = "Copiar";
    b.addEventListener("click", function () {
      var code = bloco.querySelector("code");
      navigator.clipboard.writeText(code ? code.innerText : "").then(function () {
        b.textContent = "Copiado"; setTimeout(function () { b.textContent = "Copiar"; }, 1500);
      });
    });
    bloco.appendChild(b);
  });

  // Demos de formulário: o site é exportado estático, então o envio é simulado
  // aqui e mostra o fluxo POST → 303 → GET que o Trilha faria no servidor.
  function slug(s) {
    return s.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase()
      .replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "").slice(0, 40) || "sem-nome";
  }
  document.querySelectorAll('form[data-demo="form"]').forEach(function (f) {
    var saida = f.parentNode.querySelector("[data-demo-saida]");
    f.addEventListener("submit", function (e) {
      e.preventDefault();
      var campo = f.querySelector("#nome");
      var nome = (campo && campo.value || "").trim();
      if (!nome) {
        saida.textContent = "O campo é obrigatório: o navegador barra antes do POST.";
        return;
      }
      saida.textContent =
        'POST /eventos/novo (_csrf conferido) → 303 See Other → GET /eventos/' + slug(nome);
      saida.classList.add("demo-nota-ok");
    });
  });
})();
