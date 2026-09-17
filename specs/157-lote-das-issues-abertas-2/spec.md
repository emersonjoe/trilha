# Spec 157 — o segundo lote das issues abertas

- **Issues**: [#251](https://github.com/emersonjoe/trilha/issues/251),
  [#252](https://github.com/emersonjoe/trilha/issues/252),
  [#253](https://github.com/emersonjoe/trilha/issues/253),
  [#254](https://github.com/emersonjoe/trilha/issues/254),
  [#255](https://github.com/emersonjoe/trilha/issues/255),
  [#256](https://github.com/emersonjoe/trilha/issues/256),
  [#257](https://github.com/emersonjoe/trilha/issues/257),
  [#258](https://github.com/emersonjoe/trilha/issues/258),
  [#259](https://github.com/emersonjoe/trilha/issues/259),
  [#260](https://github.com/emersonjoe/trilha/issues/260),
  [#261](https://github.com/emersonjoe/trilha/issues/261),
  [#262](https://github.com/emersonjoe/trilha/issues/262),
  [#263](https://github.com/emersonjoe/trilha/issues/263),
  [#264](https://github.com/emersonjoe/trilha/issues/264),
  [#265](https://github.com/emersonjoe/trilha/issues/265) — cada issue é a fonte do
  próprio escopo (reprodução, medição e proposta estão lá); aqui fica só a decisão.
- **Branch**: `157-lote-das-issues-abertas-2`
- **Versão**: 0.136.0

## Por quê

Quinze issues abertas em dois dias, quase todas da mesma origem: a migração do Acervo
(sessões S14–S17) passou a medir as telas por captura a 375 px e a cruzar o HTML pronto com
os seletores de `ui.css`. O que apareceu é da família que teste de conteúdo não vê — o HTML
está certo e a página está errada: a lateral recolhida que zera a largura da página, o alerta
sem ícone que vira duas colunas, a régua de abas cortada no telefone, o plural que o
`relativeText` nunca fez, o `<a>` cru no resultado da busca. Ao lado, dois cabeçalhos que
morrem na borda em silêncio (`X-Robots-Tag` no `Pipe`, o `frame-ancestors` do `Inline` quando
a CSP é escrita à mão) e o `trilha add` que aceita três receitas e escreve uma.

Como na spec 155, uma release fecha o conjunto: publicar quinze versões de uma linha
fragmentaria o contrato e deixaria quem lê o CHANGELOG sem o fio que liga as correções.

## O que muda

**Shell (#255, #256).** `html.ui-sidebar-collapsed .ui-shell` passa a `grid-template-columns:
1fr` — uma lateral com `display: none` não é item de grade, e a faixa de `0` era a página. A
preferência de desktop e a gaveta do telefone deixam de ser o mesmo sinalizador:
`ui-sidebar-collapsed` (persistida em `localStorage`) só vale a partir de 768 px;
abaixo disso a gaveta é `ui-drawer-open` no `<html>`, que não é gravada, nasce fechada,
fecha ao tocar fora (véu sobre o conteúdo), ao tocar num link do menu e com Escape. A lateral
ganha um botão de fechar visível só no telefone (`data-ui-sidebar-toggle`, o mesmo atributo
que o `ui.js` já lê). O script inline do `ui.Head` deixa de ligar a classe por padrão abaixo
de 768 px — isso agora é a CSS.

**Alerta, cartão, abas, grade, busca (#257, #260, #261, #263, #264, #265).**
- `.ui-alert` sem `.ui-icon` filho é uma coluna só (`:has()`); título em cima, descrição
  embaixo.
- `.ui-card-header` com um `.ui-btn` ou `.ui-badge` filho direto ganha a segunda coluna, com a
  ação à direita do título e alinhada ao centro.
- `.ui-tabs-list` rola de lado (`max-width: 100%; overflow-x: auto`).
- `ui.Cols(n...)` é um marcador para `ui.Grid`: `ui.Grid(ui.Cols(2, 4), ...)` dá duas colunas
  no telefone e quatro a partir de 1024 px. Um número só vale para todas as faixas; sem
  `Cols`, o `auto-fit` de hoje. O marcador vira `--ui-cols` e `--ui-cols-lg` no `style` do
  elemento; a CSS lê as variáveis.
- `SearchBoxOpts.Submit string` desenha um botão de enviar depois do campo; vazio, nenhum. O
  `ui.Kbd` do atalho não encolhe mais (`flex: none; white-space: nowrap`).
- As classes que o kit escrevia e a folha não desenhava: `ui-badge-sm` (selo menor),
  `ui-field-display` (o valor só de leitura alinhado ao campo ao lado) e as quatro do
  `ui.SearchResults` (`-results`, `-hit`, `-kind`, `-title`) ganham regra. `ui-assistant`,
  `ui-btn-primary`, `ui-deadline-card`, `ui-shell-user` e `ui-tree-branch` são ganchos de
  propósito — a referência passa a dizer isso, e um teste do pacote `ui` confere que toda
  classe escrita em `ui/*.go` tem regra em `ui.css` ou está na lista de ganchos.

**PageHeader (#262).** `ui.Subtitle(string)` é um marcador irmão de `ui.Back`: `PageHeader`
o desenha como `<p class="ui-page-subtitle">` abaixo da linha do título, e nunca entre as
ações.

**Texto relativo (#258).** Em pt-BR, `mês`/`meses` e `ano`/`anos` flexionam com `n`;
`min`, `h` e `d` continuam abreviações.

**Bars (#259).** Já resolvida na 0.129.0 pela spec 150 (#195): o viewBox cresce com o maior
`Datum.Text`. Fecha junto, sem mudança de código; o teste `TestBarsWidensForTheLongestValue`
é a prova.

**Pipe (#251).** `X-Robots-Tag` entra em `pipeHeaders`. `Content-Security-Policy` fica de fora
de propósito, e o comentário da lista diz por quê. `Config.PipeHeaders []string` acrescenta
nomes à lista — nunca a substitui; `Set-Cookie` continua sem viajar mesmo que alguém o liste.

**CSP (#252).** `Security.CSPRemove map[string][]string` tira tokens de uma diretiva da
política padrão (`{"style-src": {"'unsafe-inline'"}}`) sem abandonar o padrão, então
`Ctx.Inline` continua relaxando `frame-ancestors`. Uma diretiva que fica sem token sai como
`'none'`.

**`trilha add` (#253, #254).** Aceita várias receitas na mesma chamada, aplicadas em ordem
com os arquivos de cada uma sob o próprio nome; `--lang` e os outros flags valem em qualquer
posição. Todos os nomes são resolvidos antes de escrever qualquer arquivo — `trilha add login
typo` recusa sem tocar no projeto. O `--lang pt` que se perdia era este: o flag depois do
segundo nome nunca era lido, os caminhos das receitas são em português de nascença, e o
resultado parecia uma receita que sabia o idioma e não o usava.

## Fora de escopo

- **`ui.CardAction`** como API (#260): a CSS resolve o caso sem símbolo novo; um marcador
  entra quando aparecer um cabeçalho que a regra por `:has()` não cubra.
- **Repassar a CSP do upstream** (#251): decisão maior que a lista, e a nota no comentário
  já diz que ficou de fora de propósito.
- **Transição animada da lateral** (#255): a faixa de `0` existia para isso e nunca animou;
  o certo hoje é não ter faixa.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | nada novo; `:has()` é CSS, não dependência |
| VI — teste primeiro | um teste que falha por issue, no pacote que a corrige |
| VII — segurança por padrão | `CSPRemove` só subtrai do que a app pediu; `PipeHeaders` só acrescenta, e `Set-Cookie` continua fora |

## Tarefas

- [ ] T001 Kit: testes que falham em `ui/` (shell, alert/card/tabs via classes, `Cols`,
      `Subtitle`, `Submit`, plural, classes órfãs) e as correções em `ui.css`, `ui.js`,
      `ui.go`, `shell.go`, `search.go`, `format.go`
- [ ] T002 Runtime: `TestPipeCarriesXRobotsTag`/`Config.PipeHeaders` em `send_test.go`;
      `TestCSPRemoveKeepsInlineFraming` em `security_test.go`
- [ ] T003 CLI: `parseAddArgs` com teste unitário; `TestAddSeveralRecipesE2E`
- [ ] T004 `trilha ui` nos exemplos e no site; `make golden`; `make api`
- [ ] T005 Referência nas duas locales (`shell`, `ui`, `ctx`, `security`, `cli`)
- [ ] T006 `CHANGELOG.md`, `version`, `ROADMAP.md`
- [ ] T007 `make test` verde e `make release VERSION=0.136.0 ISSUES="251 … 265"`

## Aceitação

- **SC-001** Cada issue da lista tem um teste que falhava antes e passa depois, ou (#259) o
  teste que já provava a correção nomeado no CHANGELOG.
- **SC-002** `trilha add login users audit --lang pt` escreve as três receitas, em
  português, e `trilha add login typo` não escreve nada.
- **SC-003** `make test` verde; os `public/ui.css` dos exemplos e do site iguais ao embutido.
