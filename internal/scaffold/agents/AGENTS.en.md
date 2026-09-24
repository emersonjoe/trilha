# AGENTS.md

Instructions for coding agents working on `{{.Name}}`, a web app built with
[Trilha](https://github.com/emersonjoe/trilha) — a Go framework with file-based routing and no
external dependencies.

## Read narrow, in this order

1. **Map first, files later.** `trilha ctx --json` is the map of this project — routes, API
   contracts, types, setup — in one read, priced in tokens at the foot. `trilha ctx --pack
   <recipe>` slices it for one job and `--budget N` caps what comes back. Open files only
   after the map.
2. **Search, don't read.** Grep with line numbers (`grep -n -A5`) and read the window, never
   the whole file.
3. **Ask the catalogue.** `trilha ui components --json` lists every ui component before you
   build a screen; `trilha add` lists the recipes before you write one by hand.

## Recipes installed

The map (`trilha ctx`) lists the recipes this project installed, each with its documentation
link, and `trilha ctx --pack <recipe>` answers "what does this recipe touch?". The catalogue
of everything available: <https://emersonjoe.github.io/trilha/cookbook>.

## The gates

| Command | What it does |
|---|---|
| `trilha check` | the single gate — gen, gofmt, vet, test, audit, openapi, stopping at the first failure, every problem with its line and its fix. Run it before saying you are done |
| `trilha check --fix` | the same, rewriting `trilha_gen.go` and the formatting on the way |
| `make test` | the full suite; CI runs it |
| `trilha gen` | rewrites `trilha_gen.go` after a route is added or removed |

A route answering 404 is almost always a missing `trilha gen`; `trilha check` catches it
before the browser does.

## The three conventions

- **A folder under `app/` is a URL.** `app/blog/page.go` answers `/blog`; the file name says
  what it is — `page.go` renders, `route.go` is an API, `layout.go` wraps its subtree,
  `middleware.go` runs before it.
- **`slug_` is a parameter.** `app/blog/slug_/page.go` answers `/blog/{slug}`, read with
  `c.Param("slug")`. A name ending in `-` is a group that adds no URL segment.
- **HTML is Go, escaped by default.** `h.Div(h.Class("card"), h.Text(title))` — there are no
  templates, and nothing reaches the page unescaped unless you call `h.Raw`, which you almost
  never should.

## Do not

- **Do not edit `trilha_gen.go`** — generated and committed; the next `trilha gen` overwrites it.
- **Do not add a dependency.** The answer is in the standard library or in the framework.
- **Do not put a secret in the code.** Read it from the environment; `trilha audit` fails on a
  literal that looks like a key.
- **Do not write your own CSRF, sessions or escaping** — all three exist and are on by default.
  A write in a `route.go` needs `var Kind = trilha.KindPage` in a `kind.go` beside it.
- **Do not hand-write a listing.** `trilha.ListParams` + `ui.DataTable`: ordering, filter,
  search and pagination live in the URL and render as a fragment.
- **Do not hand-build an internal app's frame.** `ui.Shell` is the sidebar and the user menu;
  `ui.Stat`/`ui.Bars` draw dashboards as server-side SVG. Hiding a menu item is cosmetics —
  the middleware is the lock.
- **Do not hand-write autocomplete, upload queues or chat.** `ui.Combobox`, `ui.Dropzone` +
  `ui.UploadTo`, and `ai.Serve` + `ui.Chat` exist; read `trilha ui describe <Name>` first.

## Where to look

- Recipes: <https://emersonjoe.github.io/trilha/cookbook> · Reference:
  <https://emersonjoe.github.io/trilha/reference> · Bulk text:
  <https://emersonjoe.github.io/trilha/llms.txt>
