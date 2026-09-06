---
title: Docker
description: A static binary in a distroless image, the assets already inside it, the variables it needs, and a health probe the orchestrator can use.
---

A Trilha app is one binary with the pages compiled in and, if you used `//go:embed`, the
static files too. That makes the image small enough that the interesting part is what you
leave out.

## The Dockerfile

```dockerfile
FROM golang:1.22 AS build
WORKDIR /src
# Dependencies first: this layer is cached until go.mod changes.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The generated file is committed, but generating it again in the build is
# how you find out someone forgot to run trilha gen.
RUN go run ./cmd/trilha gen
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app ./

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /app /app
# The port is documentation; the platform decides what it publishes.
EXPOSE 3000
USER nonroot
ENTRYPOINT ["/app"]
```

Two lines carry the weight. `CGO_ENABLED=0` makes a static binary, which is what lets the
second stage be `distroless/static` — no shell, no package manager, nothing to exploit that
is not your code. `nonroot` means a bug in your app is a bug running as uid 65532.

:::warning
`CGO_ENABLED=0` and the SQLite drivers that need cgo are mutually exclusive. Either use a
pure-Go driver (`modernc.org/sqlite`) or build on `debian:bookworm-slim` and accept the bigger
image.
:::

## The address

`trilha.ConfigFromEnv` already reads `PORT` and `ADDR`, so the platform that hands over a port
is obeyed with no code — the default is `:3000`, and `:3000` means every interface, which is
what a container needs. The most common broken image is the one that was told to bind
`127.0.0.1`: inside the container that is the container itself, and nothing outside reaches
it.

## The variables

| Variable | What it is |
|---|---|
| `TRILHA_ENV` | `prod` — turns off the dev reload, the error page with source, the verbose log |
| `TRILHA_SECRET` | the key for cookies and CSRF; at least 32 bytes, from the platform's secret store |
| `PORT` or `ADDR` | where to listen; `:3000` by default |
| `DATABASE_URL` | your pool's DSN |
| `TRILHA_BASE_PATH` | only when the app is not at the root of the domain |

A secret baked into the image is a secret in the registry, and in every layer cache that ever
pulled it. Rotating one is `TRILHA_SECRET_PREVIOUS` with the old value for a deploy or two, so
sessions signed with the old key keep working while they expire.

## Compose

```yaml
services:
  app:
    build: .
    environment:
      TRILHA_ENV: prod
      DATABASE_URL: postgres://app:app@db:5432/app?sslmode=disable
    env_file: [.env]           # TRILHA_SECRET lives here, not in this file
    ports: ["8080:3000"]
    depends_on:
      db: { condition: service_healthy }
    healthcheck:
      test: ["CMD", "/app", "-health"]
      interval: 10s
  db:
    image: postgres:16-alpine
    environment: { POSTGRES_PASSWORD: app, POSTGRES_USER: app, POSTGRES_DB: app }
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app"]
      interval: 5s
    volumes: [pgdata:/var/lib/postgresql/data]
volumes:
  pgdata:
```

A distroless image has no `curl` and no shell, so the health check cannot be a shell command
against a URL. Two ways out: a `-health` flag in your own binary that requests
`/_trilha/health/ready` and exits with the status, or the orchestrator's own probe — which is
what Kubernetes does, and it does not need anything inside the image:

```yaml
livenessProbe:
  httpGet: { path: /_trilha/health/live, port: 3000 }
readinessProbe:
  httpGet: { path: /_trilha/health/ready, port: 3000 }
  periodSeconds: 5
```

`live` says the process is up; `ready` says it can serve, and it is the one that goes red when
the database is gone. Wiring them the other way round is how a container that lost its
database gets restarted forever instead of being taken out of the pool.

:::note
There is a smaller answer than a container. `trilha export` writes a static site when the app
has no dynamic route, and `trilha build` writes the binary if all you need is to copy a file
to a machine and run it under systemd. Not everything needs an orchestrator.
:::
