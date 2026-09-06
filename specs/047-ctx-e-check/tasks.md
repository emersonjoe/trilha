# Tasks: `trilha ctx` e `trilha check`

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — erros com conserto (SC-006)

- [x] T001 Teste que falha em `internal/scan/scan_test.go`: `page.go` com `func Index` reporta
      `Line` da função encontrada e `Fix` não vazio dizendo a assinatura; `route.go` só com
      `func get` reporta a linha do `get` e o conserto sobre maiúsculas; `app/a/slug_/b/slug_`
      reporta `ErrDuplicateParam` com as duas pastas; todo código de erro tem `Fix`.
- [x] T002 `internal/scan`: campos `Line` e `Fix` em `Error`, `errfAt`, tabela `fixes` por
      código, funções não exportadas com posição no `pkgInfo`, checagem de parâmetro repetido.

## Bloco 2 — o modelo (SC-001, SC-002, SC-003, SC-004)

- [x] T003 Teste que falha em `internal/ctx/ctx_test.go`: `Build` sobre a árvore sintética dá
      rotas em ordem de precedência, contrato de API, tipos com regras de `validate` e o
      `Setup`; duas chamadas de `JSON` dão bytes idênticos; `Markdown` sem `--all` elide a
      cadeia por método e com `--all` não.
- [x] T004 `internal/ctx/ctx.go`: `Context`, `Build`, `JSON`, `Markdown(Sections)`; leitura do
      documento do `openapi`; deduzir `trilha.Provide` do `setup.go`.
- [x] T005 Golden `testdata/golden/ctx.json.golden` e `ctx.md.golden` do `examples/blog`, com
      `make golden` regravando; teste de tempo com 40 rotas sintéticas (< 1 s).

## Bloco 3 — os comandos (SC-005, SC-007, SC-008, SC-009)

- [x] T006 Teste que falha em `cmd/trilha/check_test.go`: ordem dos passos, parada no primeiro
      erro com os seguintes como não executados, `--fix` regerando e formatando, `--json`
      batendo com `testdata/golden/check.json.golden`, código de saída.
- [x] T007 `cmd/trilha/ctx.go` e `cmd/trilha/check.go`: as flags, a execução dos passos, a
      tradução para `{tool,file,line,message,fix}` e a impressão compartilhada dos erros do
      scanner com o conserto (usada também por `gen` e `dev`).
- [x] T008 e2e em `cmd/trilha/e2e_test.go`: projeto novo passa no `check`; projeto com rota
      adicionada sem `gen` reprova e passa com `--fix`.

## Bloco 4 — fechamento (SC-010)

- [ ] T009 `internal/scaffold/agents/*.md`: `trilha check` como o portão único, `trilha ctx`
      na tabela de comandos, nos dois idiomas; `usage` da CLI e `TestAgentsMatchesUsage`.
- [ ] T010 Documentação nos dois idiomas (referência da CLI e a página de agentes) e
      `site/internal/docs` em sincronia.
- [ ] T011 CHANGELOG (0.37.0), ROADMAP (itens 25 e 26), régua (#45) antes e depois.
