# Threat model

> 🇺🇸 English · [🇧🇷 Português](docs/pt-BR/SECURITY-MODEL.md)

[`SECURITY.md`](SECURITY.md) says how to report a vulnerability and what the framework
guarantees. This document says **what it is defending against**, so an audit can tell an
intentional choice from an oversight — and so the gaps are written down instead of assumed.

It describes the framework, not your app. Everything below is checked against the code in
this repository; a control named here exists and is reachable from `Config` or from a
documented call. What has no control is listed as **open**.

## Assets

| Asset | Why it is worth attacking |
|---|---|
| The visitor's session | It is the identity: taking it is taking the account. |
| Application data | What the handlers read and write, including other people's data. |
| The signing secret (`TRILHA_SECRET`) | With it, an attacker mints any session cookie. |
| Availability of the process | One machine, one binary: exhausting it takes the site down. |
| The build chain | `trilha_gen.go`, the module graph, the CLI: code that runs with full privilege. |
| Static files and uploads | A file served from the app's origin runs in the app's origin. |

## Trust boundaries

```text
   browser ───(1)───► proxy / TLS ───(2)───► trilha process ───(3)───► database, APIs
   (hostile)                                    │      │
                                                │      └───(4)───► disk: public/, uploads
                                                │
   developer ───(5)───► trilha CLI ─────────────┘  (generated code, dependencies)
```

1. **Browser → app.** Everything is chosen by the caller: method, target, headers, cookies,
   body, `Host`, `Origin`. Nothing crossing here is trusted.
2. **Proxy → app.** `X-Forwarded-For` and `X-Forwarded-Proto` are only meaningful if the peer
   really is your proxy. The framework believes them **only** from a CIDR in
   `Config.TrustedProxies`; from anyone else the peer address wins.
3. **App → outside services.** Out of the framework's hands: it neither opens nor pools those
   connections. Your app owns it — with one exception, `Config.Upstreams`, where the
   framework does make the call. There the target is fixed in the configuration and no part
   of the request can move it; the credential injected by `Upstream.Headers` is the
   session's, never the `Authorization` the browser sent; and the response comes back with
   the upstream's own headers. Anything that upstream returns is data from outside this
   trust boundary: it is passed to the client, not interpreted by the app.
4. **App → disk.** `Config.Public` and `Config.Mounts` are an `fs.FS`, which cannot address
   anything above its root; uploads land through `Upload.Save`, which refuses a name that
   would walk out of the directory.
5. **Developer → build.** The CLI generates code that the app compiles. The generated file is
   committed and `trilha audit` warns when it is stale, so a silent generation is visible in
   review.

## Actors

- **Anonymous visitor** — the default caller of every route with no auth middleware.
- **Authenticated person** — has a session; may attack other people's data through the same
  routes.
- **Operator** — deploys, holds the secrets, reads the logs.
- **The proxy in front** — trusted only where `TrustedProxies` says so.
- **Network attacker** — sees and forges requests; assumed to control any header.

## Threats and controls (STRIDE)

Each line names the control and where it is configured. The explanation of each one lives in
the [security reference](https://emersonjoe.github.io/trilha/reference/security); it is not repeated here.

### Spoofing

| Threat | Control |
|---|---|
| Forged session cookie | HMAC-SHA256 with a key of at least 32 bytes, expiry inside the token, verified in constant time (`SetSigned`/`Signed`). |
| Session fixation | A new session identifier on every login (`auth`). |
| Stolen cookie over HTTP | `HttpOnly`, `SameSite=Lax` and `Secure` whenever the request is HTTPS, plus HSTS. |
| Key rotation without logging everyone out | `Config.PreviousSecret` still verifies while the new key signs. |
| Forged identity from the IdP | OIDC with `state`, `nonce`, PKCE `S256` and signature checked against the provider's JWKS (`auth`). |
| Forged client IP | `X-Forwarded-For` honoured only from `TrustedProxies`. |
| **Forged `Host`** (poisoned cache, password-reset link pointing at the attacker) | `Config.AllowedHosts`: a `Host` outside the list gets 400 before the router. **Off by default** — an empty list keeps today's behaviour. |

### Tampering

| Threat | Control |
|---|---|
| Cross-site request from another origin | Double-submit CSRF token on every form; `Config.CSRFForAPI` extends it to `route.go`. |
| Cookie edited by the client | The signature covers value and expiry; any change fails verification. |
| Injected HTML/JS | `h` escapes text and attributes; `tmpl` escapes per context; `h.Raw` is the one explicit way out. |
| Inline script injected into a page | CSP with a per-request nonce; `base-uri`, `form-action` and `frame-ancestors` locked down. |
| Upload that is not what it says | The media type is sniffed from the content, never from the name or the announced type (`FileRules.Accept`). |
| Path traversal, on the way in or out | `fs.FS` for static files; `Upload.Save` refuses a name that escapes the directory, and writes with mode 0600. |
| A download that the browser runs instead of saving | `Attachment` and `Inline` set the type from the content, always send `X-Content-Type-Options: nosniff`, and `Inline` refuses HTML, SVG and XML outright — a document with script served from the app's own origin is stored XSS with extra steps. |
| A document framed by a page of another origin | Every response carries `X-Frame-Options: DENY` and `frame-ancestors 'none'`. `Inline` — and only `Inline` — relaxes the pair to same-origin on that one response, because a document nobody may frame cannot be shown in place, which is what `Inline` exists for. The relaxation never applies to a page, to a download, or to an app that wrote its own `Security.CSP`. |
| A filename that writes a header of its own | The name of a download goes through the same sanitiser as an upload's, then out as a percent-encoded `filename*` plus a quoted ASCII `filename`: a quote, a separator or a newline in the name cannot add a parameter or a line. |
| Another service's headers landing on this app's session | `Pipe` copies a closed list of response headers and never `Set-Cookie`; the same is true of `Config.Upstreams`. |

### Repudiation

| Threat | Control |
|---|---|
| A blocked request leaves no trace | Every block (CSRF, 401/403, 413, 429, `host`, panic) emits a `SecurityEvent`: `slog.Warn`, the `trilha_security_events_total` metric and `Config.OnSecurityEvent`. |
| Logs that cannot be correlated | A request id per request, in the log and in the error body; `traceparent` is honoured when it is well formed. |
| **Business actions are not audited** | **Open.** The framework logs what it blocks, not what your app decides. An audit trail of domain actions is yours to write. |

### Information disclosure

| Threat | Control |
|---|---|
| Stack trace or internal detail in an error | `problem+json` in production carries status, title and request id; the detail of a 5xx only appears in `Dev`. |
| Metrics or health probes read by anyone | `Observability.Token` or `Observability.Trusted`; `trilha audit` fails when `/metrics` is exposed with neither. |
| Another origin reading responses | CORS is off unless configured; `Cross-Origin-Opener-Policy: same-origin` and `Referrer-Policy: strict-origin-when-cross-origin` by default. |
| Directory listing | Static serving refuses directories. |
| **A signed cookie is readable** | **By design.** `SetSigned` signs, it does not encrypt: the value is visible to whoever holds the cookie. Put an identifier in it, not a secret. |
| **The secret in the environment** | **Open.** `TRILHA_SECRET` lives in the process environment; the framework does not integrate with a secret manager. `trilha audit` checks that it exists and is long enough — nothing more. |

### Denial of service

| Threat | Control |
|---|---|
| Slow client holding connections | `Timeouts` (read header 10s, read 30s, write 60s, idle 120s). |
| Huge body | `Config.MaxBodyBytes` (1 MiB by default) answers 413; `MaxHeaderBytes` 64 KiB. |
| Request flood | Token bucket per client IP (`Config.RateLimit`) and `trilha.Limit` for one subtree; 429 with `Retry-After`. |
| A handler that panics takes the process down | Recovered at the edge, counted in `trilha_panics_total`, answered as 500. |
| **Volumetric attack, or one that costs the app more than it costs the attacker** | **Open, and out of scope.** A single process cannot absorb it: that belongs to the network in front (CDN, WAF, provider limits). |

### Elevation of privilege

| Threat | Control |
|---|---|
| Reaching a route without a session | `auth` middleware over the subtree; `HasRole` for role checks. |
| A logout that does not log out | `Store` (e.g. `MemoryStore`) revokes immediately; without one the cookie is stateless and valid until it expires. |
| Dependency with a known vulnerability | Zero external dependencies in the runtime and the CLI, enforced by a test; `govulncheck` on every CI run. |
| A bug in the parsing of third-party input | `go test -race` and six fuzz targets in CI (route matching, `Bind`, signed cookies, `traceparent`, escaping). |
| **Authorization inside the domain** | **Open, by design.** "Can this person read this invoice?" depends on your data model. The framework gives you the identity and the roles; the decision is yours. |

## Session without a provider

`auth.Sessions` puts the credential inside the app instead of at an identity provider, and
that moves three things in this model:

- **The password hash is now an asset of this app.** `auth.CheckPBKDF2` reads and
  `auth.HashPBKDF2` writes `pbkdf2_sha256$iterations$salt$hash`; the comparison is
  constant-time, and a hash the function cannot parse is a false, never a panic. The
  iteration count travels inside each hash, so raising the default locks nobody out.
- **The login is a guessing oracle unless it is limited.** With OIDC the provider absorbs
  that; here it is `Config.RateLimit`, and `trilha audit` says so when a `Login` has none.
  Answer the same message for a wrong e-mail and a wrong password, and take the same time.
- **`User.Extra` travels with the session** — in the signed cookie, or in the `Store`. It is
  for a token an upstream needs, a tenant, a plan: never for a password, and never for more
  than a cookie should carry.

Revocation is the `Store`'s job, as with OIDC: without one, a session lives in the cookie
until it expires and `Logout` only stops that browser from sending it.

## What the framework does not do for you

- Decide who may see what (domain authorization).
- Store secrets, rotate them, or keep them out of your logs.
- Protect what your handler does with the data after `Bind` — SQL, shell, other people's APIs.
- Absorb a volumetric attack, or replace a WAF.
- Redirect to a canonical host (`www` → apex): declare the hosts you answer for, and route the
  rest at the proxy.
- Encrypt anything at rest.

## Keeping this document honest

Every line above points at code in this repository. When a control changes, this file changes
in the same commit — the same rule the public documentation follows. Reviewed against
[spec 004](specs/004-seguranca/) (headers, CSRF, signed cookies, rate limiting),
[spec 014](specs/014-observabilidade/) (events, probes, metrics) and
[spec 034](specs/034-modelo-de-ameacas/) (this document and `AllowedHosts`).
