---
title: CLI
description: The trilha commands and their options.
---

```text
trilha new <dir> [--module path] [--template blog|app] [--with recipes] [--lang en|pt] [--agents]
    [--trilha-dir ../trilha] [--no-tidy]
trilha gen [--check] [--package name]
trilha generate page|route|test <url> | component <Name>
    [--methods GET,POST] [--bind Type] [--form Type] [--layout file] [--force] [--dir path] [--lang en|pt]
trilha dev [--addr :3000]
trilha build [-o bin/<name>]
trilha export [-o out] [--base /prefix]
trilha openapi [-o file] [--title T] [--version V] [--server URL] [--check]
trilha routes
trilha check [--json] [--fix]
trilha ctx [--json] [--routes|--types|--all]
trilha audit [--no-vuln]
trilha ui [--force] [--css-only|--js-only]
trilha ui describe [Name] [--json]
trilha agents [--force] [--lang en|pt]
trilha mcp [--write]
trilha version
```

| Command | What it does |
|---|---|
| `new` | creates a project with `go.mod`, layout, home page, 404, one API route, `public/style.css` and `.gitignore`; runs `go mod tidy` and `gen` |
| `gen` | scans `app/` and writes `trilha_gen.go`; fails with one line per violated convention |
| `generate` | writes one skeleton — a page, an API route or a component — in the folder the convention asks for |
| `generate crud` | reads a struct and writes the whole thing: listing, create, edit, delete, a store and a test |
| `add` | writes a framework recipe into the project: the audit trail, API keys, a settings section |
| `dev` | `gen` + `go build` + runs the app on an internal port + proxy on `--addr` + reload over SSE + route inspector on `/_trilha/routes` |
| `build` | `gen` + `go build -trimpath -ldflags="-s -w"` with `CGO_ENABLED=0` |
| `export` | `gen` + `go build` + runs with `TRILHA_EXPORT` to produce static HTML |
| `openapi` | writes the OpenAPI 3.1 document of the API routes (`-o -` to stdout) |
| `routes` | prints `METHODS PATTERN SOURCE` for each route |
| `check` | the single gate: `gen`, `gofmt`, `vet`, `test`, `audit` and `openapi`, in that order, stopping at the first failure |
| `ctx` | the map of the project — routes, API, types, setup — in one read, as Markdown or JSON |
| `audit` | security checklist before publishing (see [Security](/reference/security)) |
| `agents` | writes `AGENTS.md` and `CLAUDE.md` so a coding agent finds the conventions |
| `mcp` | MCP server over stdio for an agent without a shell; read-only unless `--write` |

Commands run in the folder containing `app/`. The project's import path comes from the
nearest `go.mod`, plus the subfolder, so an app can live inside a larger module.

## trilha new --template

`--template` picks the shape of the project:

| Shape | What it writes |
|---|---|
| `blog` (default) | a home page, an API route and a 404: the smallest thing that runs |
| `app` | the management app — login, [shell](/reference/shell), dashboard with [charts](/reference/charts), a [listing](/reference/listings) and a form, with tests |

The `app` project comes green: it compiles, `trilha check` passes and `go test ./...`
passes without a single edit. Its seeded account is printed on the login page, and every
route below `app/` is behind a session because the middleware sits at the root of the
folder — including the routes you add tomorrow.

### What comes in `app`

Beyond the skeleton, it asks [`trilha add`](#trilha-add) for the three screens every internal
application grows in its first month, and puts them under `app/admin/`:

| Screen | What it is |
|---|---|
| `/admin/auditoria` | the trail of who did what, with `ui.AuditTable` |
| `/admin/chaves` | API keys: issue, revoke, and the key shown once |
| `/admin/config` | a settings section, drawn from the struct that declares it |

They are **the recipes and not a second copy** — one source, so the template cannot age apart
from what `trilha add` writes. `app/admin/` requires the `admin` role, and somebody signed in
without it gets **403 and not a redirect to the login**: they are known, just not permitted, and
sending them back to a login they already passed is a loop with no exit.

`--with` decides which recipes a new project starts with, on any template:

```bash
trilha new minha-app --template app --with ""              # the skeleton alone
trilha new meu-site --with audit                           # on the blog template too
trilha new minha-app --template app --with audit,settings  # pick your own
```

An empty `--with` is a choice, not an absence: typing it asks for the skeleton and gets it.

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

## trilha build

`-o` names the binary; without it, `bin/<project folder>`. On Windows the output gets the
`.exe` the system needs to run it, whether the name came from `-o` or from the default —
`trilha build -o bin/app` writes `bin\app.exe`, because a file called `app` is one
`exec.LookPath` refuses to execute. A name that already ends in `.exe` is left alone, and
nothing changes on Linux and macOS.

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

### The contract, not only the folder

Without flags the skeleton is generic, and what is left to write — the struct, the `Bind`, the
validation, the answer, the test — is exactly where a signature gets typed wrong. The flags
write that part:

```bash
trilha generate route /api/posts/{id}/comments --methods GET,POST --bind Comment
trilha generate page /contact --form Contact --layout app/layout.go
trilha generate test /api/posts
```

- `--methods` writes one handler per method, in the signature the scanner reads, with
  `c.Param("id")` already there for each parameter of the path.
- `--bind Type` makes the methods that carry a body do `c.BindJSON(&in)`: returning that error
  is the 422 with the fields, so there is nothing else to handle. A type the project already
  declares is imported from where it is; one it does not have is born in the route's package
  with example `json` and `validate` tags. A name declared in two packages is refused, and the
  message says to write `posts.Comment`.
- `--form Type` on a page writes the whole round trip: `trilha.CSRFInput`, one `ui.Field` per
  field with the message beside it, 422 with `trilha.FieldErrors` when the `Bind` refuses and
  `POST → redirect → GET` when it accepts.
- `--layout <file>` writes the `layout.go` that is missing above the page. A path that does not
  wrap the page is refused: the scanner would never apply it, and finding that out costs a
  round trip.
- `generate test <url>` writes the test next to the route, in its own package, with one case
  per method the scanner finds — and a body built from the tags when it can read the type the
  handler binds. Right after generating, `trilha check` is green with nobody editing anything.

`--lang en|pt` chooses the language of the comments in the skeleton; identifiers, field names
and error messages stay in English.

## trilha ui

Writes or updates the UI kit in `public/`: `ui.theme.css` (only created; it is your theme),
`ui.css` and `ui.js` (updated; if edited locally, only with `--force`). `--css-only` and
`--js-only` limit what is touched. `trilha new` runs the same step. See
[UI kit](/learn/ui-kit).

### trilha ui describe

Prints the catalogue of the kit: with no argument, every component grouped and summarised in
one line each; with a name, its signature, what it is for, the fields of an options struct, an
example and the symbols the documentation cites.

```bash
trilha ui describe            # everything, grouped
trilha ui describe Field      # one component
trilha ui describe ui.field   # the same one: the prefix and the case are optional
trilha ui describe --json     # the whole catalogue as JSON
```

The catalogue is built from the kit's own doc comments and ships inside the binary, so it
answers with or without a project around it and cannot drift from the code it describes. An
unknown name exits non-zero with the closest names. `--json` is there for the agent writing
the screen: one call and it knows what exists and how each thing is spelled, instead of
guessing at a name and finding out at compile time.

## trilha client

Generates the Go client of an API that already exists, from the OpenAPI document that API
publishes. It is the other direction of `trilha openapi`, which writes the document of *your*
routes.

```bash
trilha client openapi.json                       # writes internal/api/client.go
trilha client https://api.example.com/openapi.json --out internal/acervo
trilha client openapi.json --check               # fails when the file is out of date
```

One file, deterministic, committed like `trilha_gen.go`. Inside it: a struct per schema of
`components/schemas`, with `json` tags and — where the document says `required`, `minLength`,
`format: email`, `enum` — the `validate` tags of [Validation](/reference/validation), so the
same type is the answer of the API and the `Bind` of a form. A method per operation, grouped
by tag: path parameters in the signature, query parameters in a struct, a JSON body as the
schema's type, an upload as `io.Reader` plus a filename, and a binary answer as the
`*http.Response`, so it streams into `c.Pipe`.

`New(base, WithHeader(...), WithClient(...))` is the whole surface of the constructor: the
client holds no credential of its own, and `WithHeader` runs per request, which is where the
session's token goes. A status outside 2xx is an `*Error` with `Status`, `Body` and the
`Detail` pulled out of `problem+json` or of the `{"detail": ...}` a FastAPI writes. The
generated file imports the standard library and nothing else — not even Trilha — so it also
works in a job, in a test, in a binary that is not a web app.

What it will not guess at, it says out loud: `oneOf` and `anyOf` and a schema with no type
come through as `json.RawMessage`, and each one is a line of the report the command prints.
`allOf` is flattened into one struct, a `$ref` that closes a cycle becomes a pointer, and an
operation with no `operationId` gets its name from the method and the path — also a line of
the report.

The recipe [An app in front of an existing API](/cookbook/existing-api) has the two halves
side by side: the page reading the API through this client, the islands reading it through
`Config.Upstreams`, one session token for both.

## trilha vendor

Puts one JavaScript module in the repository. An island that needs a helper — a template
library, a chart, a component runtime — gets it as a file under `public/vendor/`, downloaded
once and pinned:

```bash
trilha vendor preact@10.19.3           # public/vendor/preact.js + a line in vendor.lock
trilha vendor htm@3.1.1
trilha vendor                          # what is pinned
trilha vendor --check                  # re-hash the files against vendor.lock
trilha vendor preact@10.19.3 --from https://cdn.example.com
```

It downloads exactly what was asked for and resolves nothing: there is no dependency tree, no
`node_modules`, no install step, and a module that needs a resolver is the wrong module for an
island. `vendor.lock` holds the name, the version, the sha256 and the URL, and it is committed
— so a version bump is a diff, and a file that changed under a version that did not is what
`--check` catches. `trilha audit` says the same thing about a file in `public/vendor/` that
`vendor.lock` does not name.

The default source is `https://esm.sh`, which serves npm packages as ES modules; `--from` or
`TRILHA_VENDOR_BASE` points anywhere else. Nothing about this is a framework dependency: the
file is served like any other asset, and the island imports it by path.

## trilha migrate

Reads a Next.js project and writes the two things that are mechanical about a migration: the
folder tree of `app/`, with one Go file per screen, and a report of everything that is not.

```bash
trilha migrate next ../web --dry-run   # prints the report, writes nothing
trilha migrate next ../web             # writes app/ and MIGRATION.md
trilha migrate next ../web --out app --report MIGRATION.md --force
```

What it writes compiles: each page is a `Page` function with a title and the route's
parameters, each `route.ts` becomes the handlers it exported returning `501`, and `trilha gen`
right after leaves the project green. Nothing is overwritten without `--force`, so running it
on a project that already has screens adds what is missing and keeps what is there — the count
at the end says how many were written and how many were kept.

The doc comment above each function is the part that matters: it says which file it came from,
how many lines it had, which hooks it used, which endpoints it called, and which of three
shapes the screen probably is — **A** a form or a list with no island, **B** a page with one
island, **C** an app that really is a client. That is a suggestion printed with its reason, not
a verdict. `MIGRATION.md` gathers the same thing in one table, plus the list of what has no
equivalent here: loading states, templates, parallel and intercepting routes, middleware and
rewrites, each with the sentence explaining what takes its place.

The screens themselves are not translated. A page's body is business logic, and a machine
guessing at it would cost more to review than to write — the guide
[From Next.js to Trilha](/cookbook/from-next) has the React pattern beside the line that
replaces it.

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

Run it again after upgrading the CLI: `AGENTS.md` names the commands of the version that
wrote it, so a copy from an older release keeps sending the agent to commands that were
replaced. The whole sequence for a project coming from an older version is in
[Migration](/cookbook/migration#turning-on-the-agent-files-in-a-project-that-already-exists).

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

## trilha generate crud

Between `trilha new --template app`, which brings **one** finished CRUD as an example, and
`trilha generate page`, which writes an empty page, sits the task people repeat most: *I have a
struct, I want the screen.* Ten times in one project, each with the same skeleton and a different
small mistake — one has no delete confirmation, one does not show the validation error in the
field, one loses the page on the way back.

```bash
trilha generate crud docs.Tipo --at app/admin/tipos
```

```text
  + internal/docs/tipo_store.go       TipoStore: List/Get/Create/Update/Delete, and memory behind it
  + app/admin/tipos/page.go           listing: ui.DataTable, search, sorting, pagination, delete
  + app/admin/tipos/new/page.go       create: ui.Field per field, 422 comes back with the messages
  + app/admin/tipos/id_/page.go       edit: the same form, filled in
  + tipo_crud_test.go                 empty list, create, edit, 422, delete
  + app/setup.go                      the store, where the pages find it
```

What comes out is code somebody reads and edits, in the shape `templates/app` already proved
idiomatic — not a runtime that hides the screens behind a call. It exists so the tenth screen of
a project looks like the first.

### How the fields become the screen

The struct is read by **static analysis**, the same way `--form` already reads one: no compiler,
no module download.

- **`ID` is the key**, and a struct without one is refused with that sentence. The generator does
  not pick a field: the wrong key does not show up until the first `Update`, and by then there
  are five screens written on top of it.
- **`CriadoEm`/`CreatedAt`/`AtualizadoEm`/`UpdatedAt` and `json:"-"` stay out of the form.** They
  belong to the system, and a date somebody types by hand is a bug waiting. The store stamps
  them: created stays created on an update, because that is the one date people go looking for.
- **`validate:"required,max=80"`** becomes `required` and `maxlength` on the control, and the
  message beside the field when it fails.
- The label comes from the field name in title case, and changing it is editing a string in the
  file that was just generated — which is where it belongs.
- The first four fields become the table's columns. Four is what a table shows before it starts
  scrolling sideways.

### Nothing is overwritten, and there is no --force

A file that already exists is a refusal naming it. That is deliberate: a generator that
overwrites is a generator nobody runs twice, and a CRUD is precisely what you generate *after*
having edited one.

The single exception is `app/setup.go`, which the generator edits rather than refuses — one line,
into a function whose shape the framework defines. Without it the CRUD compiles and answers 500
on the first request, which is the worst outcome a generator can have, because it looks like it
worked. The command names that file in its output, because a generator that changes something you
did not ask for owes you the sentence.

### The store is an interface with memory behind it

The same choice every store in this framework makes, and here it pays twice: **what comes out of
the generator runs** — the screen opens, the form saves, the generated test passes — and swapping
memory for a database is implementing five methods whose signatures are already written. The
[database recipe](/cookbook/database) is the other half.

### Not here yet

`--store sqlite|postgres` with a generated migration, `--tenant`, `--policy`, the compact
`ui.SchemaForm` version, and re-running to print the diff of fields you added to the struct
later. They are on [#115](https://github.com/emersonjoe/trilha/issues/115); the SQL store in
particular would have to pick a placeholder dialect and own a DDL, which is the thing this
framework does not do anywhere else.

## trilha add

Half of what a management app needs is a **pattern**, not a primitive: the trail of who did what,
the screen that issues API keys, the settings section somebody edits instead of redeploying. In
the framework they would be rigid; as documentation they are work to copy. `trilha add` writes
them into the project as files it then owns.

```bash
trilha add              # what there is, one line each
trilha add audit
trilha add audit --dry-run
trilha add --list --json    # for the MCP server and an editor's agent
```

```text
  + internal/auditoria/store.go
  + app/auditoria/page.go
  ~ app/setup.go (one line added)

Run `trilha dev` and open /auditoria. Guard it: the trail says who did what, and that is
not for everybody.
Doc: https://trilha.dev/reference/observability
```

The difference from `generate` is the direction: **`generate crud` writes from your code** — a
struct becomes a screen — and **`add` writes from a recipe of the framework's**. Both end with
`gen`, and both leave `trilha check` green.

### Running it twice adds; it does not start over

A file that is already there is skipped with a note, not overwritten and not refused. By the
second run the file belongs to whoever received it, and they have probably edited it.

The one file a recipe edits is `app/setup.go`, and the insertion is **marked**:

```go
func Setup(a *trilha.App) error {
	// trilha:add audit
	a.Config().Audit = auditoria.Store
	return nil
}
```

That marker is what makes the second run recognise its own line instead of adding a second copy
of a store nobody wanted twice. Three recipes leave one import block, not three one-line groups —
the file is somebody's to read.

`--dry-run` prints all of that and writes nothing, which is what to run before letting a command
touch a project that already has code.

### The recipes

| Recipe | What it writes |
|---|---|
| `audit` | the sink `Config.Audit` receives, and the screen that reads it with `ui.AuditTable` |
| `api-keys` | the issuer, the screen that creates and revokes, and the key shown once with `ui.SecretOnce` |
| `settings` | a section declared as a struct, and the screen `ui.SettingsForm` draws from it |

Each one comes with memory behind it, so the screen works from the first request, and a comment
saying where a database goes. Each also says, in the file, that the folder needs guarding: an
audit trail names people, and a keys screen issues credentials.

More are coming — `login`, `share-link`, `webhooks`, `mail`, `blob`, `tasks`, `permissions`,
`tenant` — on [#116](https://github.com/emersonjoe/trilha/issues/116).

### Why the CI applies every one of them

A recipe that broke quietly is worse than no recipe, because whoever ran it already has its code
inside their project. So the test does not read the templates: it creates a project, applies every
recipe to it, and runs `trilha check` — compile, vet, test, audit — with nobody editing anything.

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

Two more sections appear only when the project has them, so a project that uses neither pays
nothing — no lines, no tokens:

```text
- `GET POST /docs` — app/docs/page.go · middleware: app/docs/middleware.go · needs docs:ver · POST needs docs:editar

## Enums

- `doc.situacao` — `rascunho` (Rascunho), `enviado` (Enviado · info) · app/setup.go
```

**What a route demands** is read from the `middleware.go` that declares it, and inherited the way
the middleware chain is: the deepest folder wins, and a `MiddlewarePOST` shows on that method
alone. It resolves the two shapes the documentation teaches — the guard called inline, and the
package-level var the `Middleware` function returns — including a wrapper of your own that hands
two strings to `RequirePolicy`, which is the idiom `examples/local-login` uses. What it cannot
read does not appear: a module that comes from a variable would take a compiler, and a map that
guesses is worse than a map that is quiet about one folder.

**The enums** are the domain lists `trilha.RegisterEnum` registered, joined to the declaration
they came from. They are here because somebody who does not know a list exists invents a second
one, and then two screens spell "in review" two ways. The same lists are readable at runtime with
`trilha.LookupEnum` and `trilha.RegisteredEnums`.

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
