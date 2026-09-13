# Tarefas — Spec 149

- [x] T001 `send_test.go`: corpo que não busca posição com `Size` — `Range: bytes=10-19` dá 206,
      `Content-Range: bytes 10-19/1000`, `Content-Length: 10` e os dez bytes certos; `bytes=900-`
      e `bytes=-10` cortam nas pontas; sem `Range` dá 200 com `Accept-Ranges: bytes` e
      `Content-Length`; `bytes=2000-3000` e `Range` ilegível dão 416 com `bytes */1000`; dois
      intervalos dão o arquivo inteiro; `HEAD` com `Range` traz os cabeçalhos e nenhum byte
- [x] T002 `send_test.go`: `ContentRange` de um corpo já parcial dá 206 sem cortar nada, com
      `Content-Length` do intervalo; valor torto e `Size` junto com `ContentRange` são erro
- [x] T003 `send_test.go`: SVG com `NeutralizeScript` dá 200 com a CSP, `Content-Disposition:
      inline` e `nosniff`; sem a opção, `errors.Is(err, trilha.ErrCannotInline)` e nada escrito;
      `application/zip` com a opção continua recusado
- [x] T004 `send.go`: `SendOpts`, `Ctx.Send`, `ErrCannotInline`, `Inline`/`Attachment`/`sendFile`
      sobre o mesmo caminho, o corte de `Range`, a passagem do `Content-Range` e a CSP
- [x] T005 `api/current.txt` (`go test -run TestSuperficiePublica -update .`)
- [x] T006 Referência do `Ctx` em `en/reference/ctx.md` e `pt/referencia/ctx.md`: `Send` e
      `SendOpts` na tabela de arquivos, uma seção para o `Range` de corpo que não busca posição
      e outra para o SVG neutralizado, e `ErrCannotInline` onde a recusa é explicada
- [x] T007 Uso em `examples/`: `internal/midia`, `app/midia/{page.go,audio,marca}`,
      `trilha_gen.go` regerado e `examples/blog/midia_test.go` de integração
- [x] T008 `CHANGELOG.md` (Added/Fixed), `version` = 0.128.0, `ROADMAP.md`
- [ ] T009 `make test` verde, `verifica-trilha.sh --sem-testes` e a PR
