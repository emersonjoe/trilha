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

### When the response belongs to a host

An app mounted inside a server that already answers for its own responses has two headers
too many, not one: the host wrote the policy, and the app writes it again.

| Field | Effect |
|---|---|
| `Delegated bool` | writes none of the headers — not the six that have an `Off`, and not the `nosniff` that has none |
| `Nonce func(*http.Request) string` | the nonce comes from the host, one call per request that asks for it |

```go
a.Security().Delegated = true
a.Security().Nonce = func(r *http.Request) string { return host.NonceOf(r) }
```

`Delegated` is a decision, not a default: the zero value writes the headers, so a
hand-written `Security{...}` never turns them off by omission. Boot records the delegation
once in the log, because a response with no headers should be visible somewhere.

Without `Nonce`, `c.Nonce()` invents a value per request, which is right for an app that
publishes its own CSP and wrong for one that does not: the host's policy never heard of that
nonce, and the browser refuses the script. With `Nonce` returning an empty string,
`trilha.NonceAttr(c)` renders no attribute at all instead of `nonce=""`.

## Trusted proxies

`Config.TrustedProxies []string` (CIDR or IP) or `TRILHA_TRUSTED_PROXIES=a,b`. Effects when
the peer is trusted: `c.ClientIP()` reads `X-Forwarded-For` (the rightmost IP that is not a
proxy), `X-Forwarded-Proto: https` turns on HSTS and marks cookies as `Secure`.

## Allowed hosts

`Config.AllowedHosts []string` or `TRILHA_ALLOWED_HOSTS=a,b`. A request whose `Host` is not in
the list is answered with 400 before the router, the probes and CORS, and emits a `host`
event. Empty list = no check.

| Pattern | Allows | Does not allow |
|---|---|---|
| `example.com` | `example.com`, `example.com:8443`, `EXAMPLE.com.` | `sub.example.com` |
| `*.example.com` | `app.example.com` | `example.com`, `a.b.example.com` |

In `Dev`, `localhost`, `127.0.0.1` and `::1` always pass. The value compared is the host the
app receives — behind a proxy that rewrites `Host`, list what the proxy sends.

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
	Kind      string // csrf | auth | body | host | rate | panic
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

A missing `TRILHA_SECRET` is critical only when the code signs something — `SetSigned`,
`Signed`, a `Signer` of its own, `Config.Secret`, or the `auth` package. An app whose
session is not Trilha's gets a warning instead: a secret that signs nothing still enters the
`.env`, the deploy and the rotation, and the day somebody rotates it nothing happens, which
is the worst thing a secret can teach. Set too short is critical either way — whoever set it
meant to use it.
