// Ilha da ordem: arrastar e soltar, que é o teto do modelo de trocas.
//
// Enquanto o ponteiro está pressionado, a verdade sobre a ordem está aqui, no
// cliente — nenhuma ida ao servidor cabe nesse intervalo. É para isso que a
// ilha existe. O que ela não faz é virar dona dos dados: ela escreve a ordem no
// campo escondido e vai embora; quem salva é o mesmo POST do resto do app.
//
// Os botões ↑ ↓ continuam onde estavam. Arrastar não é alcançável pelo teclado,
// e uma ilha que tira acessibilidade não é melhoria.
export default function (el, props) {
  const lista = el.querySelector("[data-lista]");
  const campo = el.querySelector('input[name="ordem"]');
  if (!lista || !campo) return;

  const linhas = () => Array.from(lista.querySelectorAll("li[data-slug]"));
  const grava = () => { campo.value = linhas().map((li) => li.dataset.slug).join(","); };

  // props.ordem é a ordem que o servidor renderizou. Se ela e o DOM
  // discordarem, o DOM é o que a pessoa está vendo: ele ganha.
  if (Array.isArray(props?.ordem) && props.ordem.join(",") !== campo.value) grava();

  let arrastando = null;

  for (const li of linhas()) {
    li.draggable = true;
    li.style.cursor = "grab";
  }

  lista.addEventListener("dragstart", (e) => {
    const li = e.target.closest("li[data-slug]");
    if (!li) return;
    arrastando = li;
    li.setAttribute("aria-grabbed", "true");
    e.dataTransfer.effectAllowed = "move";
    // O Firefox só começa o arrasto se algo for escrito aqui.
    e.dataTransfer.setData("text/plain", li.dataset.slug);
  });

  lista.addEventListener("dragover", (e) => {
    if (!arrastando) return;
    e.preventDefault();
    const alvo = e.target.closest("li[data-slug]");
    if (!alvo || alvo === arrastando) return;
    const meio = alvo.getBoundingClientRect().top + alvo.offsetHeight / 2;
    alvo.parentNode.insertBefore(arrastando, e.clientY < meio ? alvo : alvo.nextSibling);
  });

  const solta = () => {
    if (!arrastando) return;
    arrastando.removeAttribute("aria-grabbed");
    arrastando = null;
    grava();
  };
  lista.addEventListener("drop", (e) => { e.preventDefault(); solta(); });
  lista.addEventListener("dragend", solta);
}
