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
})();
