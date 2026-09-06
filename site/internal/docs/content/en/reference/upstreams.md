---
title: Upstreams
description: Forwarding a URL prefix to an API that already exists, with the session's credential.
---

An app that talks to an API of its own is one thing; an app that *is* the front of an API
that already exists is another. The second one needs `/api/*` on the same origin — no CORS,
no address of the API in the browser — and it needs the call to leave carrying the
credential of the session. A Next.js app writes the first half in `next.config.ts` and the
second half by hand. `Config.Upstreams` is both halves:

```go
// app/setup.go
func Config(cfg *trilha.Config) {
	cfg.Upstreams = map[string]trilha.Upstream{
		"/api/": {
			Target: os.Getenv("API_URL"), // http://api:8801
			Headers: func(c *trilha.Ctx, hdr http.Header) {
				if tok := sessao.Token(c); tok != "" {
					hdr.Set("Authorization", "Bearer "+tok)
				}
			},
			Timeout: 60 * time.Second,
		},
	}
}
```

| Field | Role |
|---|---|
| `Target` | base URL of the API; a path in it prefixes every forwarded request |
| `Headers` | runs with the outbound headers, before the request leaves |
| `Timeout` | bounds the whole exchange (default 30s; `trilha.NoTimeout` disables it) |
| `CSRF` | `trilha.Off` stops requiring the token on writes |
| `Transport` | replaces `http.DefaultTransport` (mTLS to the API, a test) |

## What the framework decides

- **A route of the app wins.** The upstream is the last route of its prefix, so a
  `route.go` at `/api/documents` answers before it — and an API can be migrated to Go one
  endpoint at a time without the front knowing.
- **The longest prefix wins**, so `/api/v2/` can point somewhere else than `/api/`.
- **The body streams both ways.** `MaxBodyBytes` and the write deadline were written for
  handlers, not for a pipe: a 200 MB upload and a PDF coming back both cross whole.
- **The response passes intact.** `Content-Type`, `Content-Disposition`, `Cache-Control`
  and `ETag` from the upstream win; Trilha's security headers stay where the upstream said
  nothing.
- **CSRF is on by default** for `POST`/`PUT`/`PATCH`/`DELETE`, as on any write of the app —
  `ui.js` already sends the header. `CSRF: trilha.Off` is for an API authenticated by key.
- **A dead upstream is 502 and a slow one is 504**, both `problem+json` whatever `Accept`
  says: the client of `/api/` is a script, never a browser in the address bar.

## What does not cross

The `Authorization` the browser sent is **dropped** before `Headers` runs. The credential
is the session's, not the client's: whoever sends a `Bearer` of their own does not get to
talk to the API with it. Hop-by-hop headers (`Connection`, `Upgrade`, `Proxy-*`) are
removed, as any proxy must. `X-Request-ID` and `traceparent` do cross, so the app's log
and the API's log are about the same request; `X-Forwarded-For`, `-Proto` and `-Host` say
who asked.

The `Cookie` header **is** forwarded — an API that has a session of its own still needs it.
If that is not your case, clear it in `Headers`:

```go
Headers: func(c *trilha.Ctx, hdr http.Header) {
	hdr.Del("Cookie")
	hdr.Set("Authorization", "Bearer "+token(c))
},
```

## Never an open proxy

The target lives in the configuration; the request only chooses the path under the prefix.
There is no header, query or path that moves it. A target that does not parse is a
complaint in the log at boot, not a 502 an hour later — and the app keeps answering
everything else.

`trilha audit` reports a `Target` written as `http://` to a host that is not this machine
(the session's credential crossing a network in the clear), and an upstream with no
`Headers` in an app that requires a login (a credential probably forgotten).

## Where it fits

The proxy answers outside the middleware chain, so nothing put the user in the request
context: read the session from the cookie, with `Session(c)` and not `User(c)`.

```go
func Token(c *trilha.Ctx) string {
	u, err := Flow.Session(c)
	if err != nil {
		return ""
	}
	return u.Extra["api_token"]
}
```

`examples/local-login` is the whole shape: a login against the app's own users table, and
`/api/` forwarded with what that login stored. See also [Auth](/reference/auth) for the
session without OIDC, and the recipe
[An app in front of an existing API](/cookbook/existing-api).
