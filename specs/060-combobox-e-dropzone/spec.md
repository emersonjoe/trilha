# Spec 060 — O campo que busca e a área que recebe arquivos

Issue: [#67](https://github.com/emersonjoe/trilha/issues/67) (`ui.Combobox` servido por
fragmento e `ui.Dropzone` multi-arquivo sobre o upload existente). A issue é a fonte do
escopo; aqui fica só a decisão.

## Por que as duas juntas

São os dois campos que sobraram do `ui`, e os dois têm o mesmo formato de resposta: o
servidor manda HTML e o cliente só mostra. O combobox pede um fragmento de opções ao
servidor; a dropzone manda um arquivo e recebe o fragmento da fila de volta. Nenhum dos dois
inventa um protocolo JSON, e quem escreve a rota escreve `c.HTML` e `c.Render` como em
qualquer outra tela.

## Decisões

1. **`c.Files` antes de tudo.** O `c.File` lê um arquivo; uma dropzone manda N com o mesmo
   nome. `c.Files(campo, regras)` aplica as regras arquivo por arquivo e nomeia a mensagem
   pela posição — `arquivos[2]` —, a mesma chave indexada da spec 059, para a linha errada da
   fila receber a mensagem certa. `Optional` passa a significar "nenhum arquivo serve", e
   `FileRules.MaxFiles` é o teto do servidor.
2. **A fila manda um arquivo por requisição.** Cada arquivo é um `POST`, com a sua barra, a
   sua resposta e a sua mensagem. Um multipart de cinquenta arquivos que morre no quadragésimo
   nono não tem como dizer qual falhou, e o retentar seria do zero. Sem JavaScript o navegador
   manda todos de uma vez, e é por isso que a rota é escrita uma vez só, contra `c.Files`.
3. **O servidor busca; o cliente mostra.** `ComboboxOpts.Source` é uma rota de GET que recebe
   `?q=` e responde `ui.ComboboxOptions(...)`: uma lista de `<li role="option">`. Não há JSON,
   não há esquema de resposta, e a autorização da busca é a da rota, como sempre.
4. **Dois campos: o texto que se vê e o valor escondido.** O texto é o que a pessoa digitou,
   o `<input type=hidden>` é o que o `Bind` lê. Sem JavaScript o texto vai sozinho e o
   servidor resolve — ou devolve 422 com `FieldErrors`, e o rótulo volta porque quem o guarda
   é o `ComboboxOpts.Label` que o app preenche.
5. **Lista pequena não vai ao servidor.** Com `Options` preenchido o filtro é no cliente, no
   mesmo HTML e no mesmo script: um caminho só, e a diferença é só de onde vêm os `<li>`.
6. **O contexto viaja junto.** `With: []string{"uf"}` manda o valor de outros campos do mesmo
   formulário na busca (`?q=…&uf=SP`), que é o que faz um seletor dependente deixar de ser
   vinte linhas de JavaScript no app.
7. **A dropzone é um formulário.** `<input type=file multiple>` dentro de um `<form
   enctype=multipart/form-data>`; a área de arrastar é o `<label>` do mesmo input. Sem
   JavaScript o botão de enviar manda tudo; com JavaScript o `ui.upload.js` toma conta da
   fila.
8. **`Accept` e `MaxSize` no cliente são cortesia.** A verdade é o `c.Files(FileRules)` no
   servidor, e o teste prova que um `.exe` renomeado passa pelo cliente e morre no servidor.
9. **Nada de biblioteca nem de asset novo.** O combobox entra no `ui.js`, a fila entra no
   `ui.upload.js`, e o `ui.Files` não ganha nome nenhum.

## Critérios de aceitação

- SC-001 `c.Files` devolve os N arquivos mandados com o mesmo nome, na ordem.
- SC-002 Regra que falha no arquivo 2 vira `arquivos[1]` no `FieldErrors`, e os outros passam.
- SC-003 `MaxFiles` recusa a lista inteira com uma mensagem no campo; `Optional` aceita zero.
- SC-004 `c.File` continua lendo o primeiro arquivo, com o mesmo comportamento de antes.
- SC-005 `ui.Combobox` rende `role="combobox"`, `aria-expanded`, o `<input type=hidden>` com o
  valor e o texto com o rótulo.
- SC-006 `ui.ComboboxOptions` rende `<li role="option" data-value=…>` e nada mais.
- SC-007 `ui.InvalidIf` e `ui.Errors` funcionam no combobox como em qualquer campo.
- SC-008 Sem valor escolhido, o `Bind` recebe o texto digitado e a rota pode devolver 422 com
  o rótulo preservado.
- SC-009 `ui.Dropzone` rende o `<input type=file multiple>` com `accept`, a área de soltar e a
  fila vazia; sem JavaScript o formulário envia igual.
- SC-010 `ui.js` ganha o combobox e `ui.upload.js` ganha a fila; `ui.Files` não muda.
- SC-011 `trilha ui` continua escrevendo os mesmos arquivos.
- SC-012 `examples/cadastro` troca o select dependente por um combobox com `With: uf` e perde
  as vinte linhas de `app.js`.
- SC-013 `examples/blog/anexos` ganha a dropzone e a rota passa a usar `c.Files`.
- SC-014 `make test` verde, `api/current.txt` regravado, docs nas duas línguas.
