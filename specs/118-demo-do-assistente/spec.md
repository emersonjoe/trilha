# Spec 118 — A demo executável do ui.Assistant

- **Issue**: [#155](https://github.com/emersonjoe/trilha/issues/155) — a issue é a fonte do escopo.
- **Branch**: `118-demo-do-assistente`
- **Versão**: 0.97.0

## Por quê

O `ui.Assistant` (spec 093) entrou com documentação e com o exemplo autenticado do blog, mas o
que o componente vende é **comportamento**: o link que vira painel, a resposta chegando palavra
por palavra, o contexto da página indo junto, o fallback sem script. Nada disso aparece numa
tabela de campos, e ver funcionando hoje exige clonar o repositório, configurar um modelo e subir
o blog.

O site já tem demos executáveis de botões, formulários e cards (spec 013). A do assistente não
existia — e, se existisse, não abriria: o `site/public/ui.js` ficou atrás do `ui/assets/ui.js` e
não tem o `preventDefault` do launcher que a spec 093 acrescentou.

## O que muda

**Uma página por locale, estática, com o assistente de verdade em cima de uma tela pequena.**

`/demos/assistant` e `/pt/demos/assistant` renderizam uma fatura (card com três fatos) e um
`ui.Assistant` cujo `ChatOpts.Context` leva `invoice_id` e a rota. O launcher aponta para
`#conversation`: a mesma conversa, como `ui.Chat` completo, logo abaixo — o fallback sem script
está na própria página, e não em outra rota.

**O servidor da demo é um script, e o cliente é o do kit.** `assistant-demo.js` intercepta o
`fetch` só para `/_demo/assistant` e responde no contrato SSE do `ai.Serve` — eventos `text`
palavra por palavra e um `done` com `html`, `output` e `history`. O `ui.chat.js` que lê isso é o
mesmo arquivo que roda numa aplicação; a demo não bifurca o cliente para parecer viva.

**Os assets do kit no site voltam a ser os do kit.** `trilha ui` no diretório do site atualiza
`ui.js` e `ui.css` (com carimbo) e traz o `ui.chat.js`; o site só carrega os que usa.

**A referência aponta para a demo.** A seção `Assistant` de `reference/ui.md` e
`referencia/ui.md` ganha o link, EN e PT no mesmo commit.

## Fora de escopo

- **Um modelo de verdade no site.** O site é estático no Pages; a resposta é determinística e
  local por construção, e a página diz isso.
- **Demos em Markdown (`@demo`).** O mecanismo da spec 013 renderiza um nó por locale; o
  assistente precisa de scripts e de um id estável na página inteira, então é página e não card.
- **Guardar a conversa.** Como no `ui.Chat`, o histórico vive no navegador enquanto a página
  está aberta.

## Constitution Check

| Princípio | Como respeita |
|---|---|
| II — só biblioteca padrão | páginas em Go com `h`/`ui`; o script é vanilla, sem dependência |
| VII — segurança por padrão | contexto como atributo escapado; o HTML do `done` é montado pelo próprio script a partir de texto fixo |
| Docs EN/PT no mesmo commit | página, textos e link da referência nos dois locales |
| Teste primeiro | `TestAssistantDemoIsRunnable` cobre os dois caminhos antes da página existir |

## Tarefas

- [x] T001 Teste que falha: as duas rotas respondem com launcher, `ctx.*`, scripts e `hreflang`
- [x] T002 `trilha ui` no site: `ui.js`, `ui.css` atualizados e `ui.chat.js` presente
- [x] T003 `site/internal/assistantdemo` e as páginas `demos/assistant` (en, pt)
- [x] T004 `assistant-demo.js` no contrato SSE do `ai.Serve`, e o CSS da página
- [x] T005 Export das rotas em `Setup`; `trilha gen`
- [x] T006 Link da referência do `ui.Assistant` (en + pt)
- [x] T007 O launcher (`<a class="ui-btn">`) deixa de herdar a cor de `.ui-body a`; cópias do `ui.css` ressincronizadas
- [x] T008 `CHANGELOG.md`, `ROADMAP.md`, `version`, `make test`, `scripts/release.sh 0.97.0`

## Aceitação

- **SC-001** `/demos/assistant` e `/pt/demos/assistant` respondem 200, aparecem em
  `ExportPaths()` e têm `hreflang` cruzado.
- **SC-002** O HTML traz `data-ui-dialog-open`, `name="ctx.invoice_id" value="42"`,
  `data-trilha-chat="/_demo/assistant"` e os scripts `ui.chat.js` e `assistant-demo.js`.
- **SC-003** Sem JavaScript o launcher é `href="#conversation"`, e a seção existe.
- **SC-004** `site/public/ui.js` e `ui.css` são o conteúdo de `ui/assets` com o carimbo do
  `trilha ui`.
- **SC-005** O rótulo do launcher tem a cor de `--primary-foreground` dentro de `.ui-body`, e não
  a tinta da página.
