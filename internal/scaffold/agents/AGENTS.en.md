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
  (`app/api/report.csv/route.go` answers `/api/report.csv`). A folder whose name *starts*
  with a dot is ignored, except `.well-known`
  (`app/.well-known/security.txt/route.go` answers `/.well-known/security.txt`). A route
  fetched from another origin declares its own policy — `var CORS = trilha.CORS{...}` in the
  `route.go` — and the framework answers the preflight.

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
| `trilha generate page /path` | writes the skeleton in the right folder (also `route`, `component`, `test`); `--methods`, `--bind` and `--form` write the contract too |
| `trilha routes` | lists every route the scanner found and the file it came from |
| `trilha audit` | checks security and configuration: secrets, CSP, cookies, dependencies |
| `trilha build` | generates and compiles a single binary |
| `trilha export` | writes the static pages as HTML |
| `trilha openapi` | writes the OpenAPI document of the API routes |
| `trilha ui` | rewrites the ui kit in `public/` |
| `trilha ui describe [Name]` | the ui catalogue: every component, or one with its signature and an example; `--json` for a tool. Read it before writing a screen instead of guessing at a name |
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
  are on by default. A write that lives in a `route.go` is the exception: a `route.go` is an API
  and an API does not check the token, so put `var Kind = trilha.KindPage` in a `kind.go` at the
  root of that branch — it is inherited by everything below, and `trilha audit` reports the
  write no `Kind` reaches.
- **Do not write a reverse proxy to reach an API that already exists.** `Config.Upstreams`
  forwards a prefix with the session's credential injected, the CSRF token required, the body
  streaming past `MaxBodyBytes`, and a 502/504 in `problem+json`. A hand-written
  `httputil.ReverseProxy` in a catch-all `route.go` hits every one of those on its own.
- **Do not copy a session recipe for an app with its own users.** `auth.Sessions` is the same
  `*Auth` without a provider: check the password with `auth.CheckPBKDF2` and call
  `Login(c, u)`. Everything else — `Require`, `RequireRole`, the `Store`, the idle window —
  is already written.
- **Do not hand-write a listing screen.** Page, ordering, filter and search live in the
  URL with `trilha.ListParams` (embedded in the struct `c.Bind` fills) and are rendered by
  `ui.DataTable` — sortable headers as real links, the filter as a `<form method=get>`,
  pagination, empty state, and the whole thing swapped as a fragment when it has an `ID`.
  `Restrict` is what makes a `sort` from the address a column name, so no repository ever
  sees one nobody declared.
- **Do not write a `setInterval` to refresh a piece of the page.** `ui.Poll("6s", src)`
  refreshes a fragment on a clock — pausing on a hidden tab, backing off on errors,
  stopping when the route answers `c.PollStop()` — and `ui.Live`/`ui.On` do the same on a
  Server-Sent Event that carries only the name of what changed. Load `ui.LiveScript(c)`
  once on the page that watches something.
- **Do not hand-build the frame of an internal app, and do not reach for a charting
  library.** `ui.Shell` is the sidebar, the header and the user menu, with the active item
  found by the longest matching prefix; `ui.PageHeader` is the title of the screen inside
  it. `ui.Stat`, `ui.Bars`, `ui.Sparkline` and `ui.Donut` draw a dashboard as server-side
  SVG, with an invisible table of the same numbers for a screen reader. Hiding a menu item
  is cosmetics: what keeps somebody out is a middleware at the root of the folder.
- **Do not number the inputs of a repeating row by hand, and do not build a form engine.**
  A list of sub-records is read from `items[0].name`, `items[1].name`… into a `[]Row`, a
  matrix from `perm[docs]` into a map, and the name of the input is the key of the message,
  so `ui.Errors(errs, "items[1].qty")` already points at the right row. When the form itself
  comes from data, `trilha.Schema` decodes from JSON, `trilha.BindSchema` validates it like
  any other form and `ui.SchemaForm` draws it.
- **Do not write an autocomplete, and do not read a file field in a loop.** `ui.Combobox` is
  the text field that searches a list: a hidden input carries the chosen value, a `Source`
  route answers `ui.ComboboxOptions` with only the `<li>`s, and `With` carries the other
  fields of the form into the query. `ui.Dropzone` is the drop area, and with `ui.UploadTo`
  its queue sends one file per request. On the server, `c.Files(field, rules)` applies the
  rules of `c.File` to every file and names a failure by position — `files[2]` — so the
  message lands on the line that earned it.
- **Do not put model text on the page with `h.Raw`, and do not write a chat by hand.**
  `ui.Markdown(text, ui.MarkdownOpts{})` returns nodes, not a string: a tag in the text comes
  out as text, and there is no way to turn that off. `ai.Serve(c, client, agent)` is the whole
  chat route — it reads `{message, history}`, streams `text`, `tool_call`, `tool_result`,
  `done` and `error`, and answers at once when nobody asked for a stream — and `ui.Chat(c,
  ui.ChatOpts{Action: "/api/chat"})` with `ui.ChatScript(c)` is the screen. The history is
  yours to keep; the framework keeps none.
- **Do not hand-write the headers of a download.** `c.Attachment(name, body, ctype)` sends a
  file and `c.Inline` opens it in the browser; both sanitise the name, write it in the two
  forms browsers actually read, detect the type from the content and send `nosniff`. `Inline`
  refuses HTML, SVG and XML — that is the point of it being a second function. `c.Pipe(res)`
  hands another service's answer through, with a closed list of headers and no `Set-Cookie`.
- **Do not invent a flash cookie or an `onclick="return confirm()"`.** After a `POST`, say what
  happened with `c.Flash(ui.FlashSuccess, "…")` — the layout's `ui.Flashes(c)` shows it on the
  page the redirect lands on — and ask before destroying with `ui.Confirm(title, description)`
  on the form. Inline script is blocked by the CSP anyway.

## Where to look

- Recipes for the usual problems (database, sessions, uploads, pagination, email, Docker):
  <https://emersonjoe.github.io/trilha/cookbook>
- Every function and type: <https://emersonjoe.github.io/trilha/reference>
- The whole documentation as plain text, cheaper to read in bulk:
  <https://emersonjoe.github.io/trilha/llms.txt>
