package trilha

import (
	"io"
	"log/slog"
	"math"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// metricsContentType is the Prometheus text exposition format.
const metricsContentType = "text/plain; version=0.0.4; charset=utf-8"

// defaultBuckets are the request-duration buckets, in seconds.
var defaultBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// overflowLabel replaces every label value of the bucket that absorbs series
// beyond Metrics.MaxSeries, so a hostile or buggy caller cannot grow the
// registry without bound (NIST SP 800-53 SC-5).
const overflowLabel = "other"

// Metrics is the process metric registry, exposed in the Prometheus text
// format. Get it with App.Metrics; it exists even when no endpoint serves it.
type Metrics struct {
	// MaxSeries caps the number of label combinations per metric (default
	// 1000). Extra combinations are folded into a single "other" series.
	MaxSeries int

	log    *slog.Logger
	start  time.Time
	mu     sync.RWMutex
	byName map[string]*metric
	order  []*metric
}

func newMetrics(log *slog.Logger) *Metrics {
	return &Metrics{MaxSeries: 1000, log: log, start: time.Now(), byName: map[string]*metric{}}
}

type metricKind int

const (
	counterKind metricKind = iota
	gaugeKind
	histogramKind
)

func (k metricKind) String() string {
	switch k {
	case gaugeKind:
		return "gauge"
	case histogramKind:
		return "histogram"
	}
	return "counter"
}

type metric struct {
	name, help string
	kind       metricKind
	labels     []string
	buckets    []float64
	max        int
	log        *slog.Logger

	mu       sync.RWMutex
	series   map[string]*series
	order    []*series
	overflow *series
	warned   bool
}

// series is one label combination. Float values live in the bits of a uint64
// so they can be updated atomically, without a lock in the hot path.
type series struct {
	values  []string
	bits    atomic.Uint64 // counter/gauge value, or histogram sum
	count   atomic.Uint64 // histogram observations
	buckets []atomic.Uint64
}

func (s *series) add(v float64) {
	for {
		old := s.bits.Load()
		if s.bits.CompareAndSwap(old, math.Float64bits(math.Float64frombits(old)+v)) {
			return
		}
	}
}

func (s *series) set(v float64)  { s.bits.Store(math.Float64bits(v)) }
func (s *series) value() float64 { return math.Float64frombits(s.bits.Load()) }

// Counter only ever grows: requests served, errors, events.
type Counter struct {
	m *metric
	s *series
}

// Gauge goes up and down: queue depth, connections in use.
type Gauge struct {
	m *metric
	s *series
}

// Histogram counts observations per bucket: durations, sizes.
type Histogram struct {
	m *metric
	s *series
}

// Counter returns (creating on first use) a counter. labels declares the
// dimension names; bind the values with With. Invalid names panic: it is a
// programming error, caught on the first run.
func (m *Metrics) Counter(name, help string, labels ...string) *Counter {
	mt := m.metric(name, help, counterKind, labels, nil)
	return &Counter{m: mt, s: mt.base()}
}

// Gauge returns (creating on first use) a gauge.
func (m *Metrics) Gauge(name, help string, labels ...string) *Gauge {
	mt := m.metric(name, help, gaugeKind, labels, nil)
	return &Gauge{m: mt, s: mt.base()}
}

// Histogram returns (creating on first use) a histogram. buckets are the
// upper bounds in ascending order; nil uses the default duration buckets.
func (m *Metrics) Histogram(name, help string, buckets []float64, labels ...string) *Histogram {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}
	b := append([]float64(nil), buckets...)
	sort.Float64s(b)
	mt := m.metric(name, help, histogramKind, labels, b)
	return &Histogram{m: mt, s: mt.base()}
}

func (m *Metrics) metric(name, help string, kind metricKind, labels []string, buckets []float64) *metric {
	if !validMetricName(name) {
		panic("trilha: invalid metric name: " + strconv.Quote(name))
	}
	for _, l := range labels {
		if !validMetricName(l) {
			panic("trilha: invalid label name: " + strconv.Quote(l))
		}
	}
	m.mu.RLock()
	mt, ok := m.byName[name]
	m.mu.RUnlock()
	if ok {
		return mt
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if mt, ok := m.byName[name]; ok {
		return mt
	}
	max := m.MaxSeries
	if max <= 0 {
		max = 1000
	}
	mt = &metric{name: name, help: help, kind: kind, labels: append([]string(nil), labels...),
		buckets: buckets, max: max, log: m.log, series: map[string]*series{}}
	m.byName[name] = mt
	m.order = append(m.order, mt)
	return mt
}

// base is the series of a metric without labels; nil when labels are declared
// (the caller must go through With).
func (m *metric) base() *series {
	if len(m.labels) > 0 {
		return nil
	}
	return m.get(nil)
}

func (m *metric) newSeries(values []string) *series {
	s := &series{values: values}
	if m.kind == histogramKind {
		s.buckets = make([]atomic.Uint64, len(m.buckets))
	}
	return s
}

// get finds or creates the series for these label values, honouring the cap.
// The key is built in a stack buffer and looked up as map[string(bytes)],
// which the compiler resolves without allocating: the read path of a metric
// runs on every request and must not add garbage (spec 012).
func (m *metric) get(values []string) *series {
	var buf [192]byte
	k := buf[:0]
	for i, v := range values {
		if i > 0 {
			k = append(k, 0)
		}
		k = append(k, v...)
	}
	m.mu.RLock()
	s, ok := m.series[string(k)]
	m.mu.RUnlock()
	if ok {
		return s
	}
	key := string(k)
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.series[key]; ok {
		return s
	}
	if len(m.series) >= m.max {
		if m.overflow == nil {
			over := make([]string, len(m.labels))
			for i := range over {
				over[i] = overflowLabel
			}
			m.overflow = m.newSeries(over)
			m.order = append(m.order, m.overflow)
		}
		if !m.warned && m.log != nil {
			m.warned = true
			m.log.Warn("trilha: metric reached the series cap; overflow grouped",
				"metric", m.name, "max", m.max, "label", overflowLabel)
		}
		return m.overflow
	}
	s = m.newSeries(append([]string(nil), values...))
	m.series[key] = s
	m.order = append(m.order, s)
	return s
}

func (m *metric) bind(values []string) *series {
	if len(values) != len(m.labels) {
		panic("trilha: metric " + m.name + " expects " + strconv.Itoa(len(m.labels)) +
			" label value(s), got " + strconv.Itoa(len(values)))
	}
	return m.get(values)
}

func (m *metric) must(s *series) *series {
	if s == nil {
		panic("trilha: metric " + m.name + " has labels; use With(...) before recording")
	}
	return s
}

// With binds label values, in the order they were declared.
func (c *Counter) With(values ...string) *Counter { return &Counter{m: c.m, s: c.m.bind(values)} }

// addTo is the lean path the framework uses on every request: it resolves the
// series and adds, without building a bound Counter (spec 012: the fixed cost
// per request is a budget, not a detail).
func (c *Counter) addTo(v float64, values ...string) { c.m.bind(values).add(v) }

// Inc adds one.
func (c *Counter) Inc() { c.Add(1) }

// Add increases the counter; negative values panic (a counter never falls).
func (c *Counter) Add(v float64) {
	if v < 0 {
		panic("trilha: counter " + c.m.name + " does not accept a negative value")
	}
	c.m.must(c.s).add(v)
}

// With binds label values, in the order they were declared.
func (g *Gauge) With(values ...string) *Gauge { return &Gauge{m: g.m, s: g.m.bind(values)} }

// Set replaces the value.
func (g *Gauge) Set(v float64) { g.m.must(g.s).set(v) }

// Add moves the value (use a negative number to go down).
func (g *Gauge) Add(v float64) { g.m.must(g.s).add(v) }

// Inc adds one; Dec subtracts one.
func (g *Gauge) Inc() { g.Add(1) }

// Dec subtracts one.
func (g *Gauge) Dec() { g.Add(-1) }

// With binds label values, in the order they were declared.
func (h *Histogram) With(values ...string) *Histogram { return &Histogram{m: h.m, s: h.m.bind(values)} }

// observeTo is Observe without building a bound Histogram; see Counter.addTo.
func (h *Histogram) observeTo(v float64, values ...string) { h.record(h.m.bind(values), v) }

// Observe records one value.
func (h *Histogram) Observe(v float64) { h.record(h.m.must(h.s), v) }

func (h *Histogram) record(s *series, v float64) {
	s.count.Add(1)
	s.add(v)
	for i, ub := range h.m.buckets {
		if v <= ub {
			s.buckets[i].Add(1)
			return
		}
	}
}

// validMetricName follows the Prometheus data model: [a-zA-Z_:][a-zA-Z0-9_:]*.
func validMetricName(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch == '_', ch == ':':
		case ch >= '0' && ch <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

var helpEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`)
var labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// write renders the whole registry in the Prometheus text format.
func (m *Metrics) write(w io.Writer) {
	var b strings.Builder
	m.mu.RLock()
	order := append([]*metric(nil), m.order...)
	m.mu.RUnlock()
	for _, mt := range order {
		mt.write(&b)
	}
	m.writeRuntime(&b)
	_, _ = io.WriteString(w, b.String())
}

func (m *metric) write(b *strings.Builder) {
	m.mu.RLock()
	all := append([]*series(nil), m.order...)
	m.mu.RUnlock()
	if len(all) == 0 {
		return
	}
	b.WriteString("# HELP " + m.name + " " + helpEscaper.Replace(m.help) + "\n")
	b.WriteString("# TYPE " + m.name + " " + m.kind.String() + "\n")
	for _, s := range all {
		if m.kind != histogramKind {
			b.WriteString(m.name)
			writeLabels(b, m.labels, s.values, "", "")
			b.WriteString(" " + num(s.value()) + "\n")
			continue
		}
		var acc uint64
		for i, ub := range m.buckets {
			acc += s.buckets[i].Load()
			b.WriteString(m.name + "_bucket")
			writeLabels(b, m.labels, s.values, "le", num(ub))
			b.WriteString(" " + strconv.FormatUint(acc, 10) + "\n")
		}
		b.WriteString(m.name + "_bucket")
		writeLabels(b, m.labels, s.values, "le", "+Inf")
		b.WriteString(" " + strconv.FormatUint(s.count.Load(), 10) + "\n")
		b.WriteString(m.name + "_sum")
		writeLabels(b, m.labels, s.values, "", "")
		b.WriteString(" " + num(s.value()) + "\n")
		b.WriteString(m.name + "_count")
		writeLabels(b, m.labels, s.values, "", "")
		b.WriteString(" " + strconv.FormatUint(s.count.Load(), 10) + "\n")
	}
}

func writeLabels(b *strings.Builder, names, values []string, extraName, extraValue string) {
	if len(names) == 0 && extraName == "" {
		return
	}
	b.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		v := ""
		if i < len(values) {
			v = values[i]
		}
		b.WriteString(n + `="` + labelEscaper.Replace(v) + `"`)
	}
	if extraName != "" {
		if len(names) > 0 {
			b.WriteByte(',')
		}
		b.WriteString(extraName + `="` + extraValue + `"`)
	}
	b.WriteByte('}')
}

// writeRuntime adds the Go runtime snapshot and the build info, read at
// scrape time so nothing is collected when nobody is looking.
func (m *Metrics) writeRuntime(b *strings.Builder) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	gauge := func(name, help string, v float64) {
		b.WriteString("# HELP " + name + " " + help + "\n# TYPE " + name + " gauge\n" + name + " " + num(v) + "\n")
	}
	gauge("go_goroutines", "Goroutines currently running.", float64(runtime.NumGoroutine()))
	gauge("go_memstats_alloc_bytes", "Bytes alocados e em uso.", float64(ms.Alloc))
	gauge("go_memstats_sys_bytes", "Bytes obtidos do sistema operacional.", float64(ms.Sys))
	b.WriteString("# HELP go_gc_cycles_total Completed garbage collection cycles.\n# TYPE go_gc_cycles_total counter\ngo_gc_cycles_total " + strconv.FormatUint(uint64(ms.NumGC), 10) + "\n")
	gauge("trilha_uptime_seconds", "Seconds since the process started.", time.Since(m.start).Seconds())
	version := "desconhecida"
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		version = bi.Main.Version
	}
	b.WriteString("# HELP trilha_build_info Binary and Go version (value is always 1).\n# TYPE trilha_build_info gauge\n")
	b.WriteString(`trilha_build_info{version="` + labelEscaper.Replace(version) + `",go_version="` + runtime.Version() + `"} 1` + "\n")
}
