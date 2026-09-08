# Plano 057 — O teto da interatividade

## Onde cada coisa mora

| Arquivo | O que muda |
|---|---|
| `ui/ui.go` | `Indicator`, `PendingAfter`, `NoTransition` — três atributos, nenhum estado |
| `ui/assets/ui.js` | limiar, marcação em três lugares, guarda de voo, transição, montagem de ilha |
| `ui/assets/ui.nav.js` | o mesmo limiar e a mesma transição na navegação |
| `ui/assets/ui.css` | indicador escondido por padrão, gatilho em espera, `::view-transition` |
| `island.go` | comentário do loader dizendo quem mais monta, e por quê |
| `site/internal/docs/docs.go` | dois slugs novos por locale, na mesma posição |
| `site/internal/docs/content/{en,pt}/…` | as duas páginas novas, e a de interatividade apontando para elas |
| `examples/blog/app/…` | a rota com a ilha de arrastar e soltar |
| `api/current.txt` | os três símbolos novos (`make api`) |

## Decisões

**O limiar vale para o `aria-busy` também.** Ele é o que pisca hoje. Manter o `aria-busy`
imediato e atrasar só o indicador novo deixaria metade do problema em pé.

**Quem monta a ilha que chega por troca é o `ui.js`, não o loader.** A troca só acontece pelo
`ui.js` — é ele que faz o `fetch` e escreve o `outerHTML` —, então é lá que a ilha nova pode
ser vista com certeza. O loader embutido continua para a página sem o kit. Os dois marcam
`data-trilha-mounted`, então a coexistência não monta duas vezes. A alternativa — executar os
`<script>` que vêm no fragmento — foi recusada: o nonce não sobrevive à inserção por
`outerHTML`, então a CSP padrão recusaria, e executar script vindo de fragmento é uma porta
que não vale abrir por um caso.

**A transição é ligada por padrão.** Ela não existe onde o navegador não tem
`startViewTransition`, e se desliga sozinha com `prefers-reduced-motion`. `ui.NoTransition()`
é a saída para quem não quer.

**A guarda de voo é por alvo, não por gatilho.** Dois botões que trocam o mesmo alvo são o
mesmo pedido em disputa; a guarda por alvo cobre os dois, a guarda por gatilho não.

## Riscos

- **Mudar quando o `aria-busy` aparece** é mudança visível para quem já estilizou em cima
  dele. Vai no `CHANGELOG` como mudança de comportamento, não como correção escondida.
- **`startViewTransition` sobre um `outerHTML`** exige que a troca aconteça dentro do
  callback e que o resultado seja aguardado (`updateCallbackDone`), senão o `swap` devolve
  `undefined` e o chamador navega de verdade sem precisar.

## Complexity Tracking

Nada a justificar: nenhum princípio é violado, e a spec não inventa convenção em `app/`.
