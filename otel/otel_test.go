package otel

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	otelapi "go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

// A parent this test always sends, so that the span it gets back can be
// recognised as the continuation of somebody else's trace.
const (
	parentTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
	parentSpan  = "00f067aa0ba902b7"
	traceparent = "00-" + parentTrace + "-" + parentSpan + "-01"
)

// newApp builds an app with one route per case, tracing installed against an
// in-memory exporter, and returns the exporter and the flush.
func newApp(t *testing.T, register func(a *trilha.App)) (*trilha.App, *tracetest.InMemoryExporter, func()) {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	a := trilha.New(trilha.Config{
		Env:    trilha.Prod,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	register(a)
	shutdown, err := Install(a, Options{Service: "blog", Version: "1.2.3", Exporter: exp})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	// Install publishes its provider as the global one, which is how an
	// application starts spans of its own inside the same trace; the flush is
	// what hands the batched spans to the exporter inside a test.
	tp, ok := otelapi.GetTracerProvider().(*sdktrace.TracerProvider)
	if !ok {
		t.Fatalf("Install must publish its TracerProvider globally, got %T", otelapi.GetTracerProvider())
	}
	return a, exp, func() {
		if err := tp.ForceFlush(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func do(t *testing.T, a *trilha.App, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

// attrs flattens one span's attributes.
func attrs(s tracetest.SpanStub) map[string]string {
	out := map[string]string{}
	for _, kv := range s.Attributes {
		out[string(kv.Key)] = kv.Value.Emit()
	}
	return out
}

// One request, one server span: the parent from the incoming traceparent, the
// route template (never the concrete path), the status and the tenant.
func TestSpanCarriesRouteStatusAndTenant(t *testing.T) {
	a, exp, flush := newApp(t, func(a *trilha.App) {
		a.Register(trilha.Route{Pattern: "/blog/{slug}", Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				c.SetActor(trilha.Actor{Subject: "u1", Email: "ana@example.com", Tenant: "acme"})
				return c.Text(200, "ok")
			},
		}})
	})
	if rec := do(t, a, "GET", "/blog/segredo-do-cliente", map[string]string{"traceparent": traceparent}); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	flush()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want one span, got %d", len(spans))
	}
	s := spans[0]
	if s.SpanContext.TraceID().String() != parentTrace {
		t.Fatalf("the span must continue the caller's trace: %s", s.SpanContext.TraceID())
	}
	if s.Parent.SpanID().String() != parentSpan {
		t.Fatalf("parent span: %s", s.Parent.SpanID())
	}
	if s.Name != "/blog/{slug}" {
		t.Fatalf("the span is named by the route template: %q", s.Name)
	}
	got := attrs(s)
	for k, want := range map[string]string{
		"http.request.method":       "GET",
		"http.route":                "/blog/{slug}",
		"url.scheme":                "http",
		"http.response.status_code": "200",
		"trilha.tenant":             "acme",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q (%v)", k, got[k], want, got)
		}
	}
	if got["trilha.request_id"] == "" {
		t.Error("the span must carry the request id, so a log line and a trace meet")
	}
	// The concrete path and the user are not in the trace: a span leaves the
	// process and takes only what can be aggregated.
	for k, v := range got {
		if strings.Contains(v, "segredo-do-cliente") || strings.Contains(v, "u1") || strings.Contains(v, "ana@example.com") {
			t.Errorf("%s leaked the concrete path or the user: %q", k, v)
		}
	}
	if s.Status.Code.String() == "Error" {
		t.Errorf("a 200 is not an error: %v", s.Status)
	}
	// The resource names the service, which is what a collector groups by.
	var service string
	for _, kv := range s.Resource.Attributes() {
		if kv.Key == "service.name" {
			service = kv.Value.Emit()
		}
	}
	if service != "blog" {
		t.Errorf("service.name = %q", service)
	}
}

// A 404 answered by a route keeps the template and is not marked as an error:
// a wrong request is not a failing server.
func TestNotFoundKeepsTheTemplateAndIsNotAnError(t *testing.T) {
	a, exp, flush := newApp(t, func(a *trilha.App) {
		a.Register(trilha.Route{Pattern: "/blog/{slug}", Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error { return trilha.ErrNotFound },
		}})
	})
	if rec := do(t, a, "GET", "/blog/nope", nil); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	flush()
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want one span, got %d", len(spans))
	}
	got := attrs(spans[0])
	if got["http.response.status_code"] != "404" || got["http.route"] != "/blog/{slug}" {
		t.Fatalf("%v", got)
	}
	if spans[0].Status.Code.String() == "Error" {
		t.Errorf("404 must not be an error span: %v", spans[0].Status)
	}
	if spans[0].SpanContext.TraceID().String() == parentTrace {
		t.Error("with no traceparent the span starts its own trace")
	}
}

// A panic is the server's own failure: 500 and an error span.
func TestServerErrorMarksTheSpan(t *testing.T) {
	a, exp, flush := newApp(t, func(a *trilha.App) {
		a.Register(trilha.Route{Pattern: "/boom", Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error { panic("no") },
		}})
	})
	if rec := do(t, a, "GET", "/boom", nil); rec.Code != 500 {
		t.Fatal(rec.Code)
	}
	flush()
	spans := exp.GetSpans()
	if len(spans) != 1 || attrs(spans[0])["http.response.status_code"] != "500" {
		t.Fatalf("%d spans: %v", len(spans), spans)
	}
	if spans[0].Status.Code.String() != "Error" {
		t.Fatalf("status = %v", spans[0].Status)
	}
}

// What leaves the handler continues the trace: an outbound call made with
// c.Context() through Transport carries a traceparent whose parent is the
// server span — the hop web → API in the same trace.
func TestTransportPropagatesTheServerSpan(t *testing.T) {
	var got string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("traceparent")
	}))
	defer api.Close()

	a, exp, flush := newApp(t, func(a *trilha.App) {
		a.Register(trilha.Route{Pattern: "/call", Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				req, err := http.NewRequestWithContext(c.Context(), "GET", api.URL, nil)
				if err != nil {
					return err
				}
				res, err := (&http.Client{Transport: Transport(nil)}).Do(req)
				if err != nil {
					return err
				}
				defer res.Body.Close()
				return c.Text(200, "ok")
			},
		}})
	})
	if rec := do(t, a, "GET", "/call", map[string]string{"traceparent": traceparent}); rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	flush()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want one span, got %d", len(spans))
	}
	server := spans[0].SpanContext
	want := "00-" + server.TraceID().String() + "-" + server.SpanID().String() + "-01"
	if got != want {
		t.Fatalf("outbound traceparent = %q, want %q", got, want)
	}
}

// The default exporter really is OTLP/HTTP: a collector at the Endpoint
// receives the span over the wire, protobuf and all.
func TestOTLPExportReachesTheCollector(t *testing.T) {
	type received struct {
		name  string
		attrs map[string]string
	}
	spans := make(chan received, 4)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			t.Errorf("OTLP/HTTP posts to /v1/traces, got %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var req coltracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &req); err != nil {
			t.Errorf("the body is not an OTLP export request: %v", err)
			return
		}
		for _, rs := range req.GetResourceSpans() {
			for _, ss := range rs.GetScopeSpans() {
				for _, s := range ss.GetSpans() {
					got := received{name: s.GetName(), attrs: map[string]string{}}
					for _, kv := range s.GetAttributes() {
						got.attrs[kv.GetKey()] = kv.GetValue().GetStringValue()
					}
					spans <- got
				}
			}
		}
		out, _ := proto.Marshal(&coltracepb.ExportTraceServiceResponse{})
		w.Header().Set("Content-Type", "application/x-protobuf")
		_, _ = w.Write(out)
	}))
	defer collector.Close()

	a := trilha.New(trilha.Config{Env: trilha.Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	a.Register(trilha.Route{Pattern: "/ping", Methods: map[string]trilha.HandlerFunc{
		"GET": func(c *trilha.Ctx) error { return c.Text(200, "ok") },
	}})
	shutdown, err := Install(a, Options{Endpoint: collector.URL, Service: "blog", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, a, "GET", "/ping", nil); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-spans:
		if got.name != "/ping" || got.attrs["http.route"] != "/ping" {
			t.Fatalf("%+v", got)
		}
	default:
		t.Fatal("the collector received no span")
	}
}

// Sampling is parent-based: a caller that did not sample its trace is not
// exported, and a caller that did is followed whatever the ratio says — so a
// service never becomes a hole in the middle of somebody else's trace.
func TestSamplingFollowsTheCaller(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	a := trilha.New(trilha.Config{Env: trilha.Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	a.Register(trilha.Route{Pattern: "/x", Methods: map[string]trilha.HandlerFunc{
		"GET": func(c *trilha.Ctx) error { return c.Text(200, "ok") },
	}})
	// A ratio small enough that nothing deciding here would be kept.
	shutdown, err := Install(a, Options{Service: "blog", Sample: 0.0000001, Exporter: exp})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = shutdown(context.Background()) }()
	do(t, a, "GET", "/x", map[string]string{"traceparent": "00-" + parentTrace + "-" + parentSpan + "-00"})
	do(t, a, "GET", "/x", map[string]string{"traceparent": traceparent})
	tp := otelapi.GetTracerProvider().(*sdktrace.TracerProvider)
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}
	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("only the sampled caller is exported, got %d", len(spans))
	}
	if spans[0].SpanContext.TraceID().String() != parentTrace {
		t.Fatal(spans[0].SpanContext.TraceID())
	}
}

// An endpoint that is not an address is a boot-time complaint.
func TestInvalidEndpointIsAnError(t *testing.T) {
	a := trilha.New(trilha.Config{Env: trilha.Prod, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if _, err := Install(a, Options{Endpoint: "://"}); err == nil {
		t.Fatal("want an error")
	}
	if a.Config().OnRequest != nil {
		t.Fatal("a failed Install must not leave a hook behind")
	}
}
