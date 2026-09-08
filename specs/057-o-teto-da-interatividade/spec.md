# Spec 057 — O teto da interatividade

- **Issues**: #81, #82, #83, #84 — as issues são a fonte do escopo; aponte para elas, não as
  reescreva aqui. Milestone: *Fase 8 — O teto da interatividade*.
- **Branch**: `057-o-teto-da-interatividade`
- **Versão**: 0.40.0

## Por quê

Um relato de campo: alguém que veio de anos de React, rodou Go + templ + htmx seis meses em
produção e escreveu o que aprendeu. Os elogios descrevem o que o Trilha já é — HTML tipado no
servidor, troca de pedaço em vez de store no cliente — e por isso valem menos que as queixas.

A queixa concreta é uma só: **em conexão lenta faltava resposta visual enquanto o servidor
responde**, e ele resolveu com `hx-indicator` e transições de CSS. Hoje o Trilha responde pela
metade — `aria-busy` no alvo, que esmaece, e nada mais. Não dá para pôr o sinal em outro
elemento, não há limiar (então a resposta de 40 ms faz o indicador piscar, que é o próprio
"flash" que ele descreve), nada impede o segundo clique, e a troca aparece num quadro, sem
transição.

A outra queixa é o teto: estado rico no cliente — edição colaborativa, arrastar e soltar
contínuo — não cabe no modelo de trocas, e a saída dele foi um componente Alpine ao lado do
htmx. O Trilha tem a saída (`Ctx.Island`), mas nunca diz que o teto existe. E, ao conferir,
ela está quebrada no caso que mais importa: a ilha que **chega dentro de um fragmento
trocado** não monta quando a página não tinha nenhuma ilha antes (#82) — em silêncio, e o
sintoma some quando você recarrega para conferir.

## O que muda

### Enquanto o servidor responde (#81)

```go
ui.Indicator("lista")   // este elemento só aparece enquanto "lista" espera resposta
ui.PendingAfter(200)    // limiar em ms no gatilho; padrão 120
ui.NoTransition()       // desliga a transição de troca neste gatilho
```

Passado o limiar com o pedido ainda no ar, o cliente marca `data-trilha-pending` em três
lugares — o alvo, o gatilho e todo `ui.Indicator` daquele alvo — e o remove ao assentar. O
`aria-busy` do alvo passa a obedecer ao mesmo limiar: é ele que piscava. Um segundo gatilho
para um alvo que já está no ar é ignorado, então o `POST` não sai duas vezes.

A substituição do DOM passa por `document.startViewTransition` onde existe, e é ignorada onde
não existe ou quando o sistema pede menos movimento (`prefers-reduced-motion`).

Eventos no `document` para quem quiser fazer diferente:

```js
document.addEventListener("trilha:pending", (e) => e.detail.target)
document.addEventListener("trilha:settled", (e) => e.detail.target)
```

### A ilha que chega por troca (#82)

Uma ilha monta ao entrar no documento, tendo vindo pela página inteira ou por uma troca, sem
depender de a página já ter outra ilha. Como a troca só acontece pelo `ui.js`, é ele quem
passa a montar o que chega; o loader embutido do `Ctx.Island` continua existindo para a página
que não usa o kit. Os dois são idempotentes (`data-trilha-mounted`), então a ilha monta uma
vez só quando ambos estão presentes.

### O teto, dito (#83) e a tradução para quem vem do htmx (#84)

Duas páginas novas em `aprender/`, nas duas línguas, e uma rota em `examples/blog` com uma
ilha de arrastar-e-soltar — estado contínuo no cliente, ordem gravada no servidor pelo mesmo
`POST` que o resto do app usa.

## Fora de escopo

- **`hx-swap-oob`** (trocar outro pedaço que não o alvo) — pedido legítimo, issue própria; a
  página do #84 diz que não existe, com o motivo, em vez de fingir.
- **Barra de progresso no topo da página** — resolve o mesmo problema por cima; o indicador
  por alvo é mais preciso e não exige lugar fixo no layout.
- **Ilhas de segunda geração** (props tipadas em `.d.ts`, canal ilha→servidor) — é o
  [#70](https://github.com/emersonjoe/trilha/issues/70), fase própria.
- **Empacotar biblioteca de terceiros** — o exemplo mostra o padrão com módulo ES servido de
  `public/`; vendorizar biblioteca no repositório é outra decisão.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nenhum `require` novo; o cliente continua sendo `ui.js` sem dependência, e a transição usa API do navegador com recuo silencioso. |
| IV — contrato pequeno e estável | Três símbolos novos em `ui`, todos atributos compostos como qualquer `h.Node`; nada removido, `api/current.txt` só cresce. |
| VI — teste primeiro | Teste do `ui` para os atributos, teste do `island` para o loader, rota no `examples/blog` com teste de integração. |
| VII — segurança por padrão | O loader continua com nonce; nenhum script novo é executado a partir de fragmento. |
| Estilo/idioma | Duas páginas novas em `en/` e `pt/`, na mesma posição, no mesmo commit. |

## Aceitação

- **SC-001** Um alvo que responde em menos que o limiar nunca acende indicador nem esmaece.
- **SC-002** Passado o limiar, alvo, gatilho e indicador do alvo ficam com `data-trilha-pending`,
  e voltam ao normal ao assentar — inclusive quando a resposta é erro.
- **SC-003** O segundo clique num gatilho cujo alvo já está no ar não dispara segundo pedido.
- **SC-004** Uma ilha que chega dentro de um fragmento monta, mesmo que a página não tivesse
  nenhuma ilha; e não monta duas vezes quando o loader embutido também está presente.
- **SC-005** As duas páginas novas existem nas duas locales e `TestLocalesInSync` passa.
- **SC-006** A rota do exemplo com a ilha de arrastar e soltar responde e é coberta pelo teste
  de integração do `examples/blog`.
