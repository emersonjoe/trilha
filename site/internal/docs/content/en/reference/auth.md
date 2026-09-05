---
title: auth
description: Provider, Options, Auth, User and Store — the API of the auth package, with the defaults and what each field changes.
---

`import "github.com/emersonjoe/trilha/auth"` — OpenID Connect login with the standard
library. The package registers no route: it exposes handlers that your `app/` publishes.

## Providers

```go
func OIDC(issuer, clientID, clientSecret, redirectURL string) *Provider
func EntraID(tenant, clientID, clientSecret, redirectURL string) *Provider
func Keycloak(baseURL, realm, clientID, clientSecret, redirectURL string) *Provider
```

| Constructor | Resulting issuer | Roles read from |
|---|---|---|
| `OIDC` | whatever you pass | `roles`, `groups` |
| `EntraID` | `https://login.microsoftonline.com/<tenant>/v2.0` | `roles`, `groups`, `wids` |
| `Keycloak` | `<baseURL>/realms/<realm>` | `realm_access.roles`, `resource_access[clientID].roles` |

AWS Cognito (`https://cognito-idp.<region>.amazonaws.com/<user-pool-id>`) and Clerk (the
Frontend API URL) have no shortcut yet ([#41](https://github.com/emersonjoe/trilha/issues/41)):
use `OIDC` and name their role claim in `Options.RoleClaims` (`cognito:groups` for Cognito).

`Provider.HTTPClient` swaps the HTTP client (default: 10 s timeout). Discovery happens on
first use and is valid for one hour; an issuer that differs between the configuration and
the document is an error, not a warning.

## Options

| Field | Default | What it does |
|---|---|---|
| `Scopes []string` | `openid profile email` | scopes requested from the provider |
| `Absolute time.Duration` | 8 h | maximum session lifetime, counted from the login |
| `Idle time.Duration` | 30 min | ends an idle session; `IdleOff: true` disables it |
| `CookieName string` | `trilha_session` | session cookie name |
| `LoginPath string` | `/entrar` | where `Require` sends an anonymous browser |
| `AfterLogin string` | `/` | destination after the callback, when there is no `next` |
| `AfterLogout string` | `/` | destination after the logout |
| `RoleClaims []string` | — | additional claims to read roles from |
| `Store Store` | `nil` | persists the session; `nil` = signed cookie, stateless |

## Auth

```go
func New(p *Provider, o Options) *Auth      // no network
func (a *Auth) Start(c *trilha.Ctx) error   // → provider (PKCE, state, nonce)
func (a *Auth) Callback(c *trilha.Ctx) error // validates the callback and creates the session
func (a *Auth) Logout(c *trilha.Ctx) error   // deletes the session; RP-Initiated Logout when available
func (a *Auth) Require() trilha.MiddlewareFunc
func (a *Auth) RequireRole(roles ...string) trilha.MiddlewareFunc
func (a *Auth) Optional() trilha.MiddlewareFunc
func (a *Auth) User(c *trilha.Ctx) *User     // nil when anonymous
func (a *Auth) Session(c *trilha.Ctx) (*User, error)
```

`Require` answers **302** to the login when the request is a navigation (Accept with
`text/html`, outside `/api/`) and **401** otherwise. `RequireRole` answers **403** to
someone authenticated without the role. **One** of the listed roles is enough; the
comparison ignores case.

## User

```go
type User struct {
	Subject   string    // sub: the stable identifier
	Email     string    // email, or preferred_username when there is none
	Name      string
	Roles     []string
	IssuedAt  time.Time // moment of the login
	ExpiresAt time.Time
	Seen      time.Time // last activity (idle window)
	SessionID string    // changes on every login
}

func (u *User) HasRole(role string) bool
```

## Store

```go
type Store interface {
	Save(id string, u *User, ttl time.Duration) error
	Load(id string) (*User, bool)
	Delete(id string) error
}

func NewMemoryStore() *MemoryStore
```

With a `Store` the cookie carries only the identifier and the logout takes effect
immediately for everyone. `MemoryStore` is for a single process: replicas do not share it,
and a restart drops every session. For several replicas, implement the interface over your
database or cache.

## Cookies

| Cookie | Lifetime | Content |
|---|---|---|
| `trilha_oidc_state` | 10 min | `state` of the request in progress |
| `trilha_oidc_nonce` | 10 min | `nonce` of the request in progress |
| `trilha_oidc_verifier` | 10 min | PKCE verifier |
| `trilha_oidc_next` | 10 min | destination after the login (relative path only) |
| `trilha_session` | `Absolute` | the session (or its id, with a `Store`) |

All are signed (they require `TRILHA_SECRET`), `HttpOnly`, `SameSite=Lax` and `Secure`
under HTTPS. The four flow cookies are deleted on the callback, whether it succeeds or not.

## Accepted algorithms

`RS256`, `RS384`, `RS512`, `ES256`, `ES384`. The list is fixed: the token's `alg` chooses
nothing. RSA keys with a modulus smaller than 2048 bits are ignored in the JWKS, `kid` is
required, and clock tolerance is 60 seconds.

## Audit

`trilha audit` checks, when the project imports `trilha/auth`: client secret written in the
code (critical) and `redirect_uri` over `http://` outside `localhost` (critical).
