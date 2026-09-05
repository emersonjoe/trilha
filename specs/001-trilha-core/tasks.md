# Tasks: Trilha — framework web para Go estilo Next.js

**Input**: Design documents from `/specs/001-trilha-core/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Exigidos pela constituição (VI): testes do núcleo antes da implementação.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [x] T001 Criar `go.mod` (`module github.com/emersonjoe/trilha`, `go 1.22`), `.gitignore`, `README.md` mínimo, `Makefile` (`test`, `vet`, `example`)
- [x] T002 [P] Criar esqueleto de pastas: `h/`, `internal/{scan,gen,dev,scaffold}`, `cmd/trilha/`, `examples/blog/`, `testdata/`

## Phase 2: Foundational — DSL `h` e contrato de runtime

- [x] T003 [P] Testes de `h`: escape de texto/atributo, void elements, nil filho, Fragment/If/Map, Raw, ordem atributos→filhos em `h/node_test.go`
- [x] T004 Implementar `h/node.go` (Node, element, attr, text, raw, fragment, doctype, Render)
- [x] T005 [P] Implementar `h/control.go` (If, IfElse, Map, Group) e `h/attrs.go`
- [x] T006 [P] Gerar `h/elements.go` via `h/gen_elements.go` (`go:generate`) com lista de tags HTML5 + void
- [x] T007 [P] Implementar `errors.go` (ErrNotFound, RedirectError, HTTPError, Redirect, Errorf) com testes em `errors_test.go`
- [x] T008 Implementar `trilha.go` (Config, ConfigFromEnv, App, New, Register, Route, tipos de função, Values, Fatal)
- [x] T009 Implementar `ctx.go` (Param, Query, Form, BindJSON, JSON, Text, HTML, Redirect, cookies, Set/Get, Title, RequestID, Env) com `ctx_test.go`

**Checkpoint**: `go test ./h/ .` verde.

## Phase 3: User Story 1 — Páginas por arquivo (P1) 🎯 MVP

- [x] T010 [P] [US1] Testes do pipeline de renderização (página → layouts de dentro para fora, título, 404/500 padrão, stack só em dev) em `render_test.go`
- [x] T011 [US1] Implementar `render.go` (renderPage, layouts, defaultNotFound, defaultError, injeção do script dev)
- [x] T012 [US1] Implementar `serve.go` (mux, registro `GET /{$}` e patterns, middlewares encadeados, recover, cabeçalhos de segurança, slog, request id, MaxBytesReader) com `serve_test.go`
- [x] T013 [P] [US1] Árvores sintéticas em `testdata/apps/{basic,dynamic,catchall,errors/*}` e testes de tabela do scanner (`internal/scan/scan_test.go`): patterns, layouts/middlewares em ordem, cada erro E_*
- [x] T014 [US1] Implementar `internal/scan` (walk de `app/`, parse com `go/parser` só declarações, validação, ordenação determinística)
- [x] T015 [P] [US1] Golden test do gerador em `internal/gen/gen_test.go` (`testdata/golden/*.go.golden`, flag `-update`)
- [x] T016 [US1] Implementar `internal/gen` (template + `go/format`, aliases seguros, embed condicional de `public/`)
- [x] T017 [US1] CLI `cmd/trilha`: `main.go` (dispatch), `gen.go`, `routes.go` (lê `go.mod` para o import path base)
- [x] T018 [US1] Exemplo `examples/blog`: `app/layout.go`, `app/page.go`, `app/blog/layout.go`, `app/blog/page.go`, `app/blog/slug_/page.go`, `app/docs/path__/page.go`, `app/not_found.go`, `app/error.go`, `public/style.css`; rodar `trilha gen` e commitar `trilha_gen.go`
- [x] T019 [US1] Teste de integração `examples/blog/blog_test.go` com `httptest` cobrindo cenários 1–6 da US1

**Checkpoint**: `go run ./examples/blog` responde `/`, `/blog/ola`, `/docs/a/b`, 404, 500.

## Phase 4: User Story 2 — API e formulários (P1)

- [x] T020 [P] [US2] Testes de CSRF (`csrf_test.go`): token criado, válido, inválido, ausente; API sem CSRF por padrão
- [x] T021 [US2] Implementar `csrf.go` (+ `h.CSRF(c)` helper em `trilha`/`h` sem ciclo de import: `trilha.CSRFField(c) h.Node`)
- [x] T022 [US2] Registrar métodos de `route.go`/`page.go` em `serve.go` (405 + Allow vem do mux; 413 via MaxBytesReader; 404 JSON para api)
- [x] T023 [US2] Exemplo: `app/api/posts/route.go` (GET/POST/DELETE em memória via `Setup`), `app/blog/novo/page.go` com `Page`+`POST`, `app/setup.go`
- [x] T024 [US2] Integração: cenários 1–6 da US2 em `examples/blog/blog_test.go`

## Phase 5: User Story 3 — Middleware (P2)

- [x] T025 [P] [US3] Testes de encadeamento de middleware (ordem, curto-circuito, erro, Set/Get) em `serve_test.go`
- [x] T026 [US3] Exemplo: `app/middleware.go` (log + request id) e `app/admin/middleware.go` + `app/admin/page.go` + `app/login/page.go` (POST seta cookie)
- [x] T027 [US3] Integração: cenários 1–3 da US3

## Phase 6: User Story 4 — `trilha dev` (P2)

- [x] T028 [P] [US4] Testes de `internal/dev/watch` (snapshot de mtime, detecção de mudança, ignorar `trilha_gen.go` e `bin/`) em `internal/dev/watch_test.go`
- [x] T029 [US4] Implementar `internal/dev/watch.go` (polling 250 ms + debounce)
- [x] T030 [US4] Implementar `internal/dev/build.go` (gen + `go build -o .trilha/app`) e `internal/dev/supervisor.go` (start/stop filho com env `TRILHA_ENV=dev`, `ADDR`; servidor de erro de compilação na mesma porta enquanto falha)
- [x] T031 [US4] Implementar `devreload.go` no runtime (`/_trilha/events` SSE + script injetado antes de `</body>` em dev)
- [x] T032 [US4] `cmd/trilha/dev.go` e `static.go` (`os.DirFS("public")` em dev, embed em prod; `Cache-Control`; path traversal bloqueado) com `static_test.go`
- [x] T033 [US4] Script `scripts/measure-reload.sh` que edita `page.go` e mede tempo até o novo texto responder (SC-002)

## Phase 7: User Story 5 — `build`, `new`, `routes` (P3)

- [x] T034 [P] [US5] Templates de scaffold em `internal/scaffold/templates/` (go.mod, app/layout.go, app/page.go, app/not_found.go, app/api/hello/route.go, public/style.css, .gitignore) embutidos
- [x] T035 [US5] `cmd/trilha/new.go` (cria pasta, escreve templates, `go mod edit -require` para o trilha, roda `gen`) com teste que executa `new` em tmp e compila
- [x] T036 [US5] `cmd/trilha/build.go` (`CGO_ENABLED=0 go build -trimpath -o bin/<nome>`)
- [x] T037 [US5] Teste end-to-end `cmd/trilha/e2e_test.go`: `new` → `build` → sobe binário em porta livre → `GET /` e `/style.css` 200 (pula se `go` não estiver no PATH)

## Phase 8: Polish

- [x] T038 [P] `README.md` completo (PT-BR + seção EN): conceito, convenções, Ctx, h, CLI, comparação com Next.js
- [x] T039 [P] Teste `TestNoExternalDeps` (`go list -deps` do runtime e da CLI) — SC-004
- [x] T040 `go vet ./...`, `gofmt -l .` vazio, `go test ./...` verde na raiz; atualizar `CLAUDE.md` com comandos
- [x] T041 Rodar `quickstart.md` do zero em diretório temporário e ajustar o que divergir

## Dependencies

- Phase 2 bloqueia tudo. US1 (Phase 3) é pré-requisito prático de US2–US5 porque o
  exemplo e o gerador nascem nela. US2 e US3 são independentes entre si após US1. US4 e
  US5 dependem só de US1 (CLI `gen`).
- Testes marcados [P] podem ser escritos em paralelo com o código anterior, mas devem
  falhar antes da implementação correspondente.

## Implementation Strategy

MVP = Phases 1–3 (páginas por arquivo com layouts, 404/500, gerador e `trilha gen`).
Depois US2 (API+form), US3 (middleware), US4 (dev), US5 (build/new), polish.
