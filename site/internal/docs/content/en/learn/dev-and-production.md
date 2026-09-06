---
title: Development and production
description: What trilha dev does under the hood, how to publish a binary and how to configure through environment variables.
---

## `trilha dev`

The command listens on `:3000` and runs your app on an internal port, forwarding the
requests. On every saved file:

1. it regenerates `trilha_gen.go` if the `app/` tree changed;
2. it recompiles with `go build`;
3. it starts the new process, waits for it to answer and only then stops the old one;
4. it notifies the browser through an event (SSE), which reloads.

Changes only in `public/` skip steps 1 to 3. A compile error becomes a page with the output
of `go build`; fix it and the page goes away. The app process runs with `TRILHA_ENV=dev`,
which turns on stack traces in error pages and turns off the static file cache.

## The route inspector

While `trilha dev` runs, `http://localhost:3000/_trilha/routes` answers with the map of the
app: every route in the order the router decides, with its kind, its methods, the folder it
comes from, the layouts that wrap it (outermost first) and the middlewares that run before
it — the two things `trilha routes` cannot show, because they are chains, not lines.

The box at the top answers the question that usually brings someone there: type `/blog/hello`
and the page says which pattern serves it and what each parameter is worth. The answer comes
from an `http.ServeMux` built from your patterns, so it is the router deciding, not a second
implementation of the precedence rules.

The page is served by the dev supervisor, not by your app: it is not in the binary
`trilha build` produces, and the same URL in production is a 404 like any other. There is
nothing to turn off before publishing.

## `trilha build`

```bash
trilha build            # → bin/agenda
TRILHA_ENV=prod PORT=8080 ./bin/agenda
```

The binary is static (`CGO_ENABLED=0`), embeds `public/` and needs neither the CLI nor any
file next to it. A `Dockerfile` fits in four lines:

```text
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN go run github.com/emersonjoe/trilha/cmd/trilha@latest build -o /app

FROM gcr.io/distroless/static
COPY --from=build /app /app
ENV PORT=8080
CMD ["/app"]
```

## Environment variables

| Variable | Effect |
|---|---|
| `PORT` or `ADDR` | listening port or address (default `:3000`) |
| `TRILHA_ENV` | `dev` or `prod` (default `prod`) |
| `TRILHA_BASE_PATH` | URL prefix when the app lives under a subpath; use `c.Base()` in links |
| `TRILHA_EXPORT` | output folder: instead of serving, export the static site and exit |
| `TRILHA_DEV_RELOAD` | `off` disables the reload script injection in dev (snapshot tests, HTML comparison); stack traces and `no-cache` stay |

Other settings (body limit, logger, CSRF in APIs) live in `trilha.Config`, which the
generated file builds with `trilha.ConfigFromEnv()`.

## Startup with `setup.go`

Opening a database, loading a cache, validating variables: all of that goes in
`app/setup.go`:

```go
package app

import "github.com/emersonjoe/trilha"

func Setup(a *trilha.App) error {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		return err // aborts startup with the message in the terminal
	}
	a.Values()["db"] = db
	return nil
}
```

The idiomatic Go way is to keep the pool in a variable of your own package
(`internal/db`), imported by the pages. `a.Values()` exists for quick glue.

## `trilha export`

If every page is static (a blog, documentation), export HTML and publish on any host:

```bash
trilha export -o out --base /agenda
```

Pages with parameters are included when `Setup` declares them with
`a.AddExportPath("/events/x")`. Pages that answer a same-site redirect become a small HTML
stub pointing to the destination. The site you are reading was generated this way.

## Assets and cache

Publishing new HTML with old CSS is the bug nobody can reproduce ten minutes later. The
cause is always the same: the file's address did not change when its content did, and some
cache layer — the browser, a CDN, GitHub Pages — still holds the old version.

`Asset` puts the content hash in the URL:

```go
h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/style.css"))) // /style.css?v=8f3a1c92
```

With that, a long cache becomes safe:

```go
cfg.StaticCacheControl = "public, max-age=31536000, immutable"
```

Whoever asks for the right versioned URL gets the one-year cache; whoever asks for
`/style.css` without a version falls under the normal rule. In `dev` nothing is immutable and
the hash follows the file, so saving the CSS and refreshing the page is enough.
`trilha export` needs no option: the exported HTML comes out with the same URLs, because the
same layout generates it.

`trilha audit` warns when it finds `immutable` in a project that does not use `Asset` — the
combination that freezes a file for a year at the wrong address.

## Secure by default

`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` and `Referrer-Policy` headers on
every response; limited body; CSRF on forms; static files without *path traversal*; logs
with method, path, status and duration, never with body or cookies. Errors in production
show an opaque page and go to the log with the `request_id` that appears in the
`X-Request-ID` header.

## Challenge

Publish the agenda on a server with `systemd` and make the service restart on its own if it
crashes.

:::solution
```text
[Unit]
Description=agenda
After=network.target

[Service]
ExecStart=/opt/agenda/bin/agenda
Environment=PORT=8080 TRILHA_ENV=prod
Restart=always
User=agenda

[Install]
WantedBy=multi-user.target
```
:::
