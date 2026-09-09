# Spec 069 — Carregar depois

- **Issue**: #98 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `069-defer`
- **Versão**: 0.51.0

## Por quê

Com SSR o esqueleto some, porque a página chega pronta. Mas some por um motivo que o iniciante
reencontra do outro lado: a tela que precisa de sete consultas para desenhar passa a esperar
pela mais lenta das sete. Hoje, no Trilha, ou a página inteira espera os dois segundos, ou o
desenvolvedor descobre sozinho que dá para servir a página e trocar um fragmento depois — com
`ui.Poll` de um tiro só, que não é para isso.

## O que muda

`ui.Defer(c, id, src, opts...)`: o placeholder agora, o fragmento assim que a página carrega.
O contrato está na referência de fragmentos vivos, nas duas línguas. O que vale registrar:

**É a máquina do `Poll` sem o relógio.** Nada de segundo caminho de swap: o elemento é pedido
pelo mesmo `ask`, trocado pelo mesmo `swap`, e o `base` fica em zero, então o `tick` nunca o
pega. É por isso que `Then: ui.Poll("30s", src)` funciona sem uma linha a mais — os atributos
viajam no mesmo elemento e a resposta da rota decide o resto.

**A falha é desenhada no servidor e escondida.** O comportamento só troca o `hidden` entre o
bloco de espera e o de erro. As frases e as classes ficam do lado do Go, o JavaScript não sabe
idioma nenhum, e o buraco nunca fica com um esqueleto pulsando para sempre — que é o que a
issue pediu com o `ui.EmptyError` e é literalmente o `ui.EmptyError` que está lá.

**Sem script, o placeholder carrega um link dentro de `<noscript>`.** Não um botão sempre
visível: o navegador que vai trocar aquilo em um instante não deve mostrar um caminho para sair
da página em que a pessoa já está.

**A altura não é enfeite.** Placeholder mais baixo que o conteúdo faz a página pular debaixo do
cursor de quem já começou a ler. Daí `Height`, com 8rem de padrão.

## Desvios da issue

- **A assinatura leva o `id`**: `ui.Defer(c, "insights", "/painel/insights", ...)`, e não só o
  `src`. O id é o elemento que vai ser trocado e precisa aparecer também na resposta da rota —
  é o mesmo contrato do `ui.Poll`. Derivá-lo do `src` em silêncio significaria ou inventar uma
  regra de nomes que o desenvolvedor teria de adivinhar do outro lado, ou um segundo caminho de
  swap só para o `Defer`, e aí o `Then` deixaria de funcionar.
- **`E_NESTED_DEFER` no `gen` não entra.** O aninhamento é dinâmico: a rota A adia para a B, e a
  página da B tem um `Defer` seu. O scanner não vê através de rotas, então a checagem estática
  pegaria só o caso que ninguém escreve. No lugar dela ficou a garantia que importa: um `Set`
  de ids já pedidos, que impede o fragmento que responde com um `Defer` do próprio id de pedir
  para sempre. O conselho — um `Defer` por parte da página, não um por linha de lista — está na
  referência.
- **Orçamento de 600 bytes**: o `ui.live.js` cresceu 1,2 KB, dos quais cerca de 600 são código
  e o resto são os comentários que o arquivo mantém em todo o kit. O `Defer` não toca no
  `ui.js`, que era onde a issue tinha posto o orçamento.

## Achado fora de escopo, corrigido

O `examples/blog/public/ui.css` estava parado desde a spec 064: a cópia do kit que o exemplo
serve não tinha os tons de enum. O `local-login` tinha uma guarda para um arquivo (`ui.live.js`)
e o blog não tinha nenhuma. Agora o blog confere a estante inteira contra o `ui.Asset`, que é o
que pega esta classe de erro de uma vez — e foi o navegador, não a suíte, que mostrou o
problema: o atributo estava no HTML e o comportamento não vinha.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nenhuma dependência; o comportamento é o `ui.live.js` que já existia. |
| III — rota no exemplo | `/documentos` adia o resumo para `/documentos/resumo`, com e2e: a página vem sem o bloco caro, o fragmento vem sozinho, e a mesma rota responde como página. |
| IV — superfície pequena | Uma função e um struct de opções, todas com padrão. |
| V — inglês no código, pt-BR junto | Referência nas duas línguas no mesmo commit. |
| VI — teste primeiro | Render (placeholder, altura, `Then`, frases do chamador), e2e no exemplo, e o swap conferido no navegador — inclusive o caminho da falha e o "tentar de novo". |
| VII — segurança por padrão | O fragmento vai com a sessão e os cabeçalhos da página; nada de token no HTML, como no `Poll`. |

## Tarefas

1. `ui/defer.go`: `Defer`, `DeferOpts`, o bloco de falha desenhado e escondido. ✅
2. `ui/assets/ui.live.js`: pedir uma vez, `show`/`fail`, o clique do "tentar de novo". ✅
3. Rota `/documentos/resumo` no exemplo e o `Defer` na tela de documentos. ✅
4. Guarda da cópia do kit em `examples/blog/public/`. ✅
5. Referência "Carregar depois" em en e pt; `api/current.txt` e o catálogo do `ui`. ✅
