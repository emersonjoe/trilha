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
	Status        string        // trilha.StatusPass | trilha.StatusFail
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
startup gate). The endpoint decides what to reveal. The two values are the constants
`trilha.StatusPass` (`"pass"`) and `trilha.StatusFail` (`"fail"`), so a status page compares
against the framework's name instead of a string it typed.

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

## The audit trail

Every internal application ends up needing "who did what": who deleted the document, who
changed the permission, who exported the list. The framework already holds half of it — the
request id, the client IP behind `TrustedProxies`, the session, the route pattern. What is
missing is the sentence, and a place for it.

```go
func DELETE(c *trilha.Ctx) error {
	if err := docs.Delete(c, c.Param("id")); err != nil {
		return err
	}
	c.Audit("document.deleted", c.Param("id"))
	return c.Redirect("/documents")
}

c.Audit("permission.changed", role, trilha.Fields{"module": "docs", "from": "view", "to": "edit"})
```

The record that comes out, with nothing else written by the application:

```json
{"at":"2026-09-08T15:04:05Z","action":"document.deleted","target":"42",
 "actor":{"subject":"u_17","email":"ana@org.br","via":"session"},
 "ip":"10.0.0.7","request_id":"…","route":"/documents/{id}"}
```

That is the whole point of it being one line: the actor, the address, the request id and the
route are already known, and writing them by hand is what every application does and what
every application forgets in half its handlers.

### Where it goes

`Config.Audit` is a `trilha.AuditSink`: one method, `Write(AuditRecord) error`, because the
decision an application actually makes is *which table*, not *which shape*. An error coming back
is logged and the request carries on — the document was deleted either way, and refusing to
answer would lose the trail *and* confuse the person.

```go
cfg.Audit = trilha.AuditFunc(func(r trilha.AuditRecord) error {
	_, err := db.Exec(`INSERT INTO audit_log (at, action, target, actor, ip) VALUES (?,?,?,?,?)`,
		r.At, r.Action, r.Target, r.Actor.Subject, r.IP)
	return err
})
```

Leave it nil and the record goes to the app's logger with `kind=audit`, which is enough to
grep and enough to ship a first version with.

**A sink that fails does not take the response with it.** The error is logged and the request
carries on. That is deliberate: the document was deleted either way, and refusing to answer now
would lose the trail *and* confuse the person who did it — two failures instead of one.

### Who is acting

`auth` sets it: any route behind `Require`, `RequireRole` or `RequirePolicy` attributes the
trail without the application writing a line, with `via: "session"`.

An application that authenticates its own way calls `c.SetActor` once in its middleware and
everything below is attributed:

```go
c.SetActor(trilha.Actor{Subject: key.ID, Name: key.Label, Via: "api_key"})
```

Nobody recognised is recorded as `anonymous`, and it is recorded — an audit trail that silently
drops the anonymous action has a hole exactly where somebody would look. `trilha audit` warns
when `c.Audit` is called in a project where no route requires a session.

### The screen

`ui.AuditTable` is the screen every application with `c.Audit` ends up writing by hand: who did
what, to what, when and from where.

```go
regs, total := auditoria.Buscar(q)
return ui.AuditTable(c, regs, ui.AuditOpts{
	Params:  q.ListParams,
	Total:   total,
	Actions: auditoria.Acoes(),
	Action:  q.Acao,
	Export:  "/auditoria/csv",
}), nil
```

It is a `ui.DataTable` underneath, and that is the point: the filter form, the ordering links,
the pagination and the fragment swap are the ones every other listing already has. A trail that
behaved differently from the rest of the app would be a second thing to learn.

**Reading the trail is the application's job.** `Config.Audit` is a write interface with one
method and stays that way — the framework has no database, and the query behind this screen (a
period, an actor, a table this app chose) is not something it could write. The example's is
thirty lines over a slice.

`Fields` is a detail and not a column: each action carries its own keys, so a column per key is
a table that grows a column every time somebody audits something new.

`Export` points at a route answering with `c.CSV`, and the button carries the query that is on
screen — an export that ignores the filter in front of somebody is an export of the wrong thing,
and they only find out in the spreadsheet.

:::warning
The trail is the list of everybody's actions, so **reading it is an administrative act**. Put
the screen behind the same guard as the rest of the administration, and put the export
**inside** the guarded folder — a download is a different response, not a different permission.
In [`examples/local-login`](https://github.com/emersonjoe/trilha/tree/main/examples/local-login/app/auditoria)
`/auditoria/csv` inherits the folder's middleware without saying a word about it; outside the
folder it would have been the one address handing the whole trail to anybody.
:::

### `Route` is the pattern

`/documents/{id}`, not `/documents/42`. The concrete id is already in `Target`; the pattern is
what lets a query group a thousand deletions into one row.

