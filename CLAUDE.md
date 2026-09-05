# Trilha — framework web para Go estilo Next.js

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan
<!-- SPECKIT END -->

## Comandos

- `make test` — gofmt (fora de testdata) + `go vet ./...` + `go test ./...` (inclui e2e da CLI).
- `make golden` — regrava os golden files do gerador após mudar `internal/gen`.
- `make dev-example` / `make reload` — dev server no exemplo e medição do ciclo de recarga.
- `cd examples/blog && go run ../../cmd/trilha gen` — regenerar `trilha_gen.go` do exemplo (commitado).

## Estrutura

- Raiz (`package trilha`): runtime — `App`, `Ctx`, erros, render, serve, csrf, static.
- `h/`: DSL de HTML (elementos gerados em `elements.go`).
- `internal/scan` (app/ → rotas), `internal/gen` (rotas → trilha_gen.go), `internal/dev` (watch + build + proxy + SSE), `internal/scaffold` (templates do `new`).
- `cmd/trilha`: CLI. `examples/blog`: app de referência com todas as convenções. `testdata/`: árvores sintéticas e goldens.

## Regras (constituição em `.specify/memory/constitution.md`)

- Zero dependências externas no runtime e na CLI (`TestNoExternalDeps`).
- Toda convenção nova precisa de: teste no scanner, rota no `examples/blog` e teste de integração.
- Gerador determinístico; arquivo gerado é commitado.
- Código/identificadores em inglês; docs, specs e mensagens da CLI em pt-BR.
- Commits sem trailer de coautoria.
