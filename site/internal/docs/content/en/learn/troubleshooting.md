---
title: Troubleshooting
description: Errors that show up in the first minutes and what each one means.
---

## `zsh: command not found: trilha`

`go install` placed the binary in `~/go/bin` (or whatever `go env GOPATH` shows plus
`/bin`), and that folder is not in your `PATH`. Add it to `~/.zshrc` or `~/.bashrc` and open
a new terminal:

```bash
export PATH="$HOME/go/bin:$PATH"
```

## `verifying module ... 404 Not Found` on `go install`

The module lives in a private repository, or it just became public and the proxy does not
know it yet. The `sum.golang.org` checksum database can only verify public modules. For a
private module, tell Go not to verify:

```bash
go env -w GOPRIVATE=github.com/your-org/*
```

For a freshly published module, prefer installing by tag (`@v0.1.0`) instead of `@latest`.

## `app/ directory not found`

CLI commands run at the project root, the folder containing `app/`. If the app lives inside
a larger module (like `examples/blog` in Trilha's repository), run the CLI inside that
subfolder: the import path is computed from the nearest `go.mod`.

## `E_NO_PAGE_FUNC` or `E_NO_METHOD`

The file exists, but the expected function is not exported with the right name. `page.go`
needs `Page`; `route.go` needs at least one of `GET`, `POST`, `PUT`, `PATCH`, `DELETE`;
`layout.go` needs `Layout`; `middleware.go` needs `Middleware`. A wrong signature is a
compile error in `trilha_gen.go`, pointing at the package.

## `E_UNUSED_METHOD_MIDDLEWARE`

A `MiddlewarePOST` (or `GET`, `PUT`, `PATCH`, `DELETE`) in a `middleware.go` that reaches no
route serving that method in its folder or below it. Usually the method moved and the rule
stayed, or the name has a typo. Either delete it, or give the route the method it is meant
to guard — a permission that guards nothing is worse than no permission, because it reads
like protection.

## `E_DUPLICATE_ROUTE`

Two folders produce the same URL, almost always because of a route group. `app/events/` and
`app/organizer-/events/` both answer at `/events`. Rename one of them.

## `E_HIDDEN_ROUTE`

A `page.go` or a `route.go` inside a folder whose name starts with a dot. The scanner skips
those folders, so the route would never answer — it used to disappear without a word, and
the only symptom was a 404. Rename the folder without the leading dot, or, if the folder is
meant to stay out of the routing, start its name with `_`. The single dot folder that *is*
routed is `.well-known` (see [conventions](/reference/conventions#folders)).

## `E_UNROUTABLE_METHOD`

`func HEAD`, `func TRACE` or `func CONNECT` in a `route.go`. The router does not take those
from a file, so the function used to compile and answer nothing: the request fell into the
405 the fallback writes before any middleware. HEAD is not missing — since Go 1.22 the
router answers it with the `GET` handler, so write the response there. `OPTIONS`, on the
other hand, is a handler like the others, and a route that only needs the preflight can
declare `var CORS` instead of writing it (see
[conventions](/reference/conventions#cross-origin-on-one-route)).

## The preflight answers 405

The route serves no `OPTIONS`. Either declare `var CORS = trilha.CORS{...}` in its
`route.go` — the framework then answers the preflight from the policy — or write `func
OPTIONS` by hand. `Config.CORS` also answers, but for the whole app: use it when every route
shares the policy, not to open three paths.

## The form answers 403

`trilha.CSRFInput(c)` is missing inside the `<form>`, or the form page was opened before the
cookie existed (for instance, a `curl` straight to the `POST`). Open the page with `GET`
first, as a browser would, or send the token in `X-CSRF-Token`.

## `trilha dev` says there is no binary here

The folder declares a package other than `main`, so `trilha gen` wrote an importable package
with `NewApp()` and no `func main()` — an app meant to be mounted by a host binary
(`mux.Handle("/", crm.NewApp().Handler())`). Run the host, not this folder. If the package
clause was a mistake, fix it in the hand-written file and generate again; the generated file
follows whatever the folder declares. See
[CLI](/reference/cli#an-app-inside-a-binary-that-already-exists).

## Port 3000 is busy

```bash
trilha dev --addr :3001
```

## The browser does not reload

The reload script is only injected when the response is HTML and goes through the layout. A
page returning `c.Text(...)` or `c.JSON(...)` does not get the script. Also check whether a
proxy (nginx, an extension) is blocking `/_trilha/events`, which is an SSE connection.

## I changed `public/` and nothing happened in production

In production `public/` is embedded in the binary. Run `trilha build` again. In development
the folder is read from disk and the change shows up immediately.

## The CLI speaks Portuguese (or English) and I want the other one

The CLI follows `TRILHA_LANG`, then `LC_ALL`, `LC_MESSAGES` and `LANG`. Set
`TRILHA_LANG=en` or `TRILHA_LANG=pt` to force a language; anything that does not start with
`pt` means English.
