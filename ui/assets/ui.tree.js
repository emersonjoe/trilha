// Kit ui do Trilha — árvore hierárquica. Carregado só por `ui.TreeScript`.
// Sem ele a árvore continua abrindo: cada nó é um <details>. O que o script
// acrescenta é buscar os filhos na primeira abertura, a busca e o teclado.
(() => {
  const TREE = "data-ui-tree", SRC = "data-ui-tree-src", PENDING = "data-ui-tree-pending";
  const PICKER = "data-ui-tree-picker", SEARCH = "data-ui-tree-search", MIN = "data-ui-tree-min";

  const items = (root) => [...root.querySelectorAll('[role="treeitem"]')].filter(visivel);

  // visivel é o que a pessoa alcança com a seta: um nó dentro de um ramo
  // fechado existe no documento e não está na tela.
  const visivel = (el) => {
    for (let p = el.parentElement; p; p = p.parentElement) {
      if (p.tagName === "DETAILS" && !p.open) return false;
    }
    return true;
  };

  const foca = (el) => {
    if (!el) return;
    el.tabIndex = 0;
    el.focus();
  };

  // ---- os filhos, na primeira abertura -------------------------------------

  const carrega = async (det, src) => {
    const vazio = det.querySelector(`[${PENDING}]`);
    if (!vazio || det.dataset.uiTreeLoading) return;
    det.dataset.uiTreeLoading = "1";
    try {
      const url = src + (src.includes("?") ? "&" : "?") + "parent=" + encodeURIComponent(det.dataset.value || "");
      const res = await fetch(url, { headers: { "Trilha-Fragment": "tree" }, credentials: "same-origin" });
      if (!res.ok) throw new Error(res.status);
      vazio.outerHTML = await res.text();
    } catch {
      // Um ramo que não abriu diz isso onde a pessoa está olhando, em vez de
      // ficar eternamente vazio e parecer um ramo sem filhos.
      vazio.textContent = vazio.getAttribute("data-ui-tree-fail") || "…";
      delete det.dataset.uiTreeLoading;
      return;
    }
    delete det.dataset.uiTreeLoading;
  };

  document.addEventListener("toggle", (e) => {
    const det = e.target;
    if (!det.matches?.('details[role="treeitem"]')) return;
    det.setAttribute("aria-expanded", det.open ? "true" : "false");
    const tree = det.closest(`[${TREE}]`);
    const src = tree?.getAttribute(SRC);
    if (det.open && src) carrega(det, src);
  }, true);

  // ---- o teclado -----------------------------------------------------------

  document.addEventListener("keydown", (e) => {
    const tree = e.target.closest?.(`[${TREE}]`);
    if (!tree) return;
    const atual = e.target.closest('[role="treeitem"]');
    if (!atual) return;
    const lista = items(tree);
    const i = lista.indexOf(atual);
    const ramo = atual.tagName === "DETAILS";
    let alvo = null;
    switch (e.key) {
      case "ArrowDown": alvo = lista[i + 1]; break;
      case "ArrowUp": alvo = lista[i - 1]; break;
      case "ArrowRight":
        if (ramo && !atual.open) { atual.open = true; return e.preventDefault(); }
        alvo = lista[i + 1];
        break;
      case "ArrowLeft":
        if (ramo && atual.open) { atual.open = false; return e.preventDefault(); }
        alvo = atual.parentElement?.closest('[role="treeitem"]');
        break;
      case "Home": alvo = lista[0]; break;
      case "End": alvo = lista[lista.length - 1]; break;
      case "*":
        for (const el of tree.querySelectorAll('details[role="treeitem"]')) el.open = true;
        return e.preventDefault();
      default: return;
    }
    if (!alvo) return;
    e.preventDefault();
    for (const el of lista) el.tabIndex = -1;
    foca(alvo);
  });

  // O primeiro nó é o único que entra na ordem de tabulação: a árvore é uma
  // parada só, e as setas andam dentro dela.
  const arma = (tree) => {
    const lista = items(tree);
    if (lista.length) lista[0].tabIndex = 0;
  };

  // ---- a busca -------------------------------------------------------------

  const busca = async (picker, campo) => {
    const src = picker.getAttribute(SEARCH), tree = picker.querySelector(`[${TREE}]`);
    if (!src || !tree) return;
    const q = campo.value.trim();
    if (q.length < (parseInt(picker.getAttribute(MIN), 10) || 2)) {
      if (picker.dataset.uiTreeOriginal) {
        tree.innerHTML = picker.dataset.uiTreeOriginal;
        delete picker.dataset.uiTreeOriginal;
        arma(tree);
      }
      return;
    }
    // A árvore inteira volta quando a busca é apagada; guardá-la aqui é mais
    // barato que pedi-la de novo ao servidor.
    if (!picker.dataset.uiTreeOriginal) picker.dataset.uiTreeOriginal = tree.innerHTML;
    try {
      const res = await fetch(src + (src.includes("?") ? "&" : "?") + "q=" + encodeURIComponent(q),
        { headers: { "Trilha-Fragment": "tree" }, credentials: "same-origin" });
      if (!res.ok) return;
      tree.innerHTML = await res.text();
      arma(tree);
    } catch { /* a árvore que já está na tela continua servindo */ }
  };

  document.addEventListener("input", (e) => {
    const picker = e.target.closest?.(`[${PICKER}]`);
    if (!picker || e.target.type !== "search") return;
    clearTimeout(picker.dataset.uiTreeTimer);
    picker.dataset.uiTreeTimer = setTimeout(() => busca(picker, e.target), 250);
  });

  // O Enter no campo de busca não envia o formulário: quem está procurando um
  // nó ainda não escolheu nenhum.
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Enter") return;
    const picker = e.target.closest?.(`[${PICKER}]`);
    if (picker && e.target.type === "search") e.preventDefault();
  });

  const start = () => { for (const t of document.querySelectorAll(`[${TREE}]`)) arma(t); };
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", start);
  else start();
  document.addEventListener("trilha:swap", start);
})();
