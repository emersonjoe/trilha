# Implementation Plan: Trilha — framework web para Go estilo Next.js

**Branch**: `001-trilha-core` | **Date**: 2026-09-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-trilha-core/spec.md`

## Summary

Framework Go com roteamento por arquivos em `app/`, layouts e middlewares aninhados por
pasta, rotas de API por método exportado, DSL de HTML tipado com escape por padrão, CLI
(`new`, `gen`, `dev`, `build`, `routes`) que gera `trilha_gen.go` via `go/ast` e um
servidor de desenvolvimento com recarga por SSE. Runtime sobre `http.ServeMux` (Go 1.22+),
zero dependências externas.

## Technical Context

**Language/Version**: Go 1.25 (mínimo suportado 1.22 por causa dos patterns do ServeMux)
**Primary Dependencies**: biblioteca padrão apenas — `net/http`, `go/ast`, `go/parser`,
`go/format`, `os/exec`, `embed`, `log/slog`, `crypto/rand`, `html`
**Storage**: N/A (framework); exemplo usa memória
**Testing**: `go test` (unit + golden + `httptest` no exemplo); script de medição de recarga
**Target Platform**: Linux/macOS server; binário único
**Project Type**: biblioteca + CLI + app de exemplo (um único módulo Go)
**Performance Goals**: recarga < 2 s; renderização de página simples < 1 ms; > 20k req/s
em `/api` trivial numa máquina de dev (só para não regredir; não é meta de marketing)
**Constraints**: zero deps no núcleo; arquivo gerado determinístico; sem `reflect` para
descoberta; nomes de pasta válidos em import path
**Scale/Scope**: ~3.5k linhas de Go no núcleo; 5 comandos de CLI; 1 app de exemplo com
~10 rotas

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Princípio | Como o plano atende |
|-----------|---------------------|
| I. Convenção sobre configuração | Scanner em `internal/scan` define as convenções; `nome_`/`nome__` para dinâmicos; toda convenção tem rota no `examples/blog` e teste |
| II. Só stdlib em runtime | `go.mod` sem `require`; teste `TestNoExternalDeps` roda `go list -deps` |
| III. Geração explícita | `internal/gen` emite `trilha_gen.go` com imports e registros tipados; golden test garante determinismo; sem `reflect` |
| IV. Contrato de handler | `trilha.Ctx`, `h.Node`, `trilha.Next`; erros `ErrNotFound`/`RedirectError`; `recover` só em `serve.go` |
| V. Dev < 2 s, prod binário único | `internal/dev` (watch por polling de mtime + build + restart + SSE); `embed` de `public/` no arquivo gerado |
| VI. Teste primeiro | tarefas de teste precedem implementação em `tasks.md` para scan, gen, h e render; exemplo coberto por `httptest` |
| VII. Segurança por padrão | escape no `h`; cabeçalhos em `serve.go`; `http.MaxBytesReader`; CSRF em `csrf.go`; estáticos por `http.FS` com raiz fechada; `slog` sem corpo |

Sem violações. "Complexity Tracking" vazio.

## Project Structure

### Documentation (this feature)

```text
specs/001-trilha-core/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── conventions.md      # convenções de arquivo e assinaturas exigidas
│   ├── ctx-api.md          # API pública de trilha.Ctx / App / erros
│   ├── h-api.md            # API do DSL de HTML
│   └── cli.md              # comandos, flags e saídas da CLI
└── tasks.md
```

### Source Code (repository root)

```text
go.mod                      # module github.com/emersonjoe/trilha (sem require)
trilha.go                   # App, Config, Run, tipos de handler (package trilha)
ctx.go                      # Ctx e métodos
errors.go                   # ErrNotFound, RedirectError, HTTPError
render.go                   # pipeline página → layouts → html; páginas 404/500 padrão
csrf.go                     # token CSRF (cookie + campo/cabeçalho)
static.go                   # arquivos públicos (fs.FS) com cache e nosniff
serve.go                    # mux, registro de rotas, recover, logging, headers
devreload.go                # endpoint SSE e script de live-reload (só em dev)
h/                          # DSL de HTML
│   ├── node.go             # Node, Element, Text, Raw, Fragment, Attr, render
│   ├── elements.go         # Html, Head, Body, Div, ... (gerado por go generate)
│   ├── attrs.go            # Class, ID, Href, Src, ... 
│   └── control.go          # If, Map, Group
internal/
│   ├── scan/               # varre app/ → []Route (go/ast)
│   ├── gen/                # []Route → trilha_gen.go (text/template + go/format)
│   ├── dev/                # watcher, builder, supervisor de processo
│   └── scaffold/           # templates de `trilha new` (embed)
cmd/trilha/                 # CLI: main.go + comandos new/gen/dev/build/routes
examples/blog/              # app de exemplo (todas as convenções)
│   ├── app/...
│   ├── public/style.css
│   └── trilha_gen.go       # commitado
testdata/                   # árvores app/ sintéticas + golden files do gerador
```

**Structure Decision**: um único módulo Go. Pacote raiz `trilha` é a API pública que apps
importam; `h` é pacote público separado (nome curto para o DSL); tudo o que só a CLI usa
fica em `internal/`. O exemplo vive no mesmo módulo para ser testado por `go test ./...`
(um `replace` seria necessário se fosse módulo à parte).

## Phase 0 — Research

Ver [research.md](research.md): decisões sobre nome de pasta para segmentos dinâmicos,
descoberta por `go/ast` vs `reflect`, watcher por polling vs `fsnotify`, SSE vs WebSocket,
DSL vs `html/template`, e embed de `public/` via arquivo gerado.

## Phase 1 — Design

- [data-model.md](data-model.md): `Route`, `Layout`, `Middleware`, `Ctx`, `Node`, `App` e
  regras de validação do scanner.
- [contracts/](contracts/): convenções de arquivo, API do `Ctx`, API do `h`, CLI.
- [quickstart.md](quickstart.md): do `trilha new` ao binário.

## Phase 2 — Tasks

Gerado por `/speckit-tasks` em `tasks.md`.

## Complexity Tracking

Sem violações da constituição.
