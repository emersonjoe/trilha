// Package otel exports Trilha's request traces to OpenTelemetry.
//
// It is a separate Go module on purpose. The framework depends on the standard
// library alone (constitution principle II), and an OTLP exporter brings
// dozens of packages with it; keeping them here means an application that does
// not want tracing does not compile a line of them. The only thing the core
// offers is a generic seam, trilha.Config.OnRequest, which speaks of nothing
// but *trilha.Ctx and a status code.
//
//	import trilhaotel "github.com/emersonjoe/trilha/otel"
//
//	func Setup(a *trilha.App) error {
//		shutdown, err := trilhaotel.Install(a, trilhaotel.Options{
//			Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
//			Service:  "blog",
//			Sample:   0.1,
//		})
//		if err != nil {
//			return err
//		}
//		a.OnShutdown(func(*trilha.App) error {
//			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//			defer cancel()
//			return shutdown(ctx)
//		})
//		return nil
//	}
//
// What a span carries is the aggregatable shape of the request — the route
// template, the method, the status, the request id and the tenant id — and
// never the concrete path, the query string, a header or a body: a trace
// leaves the process, and what leaves the process is chosen, not swept up
// (OWASP ASVS V7.1.1).
package otel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersonjoe/trilha"
	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// scopeName is the instrumentation scope every span of this module is born
// under, so a collector can tell Trilha's spans from the application's own.
const scopeName = "github.com/emersonjoe/trilha/otel"

// Options configures Install.
type Options struct {
	// Endpoint is the OTLP/HTTP address of the collector, with or without a
	// scheme: "http://localhost:4318", "localhost:4318", "tempo:4318/v1/traces".
	// Empty falls back to the SDK's default (localhost:4318) and to the
	// standard OTEL_EXPORTER_OTLP_* environment variables.
	Endpoint string
	// Service names the process in the trace ("blog", "api", "worker"). It is
	// what a Grafana or a Tempo groups by, so it should be the same string on
	// every replica and different on every service. Empty means
	// "unknown_service".
	Service string
	// Version is the build of the service ("1.4.0", a commit). Optional, and
	// the first thing to look at when a latency changes shape after a deploy.
	Version string
	// Sample is the fraction of traces kept, from 0 to 1, for the requests
	// that arrive with no decision already made; a caller that sampled its
	// trace is always followed, so a request never becomes a hole in the
	// middle of someone else's trace. Zero means 1 — every trace — because a
	// zero that silently dropped everything would be a trap.
	Sample float64
	// Insecure sends plain HTTP to the collector. It is for a collector on
	// localhost or on the same private network; over anything else the trace
	// carries route names and tenant ids in the clear (ASVS V9.1.1).
	Insecure bool
	// Headers are sent on every export, which is how a hosted collector
	// authenticates the sender ("authorization", "x-api-key"...). They are a
	// secret: read them from the environment, never from a literal.
	Headers map[string]string
	// Exporter replaces the OTLP exporter, which is what a test does
	// (tracetest.NewInMemoryExporter) and what an application already holding
	// a configured exporter does. When it is set, Endpoint, Insecure and
	// Headers are not read.
	Exporter sdktrace.SpanExporter
}

// Install makes the app export one span per request to the collector.
//
// It builds a TracerProvider (OTLP/HTTP exporter, parent-based sampling,
// a resource naming the service), publishes it and the W3C propagator as the
// global ones — so an application that starts spans of its own with
// otel.Tracer(...) lands in the same trace — and sets
// trilha.Config.OnRequest to the hook that opens the server span.
//
// The returned function flushes what is pending and closes the exporter; call
// it on shutdown, or the last seconds of traces are the ones that never
// arrive. Install replaces the app's OnRequest hook, running whatever hook was
// already there inside the span.
func Install(a *trilha.App, o Options) (shutdown func(context.Context) error, err error) {
	if a == nil {
		return nil, errors.New("otel: nil app")
	}
	exp := o.Exporter
	if exp == nil {
		if exp, err = newExporter(o); err != nil {
			return nil, err
		}
	}
	sample := o.Sample
	if sample == 0 {
		sample = 1
	}
	if sample < 0 {
		sample = 0
	}
	if sample > 1 {
		sample = 1
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(o.Service),
		semconv.ServiceVersion(o.Version),
	))
	if err != nil {
		// A schema clash between the default resource and ours is not worth a
		// dead application: the trace is better with the plain resource than
		// absent.
		res = resource.Default()
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		// Parent-based: the caller's decision wins, and the ratio only
		// decides for the requests that start here.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sample))),
	)
	otelapi.SetTracerProvider(tp)
	otelapi.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	a.Config().OnRequest = hook(tp.Tracer(scopeName), a.Config().OnRequest)
	return tp.Shutdown, nil
}

// newExporter builds the OTLP/HTTP exporter. The connection itself is lazy:
// a collector that is down at boot is not a boot failure, it is a retry.
func newExporter(o Options) (sdktrace.SpanExporter, error) {
	var opts []otlptracehttp.Option
	if o.Endpoint != "" {
		ep := o.Endpoint
		if !strings.Contains(ep, "://") {
			scheme := "https://"
			if o.Insecure {
				scheme = "http://"
			}
			ep = scheme + ep
		}
		u, err := url.Parse(ep)
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("otel: invalid endpoint %q", o.Endpoint)
		}
		opts = append(opts, otlptracehttp.WithEndpointURL(ep))
		if u.Scheme == "http" || o.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
	} else if o.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(o.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(o.Headers))
	}
	return otlptracehttp.New(context.Background(), opts...)
}

// hook is the trilha.RequestHook that opens the server span. It is written
// against nothing but the core seam: the pattern, the status and the actor are
// all the framework tells it.
func hook(tracer trace.Tracer, prev trilha.RequestHook) trilha.RequestHook {
	return func(c *trilha.Ctx, next func() int) {
		r := c.Request()
		// The parent, when the caller sent one. A malformed traceparent is
		// dropped by the propagator and the span simply starts a new trace.
		ctx := otelapi.GetTextMapPropagator().Extract(c.Context(), propagation.HeaderCarrier(r.Header))
		// The name is the route template — "/blog/{slug}", never
		// "/blog/my-post": a span name is a group, and a name per URL is a
		// collector that cannot aggregate anything (and a user id in a
		// dashboard). The method alone stands for what has no template.
		name := c.Pattern()
		if name == "" {
			name = r.Method
		}
		ctx, span := tracer.Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", c.Pattern()),
				attribute.String("url.scheme", scheme(r)),
				attribute.String("trilha.request_id", c.RequestID()),
			))
		defer span.End()
		// From here the span is in the request context, which is the context
		// Upstream forwards with and the one the ai client is called with: the
		// next hop of the trace needs no further wiring.
		c.SetContext(ctx)

		status := 0
		run := func() int { status = next(); return status }
		if prev != nil {
			prev(c, run)
		} else {
			run()
		}

		span.SetAttributes(attribute.Int("http.response.status_code", status))
		// The tenant is read after the answer because authentication runs
		// inside the chain. The id only — a trace is not the place for a name
		// or an e-mail (ASVS V8.3.1).
		if tenant := c.Actor().Tenant; tenant != "" {
			span.SetAttributes(attribute.String("trilha.tenant", tenant))
		}
		// Only the server's own failures are errors of the span: a 404 or a
		// 403 is a correct answer to a wrong request, and marking it red is
		// how an error rate stops meaning anything.
		if status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(status))
		}
	}
}

// scheme is what the request came in as. Only the connection is trusted here:
// X-Forwarded-Proto is the proxy's word, and the span does not need it enough
// to start believing headers.
func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// Transport injects the current trace into every request that leaves, so the
// service on the other side continues the trace instead of starting one. It
// wraps rt, or http.DefaultTransport when rt is nil:
//
//	client := ai.Client{HTTPClient: &http.Client{Transport: Transport(nil)}}
//	// ...and the handler calls it with the request context:
//	res, err := client.Chat(c.Context(), req)
//
// What it injects is the span of the request context, so the call has to be
// made with c.Context() — which is what the ai client, task.Do and any
// http.NewRequestWithContext(c.Context(), ...) carry.
//
// A prefix proxied by trilha.Upstream needs nothing here: it is answered
// before any route, so it opens no span of its own, and the reverse proxy
// forwards the traceparent the caller sent untouched — the trace continues on
// the other side either way.
func Transport(rt http.RoundTripper) http.RoundTripper { return roundTripper{base: rt} }

type roundTripper struct{ base http.RoundTripper }

func (t roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	// RoundTrip must not touch the request it is given.
	out := r.Clone(r.Context())
	otelapi.GetTextMapPropagator().Inject(out.Context(), propagation.HeaderCarrier(out.Header))
	return base.RoundTrip(out)
}
