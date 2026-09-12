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
`trilha.NonceFrom(r)` answers the same value to a renderer that only has the
`*http.Request` — `html/template`, `templ`, a handler of your own — so the shell of an app
being migrated does not need a middleware of its own to reach it.
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
the list is answered with 400 before the router and CORS, and emits a `host` event. Empty
list = no check.

| Pattern | Allows | Does not allow |
|---|---|---|
| `example.com` | `example.com`, `example.com:8443`, `EXAMPLE.com.` | `sub.example.com` |
| `*.example.com` | `app.example.com` | `example.com`, `a.b.example.com` |

In `Dev`, `localhost`, `127.0.0.1` and `::1` always pass. The value compared is the host the
app receives — behind a proxy that rewrites `Host`, list what the proxy sends.

The health probe (`/_trilha/health`, `/live`, `/ready`) answers before this check, with any
`Host`: a liveness/readiness probe is addressed by IP (the container's, the pod's), never by
a name in the list, and it never reflects the `Host` back — no link, no cookie, no redirect,
no Host-keyed cache. Point your orchestrator's probe at one of these three paths; a health
route the app writes itself is a route like any other and stays behind this check.

## Rate limiting

`Config.RateLimit{RPS float64, Burst int}` applies a *token bucket* per `ClientIP` before
the middlewares. `trilha.Limit(rps, burst) MiddlewareFunc` creates an independent limiter for
a subtree. Response: 429 with `Retry-After` (seconds) and a `rate` event.
`trilha.ErrRateLimited` may be returned by a handler for the same effect.

**The bucket by itself, when the key is not an address.** The interesting budget is rarely the
IP: a limit per API key, per tenant, per mailbox — one password reset per address, not one per
office behind a single address — or per background job. `trilha.NewLimiter` is the same token
bucket the middleware uses, as a value you keep and key yourself:

```go
var perKey = trilha.NewLimiter(trilha.RateLimit{RPS: 10, Burst: 30})

if ok, after := perKey.Allow(key.ID); !ok {
	c.Header().Set("Retry-After", strconv.Itoa(int(after.Seconds()+0.5)))
	return trilha.ErrRateLimited
}
```

`Limiter.Allow(key)` answers whether that key may go now and, when it may not, how long until
it may — which is the number `Retry-After` needs and the reason it comes back instead of being
logged. Buckets are created on first use and swept when they refill, so a key nobody uses again
does not stay in memory. `auth.APIKeys` limits per key with this exact type, which is why it is
exported: an application that has a string to key on should not be writing a second token
bucket, and the one it would write is the one that leaks a map.

## Signed cookies

| Symbol | Description |
|---|---|
| `c.SetSigned(name, value, ttl) error` | writes a `value|expires|hmac` cookie with `HttpOnly`, `SameSite=Lax`, `Secure` over HTTPS; `ErrNoSecret` without a key |
| `c.Signed(name) (string, bool)` | reads and verifies signature and expiry |
| `c.ClearCookie(name)` | expires a cookie |
| `trilha.NewSigner(keys...)`, `Sign`, `Verify` | the signer (HMAC-SHA256) for direct use |
| `Config.Secret`, `Config.PreviousSecret` | `TRILHA_SECRET`, `TRILHA_SECRET_PREVIOUS` (base64 or text, at least `trilha.MinSecretLen` bytes — 32; a shorter one in `prod` is the hint `trilha.ErrSecretShort`) |

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

It also warns about a write that no `Kind` reaches. A `route.go` is an API, and an API does
not check the CSRF token, so a `POST` route in an app that also serves pages usually wants
`var Kind = trilha.KindPage` in a `kind.go` above it — one line for the whole branch, see
[File conventions](/reference/conventions#kind-follows-the-subtree). Setting
`Config.CSRFForAPI` answers the same question the other way and silences the warning too.

## Secrets at rest

The framework had a secret, a signer and signed cookies. What it did not have was *encrypt this
to store it* — and what gets written instead is a token in the clear, then AES copied off the
internet with a fixed IV, then the whole key coming back in a `GET` and showing up in the
DevTools.

```go
sealed, err := trilha.Seal([]byte(apiKey)) // AES-256-GCM, random nonce
plain, err := trilha.Open(sealed)          // current secret, then PreviousSecret
```

| Symbol | Role |
|---|---|
| `Seal([]byte) ([]byte, error)` | encrypts for storage; `ErrNoSecret` without a secret, never a value in the clear |
| `Open([]byte) ([]byte, error)` | decrypts; `ErrSealed` for anything it cannot open, without saying why |
| `Secret` | a string that cannot leak by accident |
| `s.Reveal()` | the value, read on purpose |
| `s.Sealed()` / `SecretFrom(b)` | the encrypted form and back |
| `s.Value()` / `s.Scan()` | `driver.Valuer` and `sql.Scanner`: the column holds the sealed bytes |
| `s.String()`, `s.LogValue()`, `s.MarshalJSON()` | the mask, always — the three exits a value leaks through are `%v`, a `slog` field and a struct answered as JSON, and this type exists to close all three |
| `s.UnmarshalJSON(b)` | the value as it comes, because a client sending a new secret sends the secret; a **mask** arriving back is "unchanged" and is never stored as if it were the value |
| `ui.SecretField(c, name, label, current)` | the password field for a hand-written form |

**`Value()` is the driver method and `Reveal()` is the reader.** The issue that asked for this
type had it the other way round; it cannot be. A `Secret` is a string underneath, and
`database/sql` converts a string-kinded value all by itself — so unless `Value()` is the
`driver.Valuer`, passing a `Secret` to a query stores the plaintext, silently.

**A masked value coming back is "unchanged".** A form has no way to send "I did not touch this",
so a screen that pre-fills a secret either sends it back to the browser or wipes it on the first
save nobody retyped. `Bind` treats an empty **or masked** value as unchanged, `trilha.Settings`
renders a `Secret` as a password field that is always empty, and the help line says so.

**Rotation.** `Open` tries the current secret and then `Config.PreviousSecret`; `Seal` always
uses the current one. So: set the old key as `TRILHA_PREVIOUS_SECRET`, deploy the new
`TRILHA_SECRET`, let everything be re-sealed, then drop the old one. `trilha audit` warns when
there is a `trilha.Secret` in the project and no previous secret — rotating then is the moment
every stored token stops opening.

:::warning
The key is derived from the app's secret. This is encryption **against a database dump and a
backup**, not against the operator and not against anybody who holds the environment — they have
the key by definition. Encrypting against your own infrastructure needs a key manager, and the
framework does not pretend to be one.
:::
