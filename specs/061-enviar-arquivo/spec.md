# Spec 061 — Mandar um arquivo de volta

Issue: [#71](https://github.com/emersonjoe/trilha/issues/71) (`Ctx.Attachment`, `Ctx.Inline`,
`Ctx.Pipe`). A issue é a fonte do escopo; aqui fica só a decisão.

## Decisões

1. **Uma função por intenção, não uma com flag.** `Attachment` baixa, `Inline` abre no visor
   do navegador. São dois `Content-Disposition` diferentes e duas listas de tipo diferentes:
   um parâmetro booleano esconderia justamente a decisão de segurança.
2. **O nome passa pelo mesmo `safeName` do upload.** Quem manda o nome para baixar é o app,
   mas o app quase sempre está repetindo o que um dia veio de fora. Um caminho não vira nome
   de arquivo, e o cabeçalho não ganha quebra de linha.
3. **`filename*` em UTF-8 mais `filename` ASCII, nessa ordem.** RFC 6266/5987. O fallback
   ASCII é o nome com o que não é ASCII trocado por `_`, entre aspas, com `"` e `\` escapados
   — nunca o nome cru, que é o jeito de injetar um parâmetro no cabeçalho.
4. **O tipo vem do conteúdo quando o chamador não disse.** `http.DetectContentType` nos
   primeiros 512 bytes, como o `c.File` já faz para o upload. Sem `ReadSeeker` para voltar, o
   sniff usa um buffer e a resposta sai desse buffer seguida do resto.
5. **`Inline` recusa o que executa.** Só `application/pdf`, `image/*`, `audio/*`, `video/*`,
   `text/plain` e `text/csv`. HTML, SVG, XML e qualquer `text/*` fora da lista voltam como
   erro de programação (500), não como resposta com `nosniff` e torcida: o SVG é o caso claro
   — imagem para o olho, documento com script para o navegador.
6. **A CSP não é afrouxada por baixo.** `frame-ancestors 'none'` continua o padrão, e o
   `<iframe>` do próprio host precisa de `frame-src 'self'` no `CSPExtra` do app. O `Inline`
   documenta isso; ele não reescreve política de resposta que não é dele.
7. **`ReadSeeker` ganha `Range` de graça.** Com `io.ReadSeeker` a escrita é
   `http.ServeContent`: `Range`, `If-Range`, `304` e `HEAD` já são dele. Sem `Seeker` é
   `io.Copy` e nenhum `Accept-Ranges` prometido.
8. **`AttachmentFile` e `InlineFile` abrem e fecham o arquivo.** O `ETag` sai de `mtime` e
   tamanho, que é o que o `ServeContent` usa como `modtime`; abrir um diretório é erro.
9. **Todo envio desliga o write deadline.** Um arquivo de 50 MB numa linha ruim não é um
   handler lento: é um handler que está fazendo o que foi mandado. `NoWriteDeadline` já
   existe e ignorar quem não suporta já é o comportamento dele.
10. **`Pipe` copia uma lista fechada de cabeçalhos.** `Content-Type`, `Content-Disposition`,
    `Content-Length`, `Content-Encoding`, `Content-Range`, `Accept-Ranges`, `Cache-Control`,
    `ETag`, `Last-Modified`, `Expires`, `Vary` e `Age`. `Set-Cookie` e qualquer coisa de
    autenticação ficam do outro lado: o corpo de outro serviço não senta na sessão deste.
    `Pipe` fecha o corpo que recebeu.

## Critérios de aceitação

- SC-001 `Attachment` põe `filename*=UTF-8''` com o nome percent-encoded e um `filename` ASCII.
- SC-002 Nome com aspas, barra, `..` e quebra de linha sai saneado, num cabeçalho de uma linha.
- SC-003 Sem `Content-Type` do chamador, o tipo é o detectado no conteúdo, e o corpo sai inteiro.
- SC-004 `Range: bytes=0-99` num `ReadSeeker` responde 206 com `Content-Range`; `HEAD` não tem corpo.
- SC-005 Sem `Seeker`, a resposta é 200 completa e não anuncia `Accept-Ranges: bytes`.
- SC-006 `Inline` de `text/html`, `image/svg+xml` ou `application/xml` devolve erro, não 200.
- SC-007 `Inline` de PDF põe `Content-Disposition: inline` e mantém `X-Content-Type-Options`.
- SC-008 `AttachmentFile` de arquivo inexistente é 404; de diretório, erro; fecha o arquivo.
- SC-009 `Pipe` copia status e a lista fechada de cabeçalhos, e não copia `Set-Cookie`.
- SC-010 `Pipe` de um `httptest.Server` preserva `Content-Disposition` e o corpo em stream.
- SC-011 Um corpo de 50 MB sai sem esbarrar no write deadline.
- SC-012 `examples/orcamento` manda o CSV por `c.Attachment` e `examples/blog/anexos` baixa e abre.
- SC-013 Referência nas duas línguas, entrada no `SECURITY-MODEL.md`, `api/current.txt`.
