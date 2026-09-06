---
title: Migration
description: From plain net/http to Trilha one route at a time, without a rewrite — and what to look at when you move between minor versions.
---

Nobody rewrites a working app. This is the other way: put Trilha in front, move one route,
deploy, and repeat until there is nothing left to move.

## From `net/http`

Here is the app as it was. A mux with the addresses in a table, a handler that starts by
finding out which address it is, a template executed by hand, and the error handling written
once per route:

```go
// Routes is the table every net/http app grows: one mux, one line per
// address, and a handler that starts by finding out which address it is.
func Routes(find func(string) (Article, bool)) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blog/{slug}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := find(r.PathValue("slug"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, a); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /api/articles/{slug}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := find(r.PathValue("slug"))
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(a); err != nil {
			return
		}
	})
	return mux
}
```

And the chain everybody writes again — headers, host check, recover:

```go
// Secure is the middleware chain: the headers, the request id, the log and
// the recover that every app writes again, in the order that matters.
func Secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Host, "example.com") {
			http.Error(w, "bad host", http.StatusMisdirectedRequest)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

### The same thing after

The address is where the file lives, `app/blog/slug_/page.go`, so nothing declares it twice:

```go
// Page is the same blog page after the move: the address is the folder it
// lives in (app/blog/slug_/page.go), the layout is applied for it, the 404
// is an error it returns, and the HTML is a value instead of a string.
func Page(c *trilha.Ctx) (h.Node, error) {
	a, err := ArticleBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return nil, err
	}
	c.SetTitle(a.Title)
	return h.Article(
		h.H1(h.Text(a.Title)),
		h.P(h.Time(h.Attr("datetime", a.Published.Format("2006-01-02")), h.Text(a.Published.Format("2 Jan 2006")))),
	), nil
}
```

```go
// GET is the same API route: no writer, no encoder, no Content-Type by
// hand. The error carries its own status, and an unexpected one becomes a
// problem+json body with the request id in it.
func GET(c *trilha.Ctx) error {
	a, err := ArticleBySlug(c.Context(), c.Param("slug"))
	if err != nil {
		return err
	}
	return c.JSON(200, a)
}
```

What disappeared is worth listing, because it is the whole trade:

| Written by hand before | Where it went |
|---|---|
| `mux.HandleFunc("GET /blog/{slug}", …)` | the folder `app/blog/slug_/` |
| `http.NotFound` per route | `return trilha.ErrNotFound`, negotiated as HTML or `problem+json` |
| `w.Header().Set("Content-Type", …)` | `c.JSON`, `c.HTML`, `c.Text` |
| the template, executed and checked | `h`, which is Go and escapes by construction |
| the security headers and the `recover` | the runtime, on by default |
| the layout repeated in every template | `layout.go` in the folder |

### One route at a time

You do not need a big-bang cutover. Trilha's app is an `http.Handler`, and so is your mux, so
either can be in front of the other:

```go
// Front is how the two systems share a process while the move happens: the
// old mux answers what has not been moved yet, and everything it does not
// know falls through to the framework. The old middleware still wraps both,
// so nothing loses its headers halfway.
func Front(mux *http.ServeMux, a *trilha.App) http.Handler {
	mux.Handle("/", a.Handler())
	return before.Secure(mux)
}
```

Move the leaves first — a page with no dependencies, an API route that only reads. Deploy
after each one. The two systems share the same process, the same pool and the same logger; a
route is either in one or the other, never half in both.

### When the app lives inside the old binary

`Front` above assumes the two live in the same `package main`. Often they do not: what is
being moved is one area of a larger server, and it wants its own folder — `internal/crm/`,
with its own `app/`. Declare the package by hand there and `trilha gen` follows it, writing
`NewApp` into the same package instead of a `main` nobody asked for:

```go
// Package crm is one area of a server that already exists: it has its own
// app/ folder and its own package name, written by hand in this file.
// `trilha gen` follows the package it finds here and writes NewApp into the
// same one, so the binary that hosts it mounts the app with no registration
// file of its own.
package crm
```

The binary that already exists mounts it like any other handler:

```go
// Host is the same move when the app does not live in package main: crm is a
// folder of the binary that already exists, `trilha gen` wrote NewApp into
// the package that folder declares, and mounting it is one line. There is no
// registration file to keep by hand. The nonce goes in on the way past,
// because the app renders its scripts under the host's policy.
func Host(mux *http.ServeMux, nonce func(*http.Request) string) http.Handler {
	mux.Handle("/", crm.NewApp().Handler())
	return before.Secure(withNonce(mux, nonce))
}
```

```go
// withNonce hands the app the nonce the host already published. Without it
// the app invents one per request, and the policy the browser is enforcing —
// the host's — has never heard of that one.
func withNonce(next http.Handler, nonce func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, host.WithNonce(r, nonce(r)))
	})
}
```

Three things stop being the app's while it is mounted in there, and all three are one line in
`app/setup.go`:

```go
// Config is where an embedded app says what is not its to answer for. The
// host already wrote the response headers and already published a policy with
// a nonce in it, so the app writes neither: Delegated sends none of the seven,
// and Nonce hands c.Nonce() the value the host's own policy names. The CSRF
// names move out of the way of the host's, because two hidden fields called
// _csrf on one page is a bug nobody sees until a form silently posts the wrong
// token.
func Config(cfg *trilha.Config) {
	cfg.Security.Delegated = true
	cfg.Security.Nonce = func(r *http.Request) string { return host.Nonce(r) }
	cfg.CSRF = trilha.CSRF{Cookie: "crm_csrf", Field: "_crm_csrf", Header: "X-CRM-CSRF"}
}
```

`Security.Delegated` writes none of the seven headers — the host already wrote them, and two
`Content-Security-Policy` on one response is a policy nobody can reason about. `Security.Nonce`
is the other half: the app's scripts have to carry the nonce that is in the host's policy, not
one it invented for itself. And the CSRF cookie, field and header take names of their own, so
the app's hidden `_csrf` and the host's are not the same field on the same page.

The fourth is the store. A package variable is shared by every app in the process, and there
is more than one in there now:

```go
// Setup provides what the pages need. The store is a value, not a package
// variable: this app is one of several in the process, and Use gives each one
// back its own.
func Setup(a *trilha.App) error {
	trilha.Provide(a, contacts.New())
	return nil
}
```

There is no hand-written registration file, which is the point: `trilha gen --check` in the
CI keeps catching the folder someone added without generating. `trilha dev` and `trilha
build` do not apply inside `internal/crm` — the binary is the host — and they say so. See
[CLI](/reference/cli#an-app-inside-a-binary-that-already-exists).

Two things do need a decision up front:

- **Sessions.** If the old app has its own cookie, keep reading it in a middleware while the
  new one writes `SetSigned`, and drop the old reader when everything has moved.
- **Static files.** `public/` is served by the framework with hashed URLs through `c.Asset`. A
  path written by hand in old HTML keeps working; it just does not get the long cache.

:::tip
Start with `trilha new` in an empty directory and copy your handlers into it, rather than
adding the framework to the existing tree. Comparing two directories is easier than
untangling one.
:::

## Between minor versions

The rule the project follows: before 1.0, a minor version may change what a new app looks
like, but the upgrade is always written down. In practice, four steps:

```bash
go get -u github.com/emersonjoe/trilha@latest
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha gen        # the generated file must match the CLI's version
trilha audit      # among other things, it compares CLI and library
make test
```

`trilha audit` is what catches the mismatch nobody notices: a `trilha_gen.go` written by an
older CLI serves the routes of an older `app/`. It is a warning, not a crash, which is
precisely why it is worth running.

The [changelog](https://github.com/emersonjoe/trilha/blob/main/CHANGELOG.md) is the source
for what changed; the `## What changes for you` sections of a release are written for this
moment. What follows the version bump is ordinary: read the section, run the tests, and if the
release added a convention (a new folder name, a new file that gets picked up), `trilha routes`
prints what the scanner now sees, which is the fastest way to check it saw what you meant.

:::note
A public symbol never disappears in a minor version without being deprecated in one first.
The versioned surface lives in `api/current.txt`, and a change to it that was not intended
fails the framework's own test suite.
:::

### Turning on the agent files in a project that already exists

`--agents` is a flag of `trilha new`, so it is of no use to a project that was created
before it existed. The command for that case is `trilha agents`, and it does exactly the
same thing — nothing has to be recreated:

```bash
go get -u github.com/emersonjoe/trilha@latest
go install github.com/emersonjoe/trilha/cmd/trilha@latest
trilha gen                 # the generated file must match the CLI's version
trilha agents              # writes AGENTS.md and CLAUDE.md (--lang pt for Portuguese)
trilha check               # the single gate: gen, gofmt, vet, test, audit, openapi
git add AGENTS.md CLAUDE.md trilha_gen.go
```

Both files are meant to be committed: the agent reads them from the repository, not from
your machine. `trilha ctx` needs nothing installed — run it once to see the map your agent
will be reading.

The step that is easy to miss is `trilha agents` **after every CLI upgrade**. `AGENTS.md`
describes the commands of the CLI that wrote it: a copy from 0.36.0 tells the agent to run
`make test` and never mentions `trilha check`, which arrived in 0.37.0. An untouched copy is
refreshed in silence; one you edited stops the command, and then you choose:

- `trilha agents --force` overwrites it and you add your rules back, or
- you move your rules to `CLAUDE.md`, which the command never overwrites, and leave
  `AGENTS.md` as the framework's file — which is what the split is for.

In CI, the line the agent files point at is the one that replaces the list of commands:

```yaml
- run: trilha check
```
