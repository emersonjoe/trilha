package trilha

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// healthMediaType is the media type of the health response
// (draft-inadarei-api-health-check).
const healthMediaType = "application/health+json"

const (
	// StatusPass means every check passed.
	StatusPass = "pass"
	// StatusFail means at least one check failed.
	StatusFail = "fail"
)

// HealthCheck is one registered readiness check.
type healthCheck struct {
	name string
	fn   func(context.Context) error
}

// CheckResult is the outcome of one readiness check. Error is only ever sent
// to an authorized client; anonymous callers see the status alone.
type CheckResult struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	DurationMS float64 `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
}

// HealthReport is the readiness of the application.
type HealthReport struct {
	Status        string        `json:"status"`
	Checks        []CheckResult `json:"checks,omitempty"`
	UptimeSeconds float64       `json:"uptime_seconds,omitempty"`
}

// Check registers a readiness check: a dependency the app needs in order to
// serve (database, cache, queue). It runs on /_trilha/health and
// /_trilha/health/ready, with the configured timeout, and its result is
// cached for Observability.CacheFor. Liveness never runs checks, so a
// dependency blinking does not restart the process.
func (a *App) Check(name string, fn func(context.Context) error) {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	a.checks = append(a.checks, healthCheck{name: name, fn: fn})
	a.healthCache = nil
}

// HealthReport runs the readiness checks and returns the full report,
// including error messages. Use it from your own code (a status page, a
// startup gate); the HTTP endpoint decides what to reveal.
func (a *App) HealthReport(ctx context.Context) HealthReport {
	if rep, ok := a.cachedHealth(); ok {
		return rep
	}
	a.healthMu.Lock()
	checks := append([]healthCheck(nil), a.checks...)
	a.healthMu.Unlock()

	rep := HealthReport{Status: StatusPass, UptimeSeconds: time.Since(a.metrics.start).Seconds()}
	rep.Checks = make([]CheckResult, len(checks))
	timeout := a.cfg.Observability.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	var wg sync.WaitGroup
	for i, ck := range checks {
		wg.Add(1)
		go func(i int, ck healthCheck) {
			defer wg.Done()
			rep.Checks[i] = runCheck(ctx, ck, timeout)
		}(i, ck)
	}
	wg.Wait()
	for _, r := range rep.Checks {
		if r.Status != StatusPass {
			rep.Status = StatusFail
		}
	}
	a.storeHealth(rep)
	return rep
}

// runCheck applies the deadline and turns a panic into a failure, so a broken
// check cannot take the process down through the probe.
func runCheck(ctx context.Context, ck healthCheck, timeout time.Duration) (res CheckResult) {
	res = CheckResult{Name: ck.name, Status: StatusPass}
	start := time.Now()
	defer func() {
		if v := recover(); v != nil {
			res.Status = StatusFail
			res.Error = fmt.Sprintf("pânico na verificação: %v", v)
		}
		res.DurationMS = float64(time.Since(start).Microseconds()) / 1000
	}()
	if timeout != NoTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	done := make(chan error, 1)
	go func() {
		defer func() {
			if v := recover(); v != nil {
				done <- fmt.Errorf("pânico na verificação: %v", v)
			}
		}()
		done <- ck.fn(ctx)
	}()
	select {
	case err := <-done:
		if err != nil {
			res.Status = StatusFail
			res.Error = err.Error()
		}
	case <-ctx.Done():
		// The check is still running; it will finish into the buffered
		// channel and be collected. The probe does not wait for it.
		res.Status = StatusFail
		res.Error = "prazo esgotado: " + ctx.Err().Error()
	}
	return res
}

func (a *App) cachedHealth() (HealthReport, bool) {
	ttl := a.cfg.Observability.CacheFor
	if ttl == NoTimeout {
		return HealthReport{}, false
	}
	if ttl == 0 {
		ttl = time.Second
	}
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	if a.healthCache != nil && time.Since(a.healthAt) < ttl {
		return *a.healthCache, true
	}
	return HealthReport{}, false
}

func (a *App) storeHealth(rep HealthReport) {
	if a.cfg.Observability.CacheFor == NoTimeout {
		return
	}
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	cp := rep
	cp.Checks = append([]CheckResult(nil), rep.Checks...)
	a.healthCache, a.healthAt = &cp, time.Now()
}
