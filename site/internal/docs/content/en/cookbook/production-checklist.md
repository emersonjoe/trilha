---
title: Production checklist
description: What to check before publishing, in order: what trilha audit finds for you, what it cannot see, and the two things to prepare for the day it goes wrong.
---

The list below is meant to be read from top to bottom, once, before the first deploy — and
again when something changes shape. Half of it is a command; the other half is a decision
nobody can make for you.

## Run the command first

```bash
trilha audit
```

It refuses to be a formality: it exits non-zero on anything critical, so CI can gate on it.
What it checks, in its own order:

| Check | Why it is on the list |
|---|---|
| `TRILHA_SECRET` set and long enough | an unset secret means cookies and CSRF signed with a key that changes on every restart |
| trusted proxies declared | without them `ClientIP` is whatever the visitor typed, and the rate limit protects nobody |
| allowed hosts declared | a request with someone else's `Host` gets an absolute link — and your cookie — pointing there |
| metrics not public, token long enough | `/metrics` is a map of your app: routes, volumes, error rates |
| at least one `a.Check` | without one, `ready` says yes while the database is gone |
| assets `immutable` | the only cache header that is both safe and worth having, because `c.Asset` hashes the name |
| OIDC secret not hardcoded, callback not cleartext | the two ways a login gets stolen |
| `trilha_gen.go` fresh, CLI and library on the same version | a generated file that disagrees with `app/` serves the routes of last week |
| supported Go, `.gitignore` covering `.env`, `go vet`, `govulncheck` | the ordinary hygiene that is only missed when it fails |

Fix everything critical. A warning is a decision: write down why, or fix it.

## What the command cannot see

### Configuration

```go
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
```

Timeouts are the item people skip. A request that never finishes is a connection that never
returns, and the failure looks like "the site is slow" until it looks like "the site is down".

### Data

- **Backup, and a restore you have actually performed.** A backup nobody restored is a file,
  not a backup. Time the restore: that number is your worst outage.
- **Migrations applied before the new version serves**, not by the instance that just started
  — with more than one instance, two of them run the same migration at the same time.
- **A rollback that works.** A migration that drops a column makes the previous version
  unable to start. Add the column, deploy, stop using it, drop it in the next release.

### Requests

- **Body limit** in `MaxBodyBytes`, raised per route with `c.AllowBody` only where a file
  arrives.
- **Rate limit** on what costs money: login, password reset, anything that sends an e-mail or
  calls a model.
- **`AllowedHosts` and HSTS** together — HSTS is a promise the browser remembers for a year,
  so turn it on after the certificate works, never before.

### What you will look at when it breaks

- **Structured logs going somewhere you can search**, with the request id in them. `c.Log()`
  already carries it.
- **The `/_trilha/health/ready` probe wired to the orchestrator**, and `live` wired to the
  restart — the other way round restarts a container forever because its database is down.
- **An alert on something a person feels**: error rate and p95 latency, not CPU.
- **No personal data in the logs.** A log line with an e-mail in it is a copy of your user
  table in a third-party service.

## The two things to prepare for the bad day

1. **How to roll back.** The previous image, the previous tag, and the certainty that the
   previous version still talks to the current database.
2. **How to rotate the secret.** `TRILHA_SECRET` gets the new value, `TRILHA_SECRET_PREVIOUS`
   the old one, for as long as a session lasts. Sessions signed with the old key keep working
   while they expire; new ones use the new key. Removing the old value ends every session at
   once, which is exactly what you want if the key leaked.

:::note
Everything here is one repository's list. If yours has an item this one does not, that item is
worth more than all of these — it came from an outage.
:::
