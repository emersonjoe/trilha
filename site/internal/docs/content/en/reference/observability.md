---
title: Observability
description: Config.Observability, health endpoints, metrics registry, environment variables and the contract of each response.
---

## Config.Observability

| Field | Default | What it does |
|---|---|---|
| `Health string` | `/_trilha/health` | base path of the probes; `trilha.Off` removes them |
| `Metrics string` | `""` (off) | scrape path; empty registers no endpoint **and does not instrument requests** |
| `Token string` | `TRILHA_OBS_TOKEN` | authorizes details and metrics; **at least 32 bytes**, compared in constant time |
| `Trusted []string` | — | CIDRs (or IPs) that do not need the token |
| `Details string` | automatic | `trilha.Off` never reveals details, not even to a token holder; empty = open in `dev`, authorized in `prod` |
| `Timeout time.Duration` | 2 s | deadline of each check; `trilha.NoTimeout` disables it |
| `CacheFor time.Duration` | 1 s | validity of the readiness result; `trilha.NoTimeout` disables the cache |

Variables read by `ConfigFromEnv`: `TRILHA_OBS_TOKEN`, `TRILHA_METRICS`,
`TRILHA_OBS_TRUSTED` (comma-separated list).

## Endpoints

| Method and path | Response | Status |
|---|---|---|
| `GET /_trilha/health/live` | `application/health+json` | always 200 |
| `GET /_trilha/health/ready` | same, runs the checks | 200 or 503 + `Retry-After: 5` |
| `GET /_trilha/health` | same as `ready` | 200 or 503 |
| `GET <Metrics>` | `text/plain; version=0.0.4` | 200, or 401 without authorization |

All of them carry `Cache-Control: no-store`, `X-Robots-Tag: noindex` and
`X-Content-Type-Options: nosniff`. Any other method returns 405 with `Allow: GET, HEAD`.

The probes run **outside** the middleware chain: no CSRF, no layout, no rate limit (a
liveness probe that got a 429 would kill a healthy process) and logged at `Debug` level, so
they do not drown the audit log.

## Readiness checks

```go
func (a *App) Check(name string, fn func(context.Context) error)
func (a *App) HealthReport(ctx context.Context) HealthReport
```

```go
type HealthReport struct {
	Status        string        // "pass" | "fail"
	Checks        []CheckResult
	UptimeSeconds float64
}

type CheckResult struct {
	Name       string
	Status     string
	DurationMS float64
	Error      string
}
```

`HealthReport` always returns everything: it is for your code (an internal status page, a
startup gate). The endpoint decides what to reveal.

## Metrics registry

```go
func (a *App) Metrics() *Metrics

func (m *Metrics) Counter(name, help string, labels ...string) *Counter
func (m *Metrics) Gauge(name, help string, labels ...string) *Gauge
func (m *Metrics) Histogram(name, help string, buckets []float64, labels ...string) *Histogram
```

`MaxSeries` (a thousand by default) caps the label combinations per metric; the overflow
falls into one series with every label set to `other` and a single warning in the log.

| Type | Methods |
|---|---|
| `*Counter` | `Inc()`, `Add(v)`, `With(values...)` |
| `*Gauge` | `Set(v)`, `Add(v)`, `Inc()`, `Dec()`, `With(values...)` |
| `*Histogram` | `Observe(v)`, `With(values...)` |

An invalid name (outside `[a-zA-Z_:][a-zA-Z0-9_:]*`) or the wrong number of label values
causes a `panic`: it is a programming error, shows up on the first run and does not corrupt
the output. Calling `Counter` twice with the same name returns the same series.

`Histogram` with nil `buckets` uses the defaults, in seconds: 0.001 0.005 0.01 0.025 0.05
0.1 0.25 0.5 1 2.5 5 10.

## Framework metrics

| Metric | Type | Labels |
|---|---|---|
| `trilha_requests_total` | counter | `method`, `route`, `status` |
| `trilha_request_duration_seconds` | histogram | `method`, `route` |
| `trilha_requests_in_flight` | gauge | — |
| `trilha_security_events_total` | counter | `kind` (`csrf`, `auth`, `body`, `rate`, `panic`) |
| `trilha_panics_total` | counter | — |
| `go_goroutines`, `go_memstats_alloc_bytes`, `go_memstats_sys_bytes` | gauges | — |
| `go_gc_cycles_total` | counter | — |
| `trilha_uptime_seconds` | gauge | — |
| `trilha_build_info` | gauge (always 1) | `version`, `go_version` |

`route` is the registered pattern (`/blog/{slug}`). Static files, 404 and anything outside
the router come in as `other`.

## Correlation

```go
func (c *Ctx) RequestID() string  // the client's X-Request-ID, or generated
func (c *Ctx) TraceID() string    // W3C traceparent; "" when absent or malformed
func (c *Ctx) Log() *slog.Logger  // logger with request_id and trace_id
```

A malformed `traceparent` is silently dropped: a value chosen by a third party does not enter
the log as if it were a legitimate trace.

## What the audit checks

`trilha audit` adds these items: token too short (critical), metrics configured without a
token or a trusted network (critical), `0.0.0.0/0` in `Trusted` (warning) and no `a.Check(`
anywhere in the project (warning).
