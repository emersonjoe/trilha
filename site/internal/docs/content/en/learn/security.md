---
title: Security
description: What Trilha protects by default, how to adjust it, and what remains your responsibility.
---

Trilha follows two references: the **NIST Cybersecurity Framework 2.0** (the Identify,
Protect, Detect, Respond, Recover and Govern functions) and **OWASP ASVS 4.0** level 2. A web
framework can only *protect* and *detect*; the rest is the work of whoever operates the app,
and this chapter says exactly where one ends and the other begins.

## What comes turned on

| Control | Default | NIST CSF 2.0 | OWASP ASVS |
|---|---|---|---|
| HTML escaping (`h`) and contextual escaping (`tmpl`) | always | PR.DS | V5.3 |
| `Content-Security-Policy` with a per-request nonce | on | PR.PS | V14.4 |
| `Strict-Transport-Security` | on over HTTPS | PR.DS | V9.1 |
| `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy` | on | PR.PS | V14.4 |
| CSRF by *double-submit cookie* on forms | on | PR.AA | V4.2 |
| Request body limit (1 MiB) | on | PR.IR | V13.1 |
| Read, write and idle timeouts; header limit | on | PR.IR | V13.1 |
| Static files restricted to `public/` | always | PR.DS | V12.3 |
| Opaque errors in production; no stack, no paths | on | PR.DS | V7.4 |
| Structured logs without body or cookies, with `request_id` | always | DE.CM | V7.1 |
| Security events (CSRF, 401/403, 413, 429, panic) in the log | always | DE.AE | V7.2 |
| Signed cookies (`SetSigned`/`Signed`) | with `TRILHA_SECRET` | PR.AA | V3.4 |
| Per-client rate limit | optional | PR.IR | V11.1 |
| Trusted proxies (`X-Forwarded-*`) | optional | PR.AA | V14.1 |

## CSP and inline scripts

The default policy only allows scripts from the site itself or carrying the request's
**nonce**. An inline `<script>` needs it:

```go
h.Script(trilha.NonceAttr(c), h.Raw(`document.body.dataset.ready = "1"`))
```

The reload script of `trilha dev` already uses the nonce. To allow an external origin
(fonts, an image CDN) without rewriting the policy, add to `app/setup.go`:

```go
func Setup(a *trilha.App) error {
	a.Security().CSPExtra = map[string][]string{
		"style-src": {"https://fonts.googleapis.com"},
		"font-src":  {"https://fonts.gstatic.com"},
	}
	return nil
}
```

For a policy entirely your own, set `a.Security().CSP` (the string may contain `{nonce}`);
to turn a header off, assign `trilha.Off`.

## Behind a proxy

If the app runs behind nginx, Caddy or a load balancer, `RemoteAddr` is the proxy. Tell
Trilha whom to trust so that `X-Forwarded-For` and `X-Forwarded-Proto` count:

```bash
TRILHA_TRUSTED_PROXIES=10.0.0.0/8,127.0.0.1
```

Only then does `c.ClientIP()` return the real client, HSTS is sent and the rate limit counts
per client instead of per proxy. Without that variable, `X-Forwarded-*` headers are
ignored, which is the safe behavior.

## Session with a signed cookie

A signed cookie cannot be forged or altered, and it expires on its own:

```go
// in the login POST
if err := c.SetSigned("session", user.ID, 8*time.Hour); err != nil {
	return err
}

// in the middleware of the restricted area
id, ok := c.Signed("session")
if !ok {
	return trilha.RedirectCode("/login", 302)
}
```

The key comes from `TRILHA_SECRET` (32 bytes or more; `openssl rand -base64 32`). In
development, `trilha dev` generates an ephemeral key per session. In production without the
variable, `SetSigned` returns an error and logs a warning naming the cookie and the route —
once per cookie, and only for an app that actually signs one: a warning in every boot of an
app with its own session is what teaches a team to stop reading warnings. To rotate the key
without dropping sessions, put the old one in `TRILHA_SECRET_PREVIOUS` until they expire.

:::warning
A signed cookie guarantees integrity, not secrecy: the value is readable by whoever holds
the cookie. Store an identifier in it, never sensitive data.
:::

## Rate limiting

Globally, in `app/setup.go`, or per subtree, in a `middleware.go`:

```go
// app/api/middleware.go
var limit = trilha.Limit(5, 20) // 5 req/s per client, burst of 20

func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return limit(c, next)
}
```

The response is 429 with `Retry-After`. The counter lives in the process memory: with
several replicas, each one counts its own share.

## Detect and respond

Every block produces a `security` line in the log, with `kind`, `ip`, `path` and
`request_id`, and calls `Config.OnSecurityEvent` if you set one. That is the hook to count
attempts, alert or block an IP at the firewall.

Before publishing, run:

```bash
trilha audit
```

It checks `TRILHA_SECRET`, proxies, `trilha_gen.go`, the Go version, `go vet` and
`govulncheck`, and exits with an error when there is a critical item.

## Files that arrive from outside

An upload is the one request where the app writes what somebody else sent, under a name
somebody else chose. `c.File` answers with the file only after the three checks that matter:

```go
func POST(c *trilha.Ctx) error {
	c.AllowBody(8 << 20) // the request; the file limit below is another thing
	up, err := c.File("file", trilha.FileRules{
		MaxSize: 4 << 20,
		Accept:  []string{"image/*", "application/pdf"},
	})
	if err != nil {
		if errs, ok := err.(trilha.FieldErrors); ok {
			return c.Render(http.StatusUnprocessableEntity, page(c, errs))
		}
		return err
	}
	defer up.Close()
	path, err := up.Save("uploads") // never leaves "uploads"
	...
}
```

- **Size**: `MaxSize` is per file, apart from `Config.MaxBodyBytes`. A route that accepts a
  4 MB file still needs to let a slightly larger body through (`c.AllowBody`), because the
  body carries the other fields too.
- **Type**: `Accept` is matched against the type detected in the first 512 bytes of the
  content, never the extension and never the `Content-Type` the client announced — a PDF
  renamed to `photo.png` is a PDF. `up.MIME` is what it really is and `up.Ext` is the
  extension that matches. Careful: the standard library detects what it knows; formats that
  are a zip inside (`.docx`, `.xlsx`) come back as `application/zip`, and a CSV as
  `text/plain`. Where the difference matters, look at the content yourself.
- **Name**: `up.Name` has no directory, no separator of either platform, no control
  character, at most 100 characters, and is never empty or `..`. `up.Save(dir)` writes
  inside `dir` with mode 0600 and a free name (`note.pdf`, then `note-1.pdf`), so a second
  upload never eats the first.

A rule that fails is `FieldErrors` under the field's name — the same answer `Bind` gives, so
the form shows the message where the person is looking instead of the app answering 500.

Two things stay yours: **where** the file goes (a directory outside the code, a bucket,
a database) and **who** may send it. And a file the app serves back is served from a route
of yours, with the type you decided — never by handing the upload directory to
`http.FileServer`.

## What remains yours

- **Authentication and authorization**: who the user is and what they may do. Trilha gives
  you the signed cookie and the middleware; the business rule is yours.
- **Data at rest**: database encryption, backups, retention.
- **TLS**: terminate at the proxy or use a certificate in your own `http.Server` through
  `a.Handler()`.
- **Secrets**: only in environment variables or a vault; never in the repository.
- **Govern, Identify, Recover**: inventory, data classification, response plan and
  restoration are processes of the organization. The project's `SECURITY.md` describes how
  to report vulnerabilities in the framework.

## Challenge

Make the `/dashboard` area of your app require a signed session, with a limit of 10 attempts
per minute on the login form, and count in `OnSecurityEvent` how many blocks happened.

:::solution
```go
// app/login/middleware.go
var limit = trilha.Limit(10.0/60, 10)

func Middleware(c *trilha.Ctx, next trilha.Next) error { return limit(c, next) }

// app/setup.go
var blocks atomic.Int64

func Setup(a *trilha.App) error {
	a.Config().OnSecurityEvent = func(e trilha.SecurityEvent) {
		if e.Kind == "rate" { blocks.Add(1) }
	}
	return nil
}
```

`a.Config()` gives access to the configuration inside `Setup`; the login limiter uses
`trilha.Limit` with 10 tokens per minute.
:::
