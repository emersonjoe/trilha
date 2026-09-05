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
