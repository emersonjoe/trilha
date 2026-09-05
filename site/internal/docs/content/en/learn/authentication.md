---
title: Authentication with Entra ID, Keycloak and Cognito
description: OpenID Connect login with PKCE, signed session, roles and federated logout, with no external dependency and no password in your database.
---

Almost every internal app reaches the same point: someone asks "can I sign in with my
company account?". The answer is OpenID Connect — Entra ID (formerly Azure AD) and Keycloak
speak the same protocol, and the `auth` package implements the app side with the standard
library.

The advantage is not only convenience. A password you do not store is a password you do not
leak; MFA, lockout after failed attempts and rotation policy become the provider's problem,
and they have a team for that. What is left for the app is the part nobody can outsource:
validating the token properly and keeping the session in order.

## The flow, in three routes

The authorization code flow with PKCE has four steps: the app sends the browser to the
provider, the person authenticates there, the provider returns a code to a route of yours,
and the app exchanges that code for an `id_token` — this last exchange happens server to
server, it never goes through the browser. In `app/`, that is three files:

```go
// app/login/route.go
var Kind = trilha.KindPage
func GET(c *trilha.Ctx) error { return sso.Start(c) }

// app/login/callback/route.go
var Kind = trilha.KindPage
func GET(c *trilha.Ctx) error { return sso.Callback(c) }

// app/logout/route.go
var Kind = trilha.KindPage
func POST(c *trilha.Ctx) error { return sso.Logout(c) }
```

`sso` here is a package of yours, some 30 lines, that reads the environment and holds the
`*auth.Auth` (see `examples/sso/internal/sso`). `auth` registers no route: `app/` decides
the addresses, as everywhere else in the framework.

## Configuring the provider

```go
p := auth.EntraID(os.Getenv("SSO_TENANT"), id, secret, "https://app.example/login/callback")
// or
p := auth.Keycloak("https://kc.example", "production", id, secret, redirect)
// or
p := auth.Cognito("us-east-1", "us-east-1_ABC123", id, secret, redirect)
// or any conforming provider, by issuer:
p := auth.OIDC("https://accounts.example/", id, secret, redirect)

flow := auth.New(p, auth.Options{LoginPath: "/login", AfterLogin: "/dashboard"})
```

Nothing there makes a network call: discovery (`/.well-known/openid-configuration`) happens
on the first login and is cached for one hour. A provider that is down does not stop the app
from starting — it only stops people from signing in, which is the honest behavior.

The client secret **never** goes in the code. `trilha audit` complains if it finds a literal
in that position, and a secret that made it into git must be rotated at the provider, not
merely removed from the file.

## Cognito and the logout that is not standard

`auth.Cognito("us-east-1", "us-east-1_ABC123", …)` builds the issuer
`https://cognito-idp.<region>.amazonaws.com/<user-pool-id>` and reads roles from
`cognito:groups`, where a user pool keeps its groups.

One piece is missing, and it is a piece the standard does not cover: **Cognito publishes no
`end_session_endpoint`**. Ending the session there is a `GET /logout` on the managed login
domain, with a different parameter name (`logout_uri`, not `post_logout_redirect_uri`):

```go
p := auth.Cognito("us-east-1", "us-east-1_ABC123", id, secret, redirect)
p.LogoutDomain = "example.auth.us-east-1.amazoncognito.com" // or your own domain
```

Without `LogoutDomain`, `Logout` clears the cookie, writes in the log that a local logout was
all it could do, and returns to `AfterLogout` — it does not invent a federation that is not
there. The return URL has to be in the app client's *Allowed sign-out URLs*, or Cognito
refuses it.

## Other providers

Anything that speaks OIDC works through `auth.OIDC`, pointed at the issuer: roles come from
`roles`/`groups`, and a different claim name goes in `Options.RoleClaims`. Google enters that
way. A shortcut only saves you from getting the issuer wrong and knows where that provider
keeps its roles — it is convenience, not capability.

Clerk is the case we did not close. Its documentation describes `/.well-known/jwks.json` and
an `id_token` carrying `org_id`, but neither a `/.well-known/openid-configuration` — which is
where `auth` gets every endpoint — nor a claim with the person's role in the organization.
Until that is confirmed against a real instance there is no honest shortcut to write, and
the same doubt applies to plain `auth.OIDC`: see
[issue #41](https://github.com/emersonjoe/trilha/issues/41).

## Protecting part of the app

It is a `middleware.go`, like any other:

```go
// app/dashboard/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.Require(c, next) }

// app/dashboard/report/middleware.go — requires a role, not just a session
func Middleware(c *trilha.Ctx, next trilha.Next) error { return sso.RequireAdmin(c, next) }
```

Below the middleware the page reads `flow.User(c)` and trusts it: the `*auth.User` is there,
with `Subject`, `Email`, `Name` and `Roles`.

Two different answers for two different situations:

- **anonymous**: a browser goes to `/login?next=/dashboard`; any other client gets **401**.
  Redirecting an API call to an HTML form only produces a parsing error that is hard to
  understand on the other side.
- **signed in, but without the role**: **403**. Sending someone who is already signed in to
  the login creates a loop — they sign in again, come back, and get 401 once more.

## Where roles live

Each provider keeps them in one place, and `auth` already knows where to look:

| Provider | Reads from |
|---|---|
| Entra ID | `roles` (app roles), `groups`, `wids` |
| Keycloak | `realm_access.roles` and `resource_access[your-client].roles` |
| Generic | `roles`, `groups` |

Keycloak roles that belong to **another** client do not count: whoever is `admin` in the
accounting client does not become `admin` in yours. If your installation uses another claim
name, add it to `Options.RoleClaims`.

## The session

After the login, the `id_token` has done its job and is discarded. What stays is a cookie
signed with `TRILHA_SECRET`, `HttpOnly`, `SameSite=Lax` and `Secure` under HTTPS, holding
the essentials: identifier, name, e-mail, roles and deadlines.

```go
auth.Options{
	Absolute: 8 * time.Hour,  // maximum lifetime, counted from the login
	Idle:     30 * time.Minute, // gone after being idle
	Store:    auth.NewMemoryStore(), // optional: immediate revocation
}
```

Without a `Store`, the session is stateless: it is valid on any replica and needs no
database, but only truly ends when it expires. With a `Store`, the cookie carries an
identifier and the logout deletes the record right away — that is what you want when you
need to cut someone off now. The identifier changes on every login, so a cookie planted
earlier does not become a valid session.

## What `auth` refuses

Every item here is a known attack, and all of them have a test in the suite:

- **the token's `alg`**: the list is fixed (RS256/384/512, ES256/384). Reading the algorithm
  from the token and obeying it is how JWT libraries get broken — `alg: none` gets through,
  or an RSA public key becomes an HMAC secret.
- **`state`**: without it the callback can be forged by another site (CSRF on login).
- **`nonce`**: ties the `id_token` to *this* request, against replay.
- **PKCE (S256)**: a code stolen along the way is useless without the verifier.
- **`iss`, `aud`, `exp`, `nbf`**: a legitimate token issued for another app or by another
  tenant is not valid here. Clock tolerance is 60 seconds.
- **`next`**: only a path inside the app. `//evil.example` and `https://evil.example`
  become `/` — an open redirect is the classic way to lend credibility to phishing.
- **unknown key**: the JWKS is fetched again when the provider rotates the key, but at most
  once per minute, so a forged token does not turn into one network request per HTTP
  request.

Every refusal becomes a `SecurityEvent` of kind `auth` and lands in
`trilha_security_events_total`, the counter from the
[observability chapter](/learn/observability).

## Challenge

Your app needs a route that only works for people who signed in **within the last five
minutes** — a recent re-authentication before a sensitive operation, such as rotating an API
key.

:::solution
```go
func recent(c *trilha.Ctx, next trilha.Next) error {
	u := flow.User(c)
	if u == nil || time.Since(u.IssuedAt) > 5*time.Minute {
		return trilha.RedirectCode("/login?next="+url.QueryEscape(c.Request().URL.Path), 302)
	}
	return next()
}
```
`IssuedAt` is the moment of the login, and `Start` creates a new session on every round
trip through the provider — so someone already signed in only has to pass through the
provider's screen again, which usually lets them through without asking for the password.
To force typing it, add `prompt=login` to the authorization parameters.
:::
