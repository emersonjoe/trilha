# Spec 062 — Markdown de modelo e a ponte de chat

Issue: [#72](https://github.com/emersonjoe/trilha/issues/72) (`ui.Markdown`, `ai.Serve`,
`ui.Chat`). A issue é a fonte do escopo; aqui fica só a decisão.

## Decisões

1. **`ui.Markdown` devolve `h.Node`, não string de HTML.** É a decisão inteira. Uma função que
   devolve string acaba num `h.Raw`, e aí a segurança depende de alguém não errar; devolvendo
   árvore, o escape é estrutural — o `h` escapa texto e atributo por construção, e não existe
   caminho em que a saída deixe de ser escapada.
2. **HTML cru no texto é texto.** Não há modo permissivo, nem opção. O texto vem de um modelo
   ou de uma pessoa: `<script>` no meio do resumo é `&lt;script&gt;` na tela, sempre.
3. **Link só de esquema conhecido.** `http`, `https`, `mailto` e caminho relativo começando por
   `/` ou `#`. `javascript:`, `data:` e qualquer outro esquema perdem o link e viram texto —
   virar `<a href="">` seria dar um link quebrado onde havia um ataque. Link externo sai com
   `rel="noopener nofollow ugc"` e `target` fica a cargo de quem chama.
4. **Títulos rebaixados por padrão.** `#` vira `<h3>` (`MarkdownOpts{HeadingBase: 3}`): o
   texto do modelo mora dentro de uma página que já tem um `<h1>`, e um `#` no meio de uma
   resposta não pode virar o título da tela.
5. **Imagem desligada por padrão.** `![alt](url)` vira o texto do alt até que
   `MarkdownOpts{Images: true}` diga o contrário — e mesmo assim a URL passa pela mesma
   validação do link. Imagem é requisição para fora, e o texto que a pede não é confiável.
6. **O renderizador do site continua sendo o do site.** O `site/internal/md` fala outro
   dialeto — `:::callout`, `@demo`, âncora em título, realce de Go — e come conteúdo do
   repositório, que é confiável. Fundir os dois só teria dois fins: arrastar recurso de site
   para dentro do framework, ou tirar recurso do site. O que passa a valer é o contrário: o
   `ui.Markdown` é o caminho para texto **não confiável**, e é isso que a referência diz.
7. **`ai.Serve` é o contrato de eventos, não uma política.** Ele lê `{message, history}`, roda
   o agente e emite `text`, `tool_call`, `tool_result`, `handoff`, `done` e `error` — os
   mesmos nomes que o exemplo já emitia, agora com dono. Quem escolhe agente, cliente e teto
   de histórico é o app, por parâmetro.
8. **Sem `Accept: text/event-stream`, `ai.Serve` responde JSON de uma vez.** É a mesma rota e
   o mesmo handler: quem não tem script manda o formulário, o servidor roda o agente inteiro e
   devolve a resposta completa. Não há um segundo caminho para manter vivo.
9. **`ui.Chat` desenha e o `ui.chat.js` conversa.** Bolhas, `aria-live="polite"`, linha de
   passo para ferramenta, campo e botão. Durante o stream o texto entra como texto; ao fim da
   mensagem, o pedaço vai ao servidor e volta renderizado — o Markdown é do servidor, e o
   cliente nunca monta HTML a partir do que o modelo escreveu.
10. **Histórico é do app.** `ChatOpts.History` desenha o que já houve; o framework não guarda
    conversa, não abre sessão de chat e não numera turno. O `done` devolve as mensagens e o
    app decide o que fazer com elas.

## Critérios de aceitação

- SC-001 Golden do `ui.Markdown` para um corpus fixo: parágrafo, ênfase, título, lista,
  citação, código inline e cercado, tabela GFM, link e quebra.
- SC-002 `<script>`, `<img onerror=…>` e `</p>` no texto saem escapados, nunca como tag.
- SC-003 `[x](javascript:alert(1))` e `[x](data:text/html,…)` saem como texto, sem `<a>`.
- SC-004 Fuzz: nenhuma entrada produz `<script`, `javascript:` ou ` on…=` na saída, e não há pânico.
- SC-005 `#` vira `<h3>` por padrão e `HeadingBase` muda o nível; `######` não passa de `<h6>`.
- SC-006 `![alt](u)` é texto por padrão e `<img>` com `Images: true`, com a URL validada.
- SC-007 Código cercado dentro de item de lista e tabela GFM saem certos.
- SC-008 `ai.Serve` emite `text`, `tool_call`, `tool_result`, `done` na ordem, contra um
  `httptest.Server` que devolve deltas.
- SC-009 Erro do modelo vira evento `error` e o stream fecha sem pânico.
- SC-010 Sem `Accept: text/event-stream`, `ai.Serve` devolve JSON com a resposta inteira.
- SC-011 `ui.Chat` rende bolhas do histórico, o formulário e o `aria-live`; `ChatScript` carrega o `ui.chat.js`.
- SC-012 `ui.chat.js` está no embed, no `Files` e sai no `trilha ui`.
- SC-013 `examples/assistente` não tem JavaScript próprio e o `route.go` do chat é uma chamada.
- SC-014 `make test` verde, `api/current.txt` regravado, docs nas duas línguas.
