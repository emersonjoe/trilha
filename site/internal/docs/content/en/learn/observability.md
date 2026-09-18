---
title: Health and observability
description: Liveness and readiness probes, metrics in the Prometheus format and log correlation, taking care not to turn monitoring into a leak.
---

An app in production has to answer three questions for whoever operates it: *is it up?*,
*can it take traffic?* and *what is going on?*. Trilha answers all three with no dependency,
and answers in a way that does not hand the map of your infrastructure to anyone passing
by.

The reference here is twofold, as in the security chapter: **NIST SP 800-53r5** (AU-2 and
AU-3 for the content of the record, AU-9 to protect that information, SI-4 for monitoring
and SC-5 against denial of service) and **OWASP** (Top 10 2021 A09, API Security 2023 API8
and chapter V7 of the ASVS).

## The two probes

Without configuring anything, every Trilha app already answers:

| Address | Question | Runs checks? |
|---|---|---|
| `/_trilha/health/live` | can the process serve? | no |
| `/_trilha/health/ready` | can it take traffic? | yes |
| `/_trilha/health` | same as `ready` | yes |

The split is not bureaucracy. In Kubernetes, a failing *readiness* takes the pod out of the
load balancer; a failing *liveness* **kills the process**. If both ran the same database
check, a network blip would restart the whole fleet instead of waiting for the database to
come back. That is why `live` never touches a dependency.

```go
// app/setup.go
func Setup(a *trilha.App) error {
	a.Check("db", func(ctx context.Context) error {
		return db.PingContext(ctx)
	})
	a.Check("queue", func(ctx context.Context) error {
		return queue.Ping(ctx)
	})
	return nil
}
```

Each check runs with a deadline (2 s by default) and in parallel; a `panic` inside it
becomes a failure and does not bring the process down. The result is cached for 1 s: one
probe per second — or ten thousand per second, coming from someone with bad intentions —
does not become ten thousand `SELECT 1` on your database.

## What an anonymous caller sees

In production, without authorization, the response is exactly this:

```json
{"status":"fail"}
```

Check name, error message, hostname and version are left out on purpose (ASVS V7.4.1).
Knowing there is a Postgres called `finance` and that it is down is half the way for someone
probing the target. The cause goes to the log, where access control already exists, and to
whoever authenticates:

```
curl -H "Authorization: Bearer $TRILHA_OBS_TOKEN" https://app/_trilha/health
```

```json
{"status":"fail","checks":[{"name":"db","status":"fail","duration_ms":2001.4,
 "error":"deadline exceeded: context deadline exceeded"}],"uptime_seconds":8134.2}
```

In `dev` the details are open — there, the target is you.

## Metrics

The metrics endpoint **does not exist** until you ask for it. That is deliberate: a public
`/metrics` is the misconfiguration described in OWASP's API8, and it tells the visitor how
many routes you have, which ones error and at what time your traffic drops.

```go
func Config(cfg *trilha.Config) {
	cfg.Observability.Metrics = "/_trilha/metrics"   // or TRILHA_METRICS
	// TRILHA_OBS_TOKEN (32+ bytes) authorizes the scrape;
	// alternative: Trusted with the collector's CIDR.
	cfg.Observability.Trusted = []string{"10.42.0.0/16"}
}
```

The output is the Prometheus text format, so Prometheus, VictoriaMetrics, Grafana Alloy and
the OpenTelemetry Collector read it without a translator:

```
trilha_requests_total{method="GET",route="/blog/{slug}",status="200"} 1841
trilha_request_duration_seconds_bucket{method="GET",route="/blog/{slug}",le="0.05"} 1802
trilha_requests_in_flight 3
trilha_security_events_total{kind="csrf"} 2
trilha_panics_total 0
go_goroutines 14
```

Look at the `route` label: it is the **registered pattern**, `/blog/{slug}`, never the
concrete path `/blog/how-i-did-x`. A concrete path is user input — it carries identifiers,
sometimes a token in the query string, and makes the number of series grow without bound
until memory runs out. Whatever does not match a registered route (static files, 404) falls
into a single `other` label, and every metric has a series cap (a thousand by default).

Your own metrics go in the same place:

```go
posts.Published = a.Metrics().Counter("blog_posts_total", "Published posts.")
slow := a.Metrics().Histogram("blog_render_seconds", "Render time.", nil, "template")
slow.With("post").Observe(dur.Seconds())
```

## Finding a request in the log

Every request log already carries `request_id`, and the same value comes back in the
`X-Request-ID` header. When the client sends `traceparent` (W3C Trace Context — what a
gateway, an Istio or an OpenTelemetry SDK sends), `trace_id` comes along:

```go
func GET(c *trilha.Ctx) error {
	c.Log().Info("querying supplier", "tax_id", taxID)  // request_id + trace_id
	return c.JSON(200, resp)
}
```

Trilha **propagates** the context and puts it in the log; it does not export spans or sample
traces. Full distributed tracing is a collector's job, and bolting it onto the core would
cost dozens of dependencies.

What never goes in the log, by design: request body, cookies, the `Authorization` header and
the query string (ASVS V7.1.1). That is where secrets travel.

## Traces to OpenTelemetry

Propagating is not exporting. The paragraph above says the framework carries `traceparent`
and writes `trace_id`; to actually *watch* one upload cross web → API → worker → an AI
provider in a Grafana or a Tempo, the spans have to leave the process. That is an optional
module with its own `go.mod`, `github.com/emersonjoe/trilha/otel`, so the framework itself
keeps depending on the standard library alone:

```go
import (
	"context"
	"os"
	"time"

	"github.com/emersonjoe/trilha"
	trilhaotel "github.com/emersonjoe/trilha/otel"
)

func Setup(a *trilha.App) error {
	shutdown, err := trilhaotel.Install(a, trilhaotel.Options{
		Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), // "http://collector:4318"
		Service:  "blog",
		Version:  os.Getenv("BUILD_VERSION"),
		Sample:   0.1, // one trace in ten of the ones that start here
	})
	if err != nil {
		return err
	}
	// Without this the last seconds of traces never arrive: the exporter
	// batches, and a process that exits takes the batch with it.
	a.OnShutdown(func(*trilha.App) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return shutdown(ctx)
	})
	return nil
}
```

`trilhaotel.Install` reads the `traceparent` that already arrived, opens one server span per
request and exports over OTLP/HTTP. What the span carries is the aggregatable shape of the
request and nothing else:

| Attribute | Value |
|---|---|
| span name, `http.route` | the route template — `/blog/{slug}`, never `/blog/my-post` |
| `http.request.method`, `url.scheme` | `GET`, `https` |
| `http.response.status_code` | the status that was written, 500 included |
| `trilha.request_id` | the same id as `X-Request-ID` and the access log |
| `trilha.tenant` | `Actor().Tenant`, the organisation id — never a name or an e-mail |

The concrete path, the query string, headers and bodies are not there, and are not an option:
a trace leaves the process, and what leaves the process is chosen (ASVS V7.1.1). `Sample` is
parent-based — a caller that already sampled its trace is always followed, so this service
never becomes a hole in the middle of somebody else's trace.

### The next hop

The span travels in the request context, which is the context the `ai` client is called with
and the one your own calls should use. `otel.Transport` puts it on the wire:

```go
client := ai.Client{HTTPClient: &http.Client{Transport: trilhaotel.Transport(nil)}}
resp, err := client.Chat(c.Context(), req)

req, _ := http.NewRequestWithContext(c.Context(), "GET", supplierURL, nil)
res, err := (&http.Client{Transport: trilhaotel.Transport(nil)}).Do(req)
```

A prefix proxied by `Upstreams` needs nothing: it is answered before any route, so it opens no
span of its own, and the reverse proxy forwards the `traceparent` it received untouched — the
API on the other side continues the same trace. The same goes for everything else the fallback
answers, a static file or a 404 on an unknown path: there is no route template to name a span
with, and a span per URL is a collector that cannot aggregate anything.

### What the core knows about all this

Nothing, on purpose. The module hooks into one generic seam, `Config.OnRequest`, whose
signature speaks only of `*trilha.Ctx` and a status code; the framework never imports an
exporter or an SDK. Writing your own hook — a budget, a profile, a header — is the same two
lines: see [`RequestHook`](/reference/observability).

The example lives at `otel/example` inside that module and not under `examples/`, because
everything under `examples/` shares the root module and would drag the SDK into it. Its tests
run with `make test-otel`; `make test` of the framework does not see them.

## Cost

With the metrics endpoint off, the instrumentation does not run: it is a pointer comparison.
On, it costs **zero allocations** per request (two map lookups with a key built on the stack
and a few atomic increments); the time difference stays within the noise of the reference
machine. The numbers are in [Performance](/reference/performance).

## Challenge

Make your app's `/_trilha/health/ready` check the database **and** an external service, with
a 500 ms deadline for the external one; expose the metrics only to the `10.0.0.0/8` network;
and count, in a metric of your own, how many times the external service failed.

:::solution
```go
// app/setup.go
func Config(cfg *trilha.Config) {
	cfg.Observability.Metrics = "/_trilha/metrics"
	cfg.Observability.Trusted = []string{"10.0.0.0/8"}
}

func Setup(a *trilha.App) error {
	failures := a.Metrics().Counter("integration_failures_total", "Failed calls to the partner.", "service")

	a.Check("db", func(ctx context.Context) error { return db.PingContext(ctx) })

	a.Check("partner", func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := partner.Ping(ctx); err != nil {
			failures.With("partner").Inc()
			return err
		}
		return nil
	})
	return nil
}
```

The partner's short deadline coexists with the general one (`Observability.Timeout`):
whichever expires first wins. And because the counter is created in `Setup`, it shows up in
the scrape from the first request, with value zero — which beats vanishing from the
dashboard until the first failure.
:::
