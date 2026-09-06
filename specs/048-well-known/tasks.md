# Tasks: `/.well-known/`

**Input**: [spec.md](./spec.md), [plan.md](./plan.md) · uma rodada de `make test` por bloco.

## Bloco 1 — scanner (SC-001, SC-002, SC-003)

- [ ] T001 Fixtures `testdata/apps/wellknown` (rota em `app/.well-known/security.txt/`, mais
      `app/.git/`, `app/_park/page.go` e `app/testdata/page.go` que devem continuar mudos) e
      `testdata/apps/err_hidden_route` (`app/.oauth/route.go`).
- [ ] T002 Teste que falha em `internal/scan/scan_test.go`: o fixture `wellknown` dá `/` e
      `/.well-known/security.txt` sem erro; `err_hidden_route` dá `E_HIDDEN_ROUTE` com o
      caminho do arquivo e conserto não vazio.
- [ ] T003 `internal/scan`: `const WellKnown`, exceção no pulo do `walk`, `ErrHiddenRoute` +
      entrada em `fixes`, varredura rasa da pasta pulada.

## Bloco 2 — openapi e dev (SC-005, SC-006)

- [ ] T004 Teste que falha em `internal/dev/watch_test.go` (arquivo sob `app/.well-known/`
      entra no snapshot) e em `internal/openapi` (tipo declarado no pacote `.well-known`
      aparece no documento).
- [ ] T005 `internal/openapi/schema.go` e `internal/dev/watch.go` lendo `scan.WellKnown`.

## Bloco 3 — uso real e integração (SC-001, SC-004, SC-006)

- [ ] T006 `examples/blog/app/.well-known/security.txt/route.go` (RFC 9116) + `trilha gen` do
      exemplo; teste em `examples/blog/blog_test.go` batendo status, `Content-Type` e corpo.
- [ ] T007 Rota tipada em `testdata/apps/openapi/app/.well-known/oauth-authorization-server/`
      e `make golden`.
- [ ] T008 e2e em `cmd/trilha/e2e_test.go`: `trilha gen` no app do scaffold enxerga a rota
      `.well-known` e `trilha check` reclama do `route.go` escondido em pasta com ponto.

## Bloco 4 — fechamento (SC-007)

- [ ] T009 Documentação nas duas locales: tabela de pastas em `reference/conventions.md` e
      `referencia/convencoes.md`, com a exceção e o novo erro; menção onde a #14 é explicada.
- [ ] T010 `CHANGELOG.md` 0.38.0, `version` em `cmd/trilha/main.go`, item do `ROADMAP.md`.
- [ ] T011 `make test` verde; release com aprovação do usuário.
