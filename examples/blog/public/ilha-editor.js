// Ilha do editor: contagem, pré-visualização e rascunho enquanto se escreve.
//
// O módulo é carregado só nesta página, e só porque a página declarou a ilha
// com c.Island. Sem ele, o formulário continua sendo um formulário: o textarea
// é o mesmo, o POST é o mesmo, e nada aqui é obrigatório para publicar.
//
// O terceiro argumento é o caminho de volta ao servidor. Ele já leva o token
// CSRF que a resposta escreveu no elemento, já traz de volta os erros por
// campo de um 422, e o signal dele é abortado quando o elemento sai da página.
/** @type {import("/islands.d.ts").IslandMount<"/ilha-editor.js">} */
export default function (el, props, island) {
  const area = el.querySelector("textarea");
  const info = el.querySelector("[data-info]");
  const previa = el.querySelector("[data-previa]");
  const aviso = el.querySelector("[data-rascunho]");
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

  // O rascunho vai por island.post: JSON com o token junto, e o 422 volta com
  // os mesmos erros por campo que o formulário mostraria.
  const salva = async () => {
    if (!props.rascunho || !aviso) return;
    const titulo = el.closest("form")?.querySelector("[name=titulo]");
    try {
      const r = await island.post(props.rascunho, {
        titulo: titulo ? titulo.value : "",
        corpo: area.value,
      });
      aviso.textContent = `rascunho salvo · ${r.palavras} palavras`;
    } catch (e) {
      if (e.name === "IslandInvalid") {
        aviso.textContent = e.fields.titulo || e.fields.corpo || "rascunho incompleto";
      } else if (e.name !== "AbortError") {
        aviso.textContent = "não deu para salvar o rascunho";
      }
    }
    aviso.hidden = false;
  };

  let timer = 0;
  info.hidden = false;
  area.addEventListener("input", () => {
    atualiza();
    clearTimeout(timer);
    timer = setTimeout(salva, 1500);
  });
  // O signal fecha o que ficou pendente quando a ilha sai da página.
  island.signal.addEventListener("abort", () => clearTimeout(timer));
  atualiza();
}
