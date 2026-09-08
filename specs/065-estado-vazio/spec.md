# Spec 065 — O estado vazio

- **Issue**: #97 — a issue é a fonte do escopo; aponte para ela, não a reescreva aqui.
- **Branch**: `065-empty`
- **Versão**: 0.47.0

## Por quê

Trinta e cinco telas do app medido têm um estado vazio escrito à mão, e não há dois iguais.
Quem começa escreve um `<p>` e fica por isso — e o `<p>` diz que a tela está vazia sem dizer o
que fazer a respeito.

Pior que isso: nove telas confundem **lista vazia** com **lista filtrada até o vazio**. Dizer
"nada por aqui" para quem acabou de buscar *xyz* informa que a aplicação está vazia, quando o
que aconteceu é que o termo não casou com nada. São duas telas diferentes, e só uma delas tem
uma saída que não é desistir.

## O que muda

`ui.Empty`, `ui.EmptyOpts` e `ui.EmptyError`, e o `ui.DataTable` passa a desenhar o estado
vazio sozinho — distinguindo os dois casos. O contrato está na referência de listagens, nas
duas línguas.

O que vale registrar:

**A distinção é o ponto.** Sem busca, um ícone e "ainda não há nada". Com `?q=xyz`, "nenhum
resultado para xyz" e um link que limpa o termo **e volta à primeira página** — limpar a busca
mantendo `page=2` levaria a pessoa a outra tela vazia.

**`Hint` é o campo que se paga.** É a diferença entre dizer que a tela está vazia e dizer o
que fazer.

**Ícone desconhecido não desenha nada.** O `ui.Icon` entra em pânico com nome que não existe,
o que é certo numa página que o desenvolvedor está escrevendo e errado num componente que
desenha o que uma opção calhar de ter — e pânico na tela que já está vazia é o pior lugar
possível.

**`EmptyError` mostra o erro real só em desenvolvimento.** A frase de um driver numa página de
produção é vazamento de informação com fonte amigável.

## Fora de escopo

- **Traduzir os textos padrão** — o kit é em inglês por padrão, como todo o resto dele; app em
  português passa o seu `ListState.Empty`, e é o que o exemplo faz.
- **`ui.Poll` com altura mínima** — o `min-height` do CSS já resolve o pulo, sem código.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | Nada novo. |
| IV — superfície pequena | Três símbolos, e o `DataTable` melhora sem mudar assinatura. |
| VI — teste primeiro | A distinção busca-vazia × lista-vazia, os campos opcionais, o ícone inexistente e o erro escondido fora de desenvolvimento. |
| VII — segurança por padrão | O erro real não sai de produção. |
| FR-007 | O CSS ficou 24 bytes abaixo do teto; elevar orçamento por 36 bytes seria absurdo, então enxuguei o que eu mesmo tinha escrito. |

## Tarefas

- [x] T001 `ui/empty.go`: `Empty`, `EmptyOpts`, `EmptyError`, e o padrão do `DataTable`.
- [x] T002 CSS do estado vazio, dentro do orçamento.
- [x] T003 Testes da distinção, dos opcionais, do ícone e do erro.
- [x] T004 `examples/blog` usa os dois; a receita do cookbook volta a bater com o código.
- [x] T005 `AGENTS.md` nas duas línguas e referência de listagens nas duas línguas.
- [ ] T006 `CHANGELOG.md`, `version`, `make test` verde e release.

## Aceitação

- **SC-001** Lista vazia sem busca não oferece limpar busca nenhuma.
- **SC-002** Com `?q=xyz&page=2`, o vazio cita o termo e o link limpa termo e página.
- **SC-003** `ui.Empty` com só o título não desenha ícone, dica nem ação.
- **SC-004** Ícone que não existe não derruba a página.
- **SC-005** O erro real aparece em `Dev` e não aparece em `Prod`.
