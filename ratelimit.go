package trilha

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimit configures the per-client token bucket. Zero disables it.
type RateLimit struct {
	// RPS is the sustained requests per second per client.
	RPS float64
	// Burst is the bucket size (requests allowed at once).
	Burst int
}

type bucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	mu      sync.Mutex
	rps     float64
	burst   float64
	buckets map[string]*bucket
	calls   int
	now     func() time.Time
}

func newLimiter(rl RateLimit) *limiter {
	if rl.Burst <= 0 {
		rl.Burst = 1
	}
	return &limiter{rps: rl.RPS, burst: float64(rl.Burst), buckets: map[string]*bucket{}, now: time.Now}
}

// allow takes one token for key. It returns false and the seconds to wait
// when the bucket is empty.
func (l *limiter) allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rps
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	l.calls++
	if l.calls%1000 == 0 {
		l.sweep(now)
	}
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := (1 - b.tokens) / l.rps
	return false, int(wait) + 1
}

// sweep forgets clients idle for ten minutes.
func (l *limiter) sweep(now time.Time) {
	for k, b := range l.buckets {
		if now.Sub(b.last) > 10*time.Minute {
			delete(l.buckets, k)
		}
	}
}

// ErrRateLimited is the 429 error; handlers may return it themselves.
var ErrRateLimited = &HTTPError{Code: http.StatusTooManyRequests, Message: "too many requests"}

func (l *limiter) check(c *Ctx) error {
	ok, wait := l.allow(c.ClientIP())
	if ok {
		return nil
	}
	c.w.Header().Set("Retry-After", strconv.Itoa(wait))
	return ErrRateLimited
}

// Limit returns a middleware applying its own per-client limit to a subtree:
// put `var limit = trilha.Limit(2, 5)` in middleware.go and call it.
func Limit(rps float64, burst int) MiddlewareFunc {
	l := newLimiter(RateLimit{RPS: rps, Burst: burst})
	return func(c *Ctx, next Next) error {
		if err := l.check(c); err != nil {
			return err
		}
		return next()
	}
}
