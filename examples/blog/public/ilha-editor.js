// Ilha do editor: contagem e pré-visualização enquanto se escreve.
//
// O módulo é carregado só nesta página, e só porque a página declarou a ilha
// com c.Island. Sem ele, o formulário continua sendo um formulário: o textarea
// é o mesmo, o POST é o mesmo, e nada aqui é obrigatório para publicar.
export default function (el, props) {
  const area = el.querySelector("textarea");
  const info = el.querySelector("[data-info]");
  const previa = el.querySelector("[data-previa]");
  if (!area || !info || !previa) return;

  const ppm = props.palavrasPorMinuto || 200;
  const atualiza = () => {
    const texto = area.value.trim();
    const palavras = texto ? texto.split(/\s+/).length : 0;
    const minutos = Math.max(1, Math.round(palavras / ppm));
    info.textContent = `${palavras} palavra${palavras === 1 ? "" : "s"} · ${minutos} min de leitura`;
    previa.replaceChildren(
      ...texto.split(/\n{2,}/).filter(Boolean).map((p) => {
        const n = document.createElement("p");
        n.textContent = p;
        return n;
      }),
    );
    previa.hidden = palavras === 0;
  };

  info.hidden = false;
  area.addEventListener("input", atualiza);
  atualiza();
}
