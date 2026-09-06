# AGENTS.md

Instructions for coding agents working on `{{.Name}}`, a web app built with
[Trilha](https://github.com/emersonjoe/trilha) — a Go framework with file-based routing and no
external dependencies.

## The three conventions

- **A folder under `app/` is a URL.** `app/blog/page.go` answers `/blog`.
- **The file name says what the file is.** `page.go` renders a page (`func Page(c *trilha.Ctx)
  (h.Node, error)`), `route.go` is an API (`func GET`, `func POST`, ...), `layout.go` wraps
  everything below it, `middleware.go` runs before everything below it.
- **A folder named `slug_` is a parameter.** `app/blog/slug_/page.go` answers `/blog/{slug}`,
  read with `c.Param("slug")`. A folder with a dot in its name is a fixed path instead
  (`app/api/report.csv/route.go` answers `/api/report.csv`).

HTML is written in Go with the `h` package, not with templates:
`h.Div(h.Class("card"), h.H1(nil, h.Text(title)))`. Everything it renders is escaped.

## Commands

| Command | What it does |
|---|---|
| `trilha check` | the single gate: gen, gofmt, vet, test, audit, openapi, in that order, stopping at the first failure. Run it before saying you are done — it is also the one line CI runs |
| `trilha check --fix` | the same, rewriting `trilha_gen.go` and the formatting on the way |
| `trilha ctx` | the map of the project — routes, API, request and response types, setup — in one read; `--json` for a tool, `--all` for nothing elided. Read it before opening files one by one |
| `trilha dev` | dev server with live reload; keep it running while you work |
| `trilha gen` | rewrites `trilha_gen.go` from `app/`; run it after adding or removing a route |
| `trilha generate page /path` | writes the skeleton in the right folder (also `route`, `component`) |
| `trilha routes` | lists every route the scanner found and the file it came from |
| `trilha audit` | checks security and configuration: secrets, CSP, cookies, dependencies |
| `trilha build` | generates and compiles a single binary |
| `trilha export` | writes the static pages as HTML |
| `trilha openapi` | writes the OpenAPI document of the API routes |
| `trilha ui` | rewrites the ui kit in `public/` |
| `trilha agents` | rewrites this file |
| `trilha new` | creates another project |
| `trilha version` | the version of the framework |

A route that answers 404 is almost always a missing `trilha gen`; `trilha check` catches it
before the browser does. Every problem it reports comes with the line it is on and the sentence
that resolves it — read that line instead of guessing.

## Do not

- **Do not edit `trilha_gen.go`.** It is generated and committed, and the next `trilha gen`
  overwrites whatever you put there. Change `app/` instead.
- **Do not add a dependency.** The framework runs on the standard library alone; the answer is
  usually in `net/http`, `database/sql`, or in the framework itself.
- **Do not put a secret in the code.** Read it from the environment. `trilha audit` fails on a
  literal that looks like a key.
- **Do not write your own CSRF, session signing or HTML escaping.** All three already exist and
  are on by default.

## Where to look

- Recipes for the usual problems (database, sessions, uploads, pagination, email, Docker):
  <https://emersonjoe.github.io/trilha/cookbook>
- Every function and type: <https://emersonjoe.github.io/trilha/reference>
- The whole documentation as plain text, cheaper to read in bulk:
  <https://emersonjoe.github.io/trilha/llms.txt>
