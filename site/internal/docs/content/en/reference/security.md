---
title: Security
description: Complete configuration of headers, proxies, rate limiting, signed cookies and events.
---

## Config.Security

| Field | Default | Header |
|---|---|---|
| `CSP` | nonce policy (below) | `Content-Security-Policy` |
| `CSPExtra map[string][]string` | — | adds origins to directives of the default policy |
| `HSTS` | `max-age=31536000; includeSubDomains` (HTTPS only) | `Strict-Transport-Security` |
| `PermissionsPolicy` | `camera=(), microphone=(), geolocation=(), payment=(), usb=()` | `Permissions-Policy` |
| `COOP` | `same-origin` | `Cross-Origin-Opener-Policy` |
| `FrameOptions` | `DENY` | `X-Frame-Options` |
| `Referrer` | `strict-origin-when-cross-origin` | `Referrer-Policy` |

`trilha.Off` in any field removes the header. `X-Content-Type-Options: nosniff` is always
sent. Default policy:

```text
default-src 'self'; script-src 'self' 'nonce-…'; style-src 'self' 'unsafe-inline';
img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';
base-uri 'self'; form-action 'self'
```

`c.Nonce()` returns the request's nonce; `trilha.NonceAttr(c)` puts it on an `h.Script`.
Adjust in `Setup` through `a.Security()`.

## Trusted proxies

`Config.TrustedProxies []string` (CIDR or IP) or `TRILHA_TRUSTED_PROXIES=a,b`. Effects when
the peer is trusted: `c.ClientIP()` reads `X-Forwarded-For` (the rightmost IP that is not a
proxy), `X-Forwarded-Proto: https` turns on HSTS and marks cookies as `Secure`.

## Rate limiting

`Config.RateLimit{RPS float64, Burst int}` applies a *token bucket* per `ClientIP` before
the middlewares. `trilha.Limit(rps, burst) MiddlewareFunc` creates an independent limiter for
a subtree. Response: 429 with `Retry-After` (seconds) and a `rate` event.
`trilha.ErrRateLimited` may be returned by a handler for the same effect.

## Signed cookies

| Symbol | Description |
|---|---|
| `c.SetSigned(name, value, ttl) error` | writes a `value|expires|hmac` cookie with `HttpOnly`, `SameSite=Lax`, `Secure` over HTTPS; `ErrNoSecret` without a key |
| `c.Signed(name) (string, bool)` | reads and verifies signature and expiry |
| `c.ClearCookie(name)` | expires a cookie |
| `trilha.NewSigner(keys...)`, `Sign`, `Verify` | the signer (HMAC-SHA256) for direct use |
| `Config.Secret`, `Config.PreviousSecret` | `TRILHA_SECRET`, `TRILHA_SECRET_PREVIOUS` (base64 or text, ≥ 32 bytes) |

Without a secret: in `dev` an ephemeral key is generated (`trilha dev` keeps one per
session); in `prod` the app warns in the log and `SetSigned` returns `ErrNoSecret`.

## Timeouts

`Config.Timeouts{ReadHeader 10s, Read 30s, Write 60s, Idle 120s, MaxHeaderBytes 64 KiB}`.
For long responses (SSE, download), call `c.NoWriteDeadline()` before writing.

## Security events

```go
type SecurityEvent struct {
	Kind      string // csrf | auth | body | rate | panic
	Status    int
	Method    string
	Path      string
	IP        string
	RequestID string
}
```

Logged with `slog.Warn("security", ...)` and delivered to `Config.OnSecurityEvent`, once per
request.

## `trilha audit`

Checks: `TRILHA_SECRET`, `TRILHA_TRUSTED_PROXIES`, up-to-date `trilha_gen.go`, Go version,
`.gitignore`, `go vet` and `govulncheck` (`--no-vuln` to skip). Exit code 1 with a critical
item.
