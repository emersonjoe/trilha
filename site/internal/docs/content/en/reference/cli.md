---
title: CLI
description: The trilha commands and their options.
---

```text
trilha new <dir> [--module path] [--lang en|pt] [--agents] [--trilha-dir ../trilha] [--no-tidy]
trilha gen [--check] [--package name]
trilha generate page|route <url> | component <Name> [--force] [--dir path]
trilha dev [--addr :3000]
trilha build [-o bin/<name>]
trilha export [-o out] [--base /prefix]
trilha openapi [-o file] [--title T] [--version V] [--server URL] [--check]
trilha routes
trilha check [--json] [--fix]
trilha ctx [--json] [--routes|--types|--all]
trilha audit [--no-vuln]
trilha ui [--force] [--css-only|--js-only]
trilha agents [--force] [--lang en|pt]
trilha version
```

| Command | What it does |
|---|---|
| `new` | creates a project with `go.mod`, layout, home page, 404, one API route, `public/style.css` and `.gitignore`; runs `go mod tidy` and `gen` |
| `gen` | scans `app/` and writes `trilha_gen.go`; fails with one line per violated convention |
| `generate` | writes one skeleton — a page, an API route or a component — in the folder the convention asks for |
| `dev` | `gen` + `go build` + runs the app on an internal port + proxy on `--addr` + reload over SSE + route inspector on `/_trilha/routes` |
| `build` | `gen` + `go build -trimpath -ldflags="-s -w"` with `CGO_ENABLED=0` |
| `export` | `gen` + `go build` + runs with `TRILHA_EXPORT` to produce static HTML |
| `openapi` | writes the OpenAPI 3.1 document of the API routes (`-o -` to stdout) |
| `routes` | prints `METHODS PATTERN SOURCE` for each route |
| `check` | the single gate: `gen`, `gofmt`, `vet`, `test`, `audit` and `openapi`, in that order, stopping at the first failure |
| `ctx` | the map of the project — routes, API, types, setup — in one read, as Markdown or JSON |
| `audit` | security checklist before publishing (see [Security](/reference/security)) |
| `agents` | writes `AGENTS.md` and `CLAUDE.md` so a coding agent finds the conventions |

Commands run in the folder containing `app/`. The project's import path comes from the
nearest `go.mod`, plus the subfolder, so an app can live inside a larger module.

## Language

CLI messages follow `TRILHA_LANG`, then `LC_ALL`, `LC_MESSAGES` and `LANG`: a value starting
with `pt` (any case) selects Portuguese; anything else, including an unset variable, selects
English. Messages from the runtime, the scanner and the generator (the ones that end up in
your code and logs) are always in English.

`trilha new --lang en|pt` chooses the language of the generated texts (home page, 404,
`<html lang>`); the default is the CLI's language.

## trilha dev

Besides the proxy and the reload, the supervisor serves the route inspector on
`/_trilha/routes`: the table of routes in precedence order with layouts and middlewares per
route, and a box that answers which pattern would serve a given path. The page belongs to the
supervisor, not to the app, so it does not exist in the binary `trilha build` produces — see
[Development and production](/learn/dev-and-production#the-route-inspector).

## trilha generate

The convention is what costs to remember: that `/blog/{slug}` lives in `app/blog/slug_/`,
that a catch-all folder ends in `__`, that a group ends in `-`. `generate` takes the URL and
does the translation:

```bash
trilha generate page /blog/{slug}     # app/blog/slug_/page.go
trilha generate route /api/itens/{id} # app/api/itens/id_/route.go
trilha generate component Aviso       # internal/components/aviso.go
```

The page and the route come out compiling, with `c.Param` already reading each parameter, and
`trilha_gen.go` is regenerated at the end — the URL answers before you open the editor. A
component is a function returning `h.Node`, so it composes like any other; `--dir` puts it
somewhere else (`internal/icons`, for instance).

The package name is the one already declared in the folder, when there is one; otherwise it
comes from the folder name (`slug_` → `slug`, `relatorio.csv` → `relatoriocsv`, `type` →
`type_`).

An existing file is not overwritten without `--force`, and `--force` does not cover the one
refusal that is a convention: a folder answers either a page or a route, never both.

## trilha ui

Writes or updates the UI kit in `public/`: `ui.theme.css` (only created; it is your theme),
`ui.css` and `ui.js` (updated; if edited locally, only with `--force`). `--css-only` and
`--js-only` limit what is touched. `trilha new` runs the same step. See
[UI kit](/learn/ui-kit).

## trilha agents

Writes two files at the root of the project, and only when asked: support for coding agents is
opt-in, so `trilha new` on its own leaves neither behind. `trilha new --agents` adds them at
creation time.

| File | Who owns it |
|---|---|
| `AGENTS.md` | the framework: the conventions, the commands, and what not to do |
| `CLAUDE.md` | you: three lines pointing at `AGENTS.md`, plus whatever this repository needs |

`AGENTS.md` carries a stamp with the hash of its own body, the same rule the ui kit uses. An
untouched copy from an older version is refreshed in silence on the next run; one you edited is
only overwritten with `--force`, and without it the command stops and says so. `CLAUDE.md` is
never overwritten.

`--lang en|pt` picks the language of both files and defaults to the CLI's.

## trilha openapi

Reads `app/`, deduces the document from the handlers and writes `openapi.json`. `-o -` writes
to stdout; `--title`, `--version` and `--server` fill what the code cannot know (they default
to the module name, `0.0.0` and no server). `--check` compares with the file on disk and exits
`1` when they differ — the same line `gen --check` is, for the same reason:

```yaml
- run: trilha openapi --check
```

What is deduced and the `openapi:` directives are in [APIs](/learn/api#the-openapi-document).

## trilha check

Six gates in one command, in the order that fails cheapest first: `gen`, `gofmt`, `vet`,
`test`, `audit` (without the vulnerability scan, which needs the network) and `openapi` (only
if the project keeps the document). It stops at the first failure — what comes after a broken
build says nothing about the project — and the steps that never ran say so:

```text
✓ gen
✗ gofmt (failed)
    app/blog/page.go: not gofmt'd
    → run gofmt -w (or trilha check --fix)
- vet (not run)
- test (not run)
- audit (not run)
- openapi (not run)
```

Every problem carries the file, the line and the sentence that resolves it. `--fix` rewrites
`trilha_gen.go` and the formatting before judging them, and the step then reports `fixed`.
`--json` writes the report a tool reads, with the same fields:

```json
{
  "ok": false,
  "steps": [{ "tool": "gen", "status": "failed" }],
  "problems": [
    {
      "tool": "gen",
      "file": "app/page.go",
      "line": 3,
      "message": "page.go must export func Page(c *trilha.Ctx) (h.Node, error); found func Render",
      "fix": "rename the function to Page, or delete page.go if this directory is not a page"
    }
  ]
}
```

Exit `1` when anything failed, so in CI it is the single line:

```yaml
- run: trilha check
```

## trilha ctx

The map of the project in one read: the module, whether `trilha_gen.go` is up to date, every
route with its file, methods, parameters, layouts and middlewares, each API operation with its
query, body and responses, the types those operations exchange, and what `app/setup.go`
provides:

```text
# example.com/store

- trilha 0.37.0 · 8 routes (6 pages, 2 APIs)
- trilha_gen.go: up to date
- app/setup.go: Setup, Config

## Routes

- `GET /` — app/page.go · layouts: app/layout.go
...
```

The default is compact Markdown for reading. `--routes` and `--types` print one section alone,
`--all` elides nothing (the per-method middlewares, every error response, the `Problem` type),
and `--json` writes the same model as a document, sorted and free of clocks and absolute paths,
so two runs of the same tree produce the same bytes.

The API section and the types come from the same inference behind `trilha openapi`, so the map
and the document can never disagree. Like `openapi.json`, the output itself is a machine
document and is not translated.

## trilha gen --check

Generates in memory, compares with the committed `trilha_gen.go` and exits `1` with the
differing lines when they diverge — one line in the CI, and a folder added to `app/` without
running `trilha gen` stops being a 404 nobody can explain:

```yaml
- run: trilha gen --check
```

`trilha check` runs this same comparison as its first gate, which is why a project that uses
it needs no separate `gen --check` line. `trilha audit` runs the comparison as a warning, and
also compares the CLI's version
with the library's in `go.mod`: a newer CLI writes code the library may not have yet, and
the error then shows up inside generated code — the worst place to look for it.

## Generated file

`trilha_gen.go` is deterministic (same tree, same bytes), carries the header
`// Code generated by trilha. DO NOT EDIT.` plus a `//go:generate trilha gen` directive (so
`go generate ./...` works without knowing the tool's name) and must be committed: `go build ./...` works
without the CLI installed. It defines `newApp() *trilha.App` and `main()`; if another file
in the package already has `func main()`, the generator omits its own (see
[App](/reference/app)).

### An app inside a binary that already exists

The generated file takes the package the folder declares, so a Trilha app can be a normal,
importable package inside a `net/http` server you already run:

```go
// internal/crm/crm.go — package crm, written by hand
// internal/crm/app/…    — the routes
// internal/crm/trilha_gen.go — package crm, func NewApp() *trilha.App

mux.Handle("/", crm.NewApp().Handler())
```

Precedence, most explicit first: `--package <name>`; the package the hand-written `.go` files
of the folder declare; the package an existing `trilha_gen.go` declares; `main`. The third
step is what makes the flag a one-off — the generated file remembers the choice, so
`trilha gen --check` in the CI needs no flag of its own.

Outside `package main` the constructor is exported (`NewApp`, since the caller lives in
another package) and no `func main()` is written. `trilha dev` and `trilha build` refuse such
an app and say what runs it: there is no binary here, the host has one.

## Exit codes

`0` success; `1` generation, compilation or execution error; `2` incorrect usage.
