package cookbook

import (
	"os"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// Config is the production side of app/setup.go. Everything here has a
// default that works in dev and is wrong behind a proxy on the open
// internet — which is exactly the list worth reviewing before a deploy.
func Config(cfg *trilha.Config) error {
	// Who may say which Host: without this, a request with someone else's
	// Host is answered with your session cookie in it.
	cfg.AllowedHosts = strings.Split(os.Getenv("ALLOWED_HOSTS"), ",")
	// The proxy in front. Only these addresses may set X-Forwarded-For, so
	// ClientIP is the visitor and not whatever the visitor typed.
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	// A request that never finishes is a connection that never returns.
	cfg.Timeouts = trilha.Timeouts{
		ReadHeader: 5 * time.Second,
		Read:       30 * time.Second,
		Write:      30 * time.Second,
		Idle:       60 * time.Second,
		Shutdown:   20 * time.Second,
	}
	// The ceiling on a body nobody asked for; a route that receives files
	// raises its own with c.AllowBody.
	cfg.MaxBodyBytes = 1 << 20
	cfg.RateLimit = trilha.RateLimit{RPS: 20, Burst: 40}
	// Metrics are opt-in and never public. ConfigFromEnv already read
	// TRILHA_METRICS and TRILHA_OBS_TOKEN; what is left is who may scrape.
	cfg.Observability.Trusted = []string{"10.0.0.0/8"}
	// HSTS is a promise the browser remembers: turn it on when the
	// certificate is already working, not before.
	cfg.Security.HSTS = "max-age=31536000; includeSubDomains"
	return nil
}
