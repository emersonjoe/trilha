# Tarefas — Spec 042

## Fase 1 — pacote do arquivo gerado (#51)

- [ ] T101 `internal/scan`: `rootPackage(root)` (irmão do `rootHasMain`, mesma exclusão de
      `_test.go`/`trilha_gen.go`), com fallback para o pacote de um `trilha_gen.go` existente;
      campo `Result.Package`.
- [ ] T102 `internal/gen`: `package {{.Package}}`, construtor `newApp`/`NewApp` conforme o
      pacote, `func main()` só em `package main`; comentário do construtor citando
      `a.Handler()` no caso embutido.
- [ ] T103 `testdata/apps/embedded` (um `crm.go` com `package crm`, duas rotas) e o golden
      `testdata/golden/embedded.go.golden`; `TestGolden` passa a incluí-lo.
- [ ] T104 `cmd/trilha`: `gen --package <nome>`, aplicado sobre o `Result` antes do
      `gen.Generate`; `--check` sem bandeira continua valendo (o arquivo lembra).
- [ ] T105 `cmd/trilha`: `dev` e `build` recusam pacote ≠ `main`; mensagens em `i18n.go` (en+pt).
- [ ] T106 `make test`.

## Fase 2 — middleware por método (#56)

- [ ] T201 `internal/scan`: reconhecer `MiddlewareGET|POST|PUT|PATCH|DELETE` em `middleware.go`,
      herdar pela subárvore, `Route.MiddlewaresByMethod`; `middleware.go` válido com um ou outro.
- [ ] T202 `internal/scan`: `E_UNUSED_METHOD_MIDDLEWARE` quando nenhuma rota da subárvore serve
      o método; teste com `testdata/apps/err_unused_method_mw`.
- [ ] T203 `internal/gen` + golden: emitir `MiddlewaresByMethod` (mapa ordenado).
- [ ] T204 `trilha.go`: campo `MiddlewaresByMethod` na `Route`, documentado com a ordem.
- [ ] T205 `serve.go`: `Register` compõe a cadeia do método em cada `wrap` (inclusive o `Page`).
- [ ] T206 `serve_test.go`: ordem `Middleware` → `MiddlewareX` → CSRF → handler, e o GET que não
      vê o middleware do POST.
- [ ] T207 `examples/blog`: uma rota mista usando a convenção (a que o exemplo já tem com POST).
- [ ] T208 `make test`.

## Fase 3 — `error.go` para todo status (#53)

- [ ] T301 `errors.go`: `StatusOf` exportada, `statusOf` delegando; doc dizendo para que serve
      no `error.go`.
- [ ] T302 `render.go`: `renderErrorPage(c, cause, code)`, `simplePage` de fallback por faixa de
      status, `handleError` com dois ramos (404 e o resto).
- [ ] T303 `render_test.go`/`serve_test.go`: 403 com layout e `error.go` do app; 403 sem
      `error.go` mantendo a página interna; `error.go` que falha caindo na rede; API em
      `problem+json` intocada.
- [ ] T304 `examples/blog/app/error.go`: `switch trilha.StatusOf(err)` como o exemplo canônico.
- [ ] T305 `make test`.

## Fase 4 — documentação e release

- [ ] T401 `learn/middleware` + `pt/aprender/middleware`: seção do middleware por método.
- [ ] T402 `reference/conventions` + pt: `MiddlewareX`, pacote do `trilha_gen.go`, `error.go`
      para todo status.
- [ ] T403 `reference/errors` + pt: `StatusOf` e a tabela de quem renderiza cada status.
- [ ] T404 `reference/app` + pt: `Route.MiddlewaresByMethod`; `reference/cli` + pt:
      `gen --package`.
- [ ] T405 `cookbook/migration` + `pt/receitas/migracao`: a seção do app embutido de ponta a
      ponta (`trilha gen` no diretório, `NewApp`, `mux.Handle("/", a.Handler())`).
- [ ] T406 `learn/troubleshooting` + pt: os três sintomas (403 sem layout, `MiddlewarePOST` que
      não guarda nada, `package main` onde devia ser pacote comum).
- [ ] T407 CHANGELOG (0.32.0), ROADMAP, `cmd/trilha/main.go` version.
- [ ] T408 `make test` e `make release VERSION=0.32.0 ISSUES="51 53 56"`.
