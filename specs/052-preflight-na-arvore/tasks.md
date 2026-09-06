# Tarefas — spec 052

- [x] **T001** `scan.Methods` com `OPTIONS`; conferir `Methods[1:]` do `page.go` e as
      mensagens de `middleware.go` e `route.go`.
- [x] **T002** `E_UNROUTABLE_METHOD` para `HEAD`, `TRACE` e `CONNECT` exportados, com linha e
      conserto.
- [x] **T003** `Route.HasCORS` no varredor; `E_CORS_ON_PAGE` quando o `page.go` declara.
- [x] **T004** Testes do varredor (T001–T003).
- [x] **T005** `internal/gen`: `CORS: &pkg.CORS`; golden novo.
- [x] **T006** Runtime: `Route.CORS`, política por rota no `Register`, OPTIONS gerado quando
      o arquivo não escreve o seu.
- [x] **T007** Testes de runtime: preflight 204, origem recusada 403, requisição simples,
      OPTIONS à mão vence, rota vizinha intacta.
- [x] **T008** `internal/openapi` pula OPTIONS.
- [x] **T009** `audit`: segredo ausente vira aviso quando nada assina; i18n nas duas línguas.
- [x] **T010** Teste do `audit` nos dois lados.
- [x] **T011** `examples/blog`: `var CORS` + `func OPTIONS` no `/.well-known/security.txt`,
      `trilha_gen.go` regenerado.
- [x] **T012** Teste de integração no `examples/blog` (preflight de fora chega ao documento).
- [x] **T013** Docs nas duas línguas (convenções + receita) e `CHANGELOG` na 0.39.0.
- [x] **T014** `make test` verde e revisão do diff.
