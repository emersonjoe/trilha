package trilha

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

const obsTok = "0123456789abcdef0123456789abcdef"

func authed() map[string]string { return map[string]string{"Authorization": "Bearer " + obsTok} }

func scrape(t *testing.T, a *App, hdr map[string]string) (int, string) {
	t.Helper()
	rec := get(t, a, "GET", "/_trilha/metrics", "", hdr)
	return rec.Code, rec.Body.String()
}

// US3: off unless configured; on, it speaks the Prometheus text format.
func TestMetricsOffByDefault(t *testing.T) {
	a := obsApp(t, Prod, Observability{})
	if code, _ := scrape(t, a, nil); code != 404 {
		t.Fatalf("metrics must not exist unless configured: %d", code)
	}
	if a.Metrics() == nil {
		t.Fatal("the registry exists even when not exposed")
	}
}

func TestMetricsExposition(t *testing.T) {
	a := obsApp(t, Prod, Observability{Metrics: "/_trilha/metrics", Token: obsTok})
	get(t, a, "GET", "/", "", nil)
	get(t, a, "GET", "/nao-existe", "", nil)

	if code, _ := scrape(t, a, nil); code != 401 {
		t.Fatalf("anonymous scrape: %d", code)
	}
	code, body := scrape(t, a, authed())
	if code != 200 {
		t.Fatal(code)
	}
	for _, want := range []string{
		"# HELP trilha_requests_total",
		"# TYPE trilha_requests_total counter",
		`trilha_requests_total{method="GET",route="/",status="200"} 1`,
		"# TYPE trilha_request_duration_seconds histogram",
		"trilha_request_duration_seconds_bucket{",
		"trilha_request_duration_seconds_count{",
		"trilha_request_duration_seconds_sum{",
		"trilha_requests_in_flight 0",
		"trilha_build_info{",
		"go_goroutines ",
		"trilha_uptime_seconds ",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	// FR-003: the label is the route pattern, never the concrete path.
	if strings.Contains(body, "/nao-existe") {
		t.Error("raw path leaked into a label")
	}
	if !strings.Contains(body, `route="other"`) {
		t.Error("unmatched requests should fall into the other bucket")
	}
	if ct := metricsContentType; !strings.HasPrefix(ct, "text/plain") {
		t.Fatal(ct)
	}
}

func TestApplicationMetrics(t *testing.T) {
	a := obsApp(t, Prod, Observability{Metrics: "/_trilha/metrics", Token: obsTok})
	m := a.Metrics()
	m.Counter("blog_posts_total", "posts criados").Inc()
	m.Counter("blog_hits_total", "acessos", "secao").With("home").Add(3)
	m.Gauge("blog_queue_depth", "fila").Set(7)
	h := m.Histogram("blog_job_seconds", "duração", []float64{0.1, 1}, "tipo")
	h.With("indexar").Observe(0.5)
	_, body := scrape(t, a, authed())
	for _, want := range []string{
		"blog_posts_total 1",
		`blog_hits_total{secao="home"} 3`,
		"blog_queue_depth 7",
		`blog_job_seconds_bucket{tipo="indexar",le="0.1"} 0`,
		`blog_job_seconds_bucket{tipo="indexar",le="1"} 1`,
		`blog_job_seconds_bucket{tipo="indexar",le="+Inf"} 1`,
		`blog_job_seconds_count{tipo="indexar"} 1`,
		`blog_job_seconds_sum{tipo="indexar"} 0.5`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	// Same name twice returns the same series instead of duplicating output.
	m.Counter("blog_posts_total", "posts criados").Inc()
	if _, body := scrape(t, a, authed()); strings.Count(body, "# TYPE blog_posts_total") != 1 || !strings.Contains(body, "blog_posts_total 2") {
		t.Fatal(body)
	}
}

func TestMetricNamesAndLabelsAreValidated(t *testing.T) {
	m := New(Config{Logger: quiet()}).Metrics()
	for _, bad := range []string{"", "com-traço", "1comeca_com_numero", "espaço aqui"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("invalid name accepted: %q", bad)
				}
			}()
			m.Counter(bad, "x")
		}()
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("wrong number of label values must be caught")
			}
		}()
		m.Counter("ok_total", "x", "a", "b").With("só-um")
	}()
	// Escaping keeps the exposition parseable.
	m.Counter("escapa_total", `ajuda com "aspas"`+"\n e quebra", "l").With(`va"l\or` + "\n").Inc()
	var b strings.Builder
	m.write(&b)
	out := b.String()
	if !strings.Contains(out, `# HELP escapa_total ajuda com "aspas"\n e quebra`) {
		t.Error(out)
	}
	if !strings.Contains(out, `escapa_total{l="va\"l\\or\n"} 1`) {
		t.Error(out)
	}
}

// FR-003: a metric cannot grow without bound.
func TestCardinalityCap(t *testing.T) {
	a := New(Config{Logger: quiet()})
	m := a.Metrics()
	m.MaxSeries = 10
	c := m.Counter("teste_total", "x", "id")
	for i := 0; i < 50; i++ {
		c.With("v" + strconv.Itoa(i)).Inc()
	}
	var b strings.Builder
	m.write(&b)
	got := len(regexp.MustCompile(`(?m)^teste_total\{`).FindAllString(b.String(), -1))
	if got != 11 {
		t.Fatalf("expected 10 series plus the overflow bucket, got %d", got)
	}
	if !strings.Contains(b.String(), `teste_total{id="other"} 40`) {
		t.Fatal(b.String())
	}
}

func TestSecurityEventAndPanicAreCounted(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quiet(), Observability: Observability{Metrics: "/_trilha/metrics", Token: obsTok}})
	a.Register(Route{Pattern: "/boom", Page: func(c *Ctx) (h.Node, error) { panic("x") }})
	a.Register(Route{Pattern: "/form", Page: func(c *Ctx) (h.Node, error) { return nil, nil },
		Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error { return nil }}})
	get(t, a, "GET", "/boom", "", nil)
	get(t, a, "POST", "/form", "", map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	_, body := scrape(t, a, authed())
	if !strings.Contains(body, "trilha_panics_total 1") {
		t.Error(body)
	}
	if !strings.Contains(body, `trilha_security_events_total{kind="csrf"} 1`) {
		t.Error(body)
	}
}
