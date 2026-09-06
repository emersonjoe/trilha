---
title: auth
description: Provider, Options, Auth, User and Store — the API of the auth package, with the defaults and what each field changes.
---

`import "github.com/emersonjoe/trilha/auth"` — login with the standard library, through a
provider (OpenID Connect) or against the app's own users table. The package registers no
route: it exposes handlers that your `app/` publishes.

## Providers

```go
func OIDC(issuer, clientID, clientSecret, redirectURL string) *Provider
func EntraID(tenant, clientID, clientSecret, redirectURL string) *Provider
func Keycloak(baseURL, realm, clientID, clientSecret, redirectURL string) *Provider
func Cognito(region, userPoolID, clientID, clientSecret, redirectURL string) *Provider
func Clerk(frontendAPI, clientID, clientSecret, redirectURL string) *Provider
```

| Constructor | Resulting issuer | Roles read from |
|---|---|---|
| `OIDC` | whatever you pass | `roles`, `groups` |
| `EntraID` | `https://login.microsoftonline.com/<tenant>/v2.0` | `roles`, `groups`, `wids` |
| `Keycloak` | `<baseURL>/realms/<realm>` | `realm_access.roles`, `resource_access[clientID].roles` |
| `Cognito` | `https://cognito-idp.<region>.amazonaws.com/<userPoolID>` | `cognito:groups` |
| `Clerk` | the Frontend API URL, normalized (`https://<slug>.clerk.accounts.dev`) | `roles`, `groups` — Clerk's `id_token` carries the organization (`org_id`), not the role in it; a configured claim goes in `Options.RoleClaims` |

`Provider.LogoutDomain` exists for Cognito: set it to the managed login domain
(`<prefix>.auth.<region>.amazoncognito.com`, or your own) and `Logout` redirects to
`/logout?client_id=…&logout_uri=…` there; the return URL must be in the app client's
*Allowed sign-out URLs*. Left empty, `Logout` clears the local session, says so in the log
and does not pretend it federated. Other providers ignore the field. Clerk publishes no
`end_session_endpoint` either, and has no equivalent address: there `Logout` is always local,
and the log says the Clerk session was left open.

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
| `OnLogin func(c, *User) error` | — | runs inside `Login` and `Callback`, session not yet written; its error stops the login |

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

func Sessions(o Options) *Auth               // the same type, without a provider
func (a *Auth) Login(c *trilha.Ctx, u *User) error // session for a user the app authenticated
func (a *Auth) RequireFunc(pred func(*User, *trilha.Ctx) bool) trilha.MiddlewareFunc
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
	Extra map[string]string // what OIDC has no claim for (the API token, the tenant)
}

func (u *User) HasRole(role string) bool
```

## Session without OIDC

An app whose users are a table of its own — e-mail, password hash, role — builds the same
`*Auth` with `Sessions`, and says who the person is itself:

```go
sessions := auth.Sessions(auth.Options{Store: store, LoginPath: "/login"})

// app/login/page.go
func POST(c *trilha.Ctx) error {
	u, err := users.Verify(c.Form("email"), c.Form("password"))
	if err != nil {
		return c.Render(422, form(c, "wrong e-mail or password"))
	}
	return sessions.Login(c, &auth.User{Subject: u.ID, Email: u.Email,
		Roles: []string{u.Role}, Extra: map[string]string{"api_token": u.JWT}})
}
```

`Login` redirects to the `next` it was asked for, or to `AfterLogin`. `Start` and
`Callback` answer a clear error on an `Auth` with no provider; `Logout` clears the session
and lands, since there is nothing above this app to end.

`User.Extra` is what this session carries and a claim cannot express — the token an
[upstream](/reference/upstreams) injects, the tenant. It travels where the rest of the
session travels: keep it small, and never put a password in it.

### Passwords

```go
func HashPBKDF2(password string) (string, error)  // pbkdf2_sha256$600000$salt$hash
func CheckPBKDF2(encoded, password string) bool   // constant-time; false on a hash it cannot read
func PBKDF2(password, salt []byte, iter, keyLen int) []byte
```

The format is the one Django and `hashlib.pbkdf2_hmac` write, so an existing users table
stays valid without a password migration — the iteration count travels inside each hash, so
raising `DefaultPBKDF2Iterations` locks nobody out. `bcrypt` and `argon2` are better at this
and neither is in the standard library, which is the whole reason this one is here.

### Authorization the roles do not express

A matrix of module and level, a tenant, the owner of a record: all the same shape, and none
of them a list of role names.

```go
// app/painel/middleware.go
func Middleware(c *trilha.Ctx, next trilha.Next) error {
	return sessions.RequireFunc(func(u *auth.User, c *trilha.Ctx) bool {
		return u.Extra["tenant"] == c.Param("tenant")
	})(c, next)
}
```

Anonymous never reaches the predicate: it is sent to the login first, as `Require` does.
Hiding the menu item is cosmetic — the rule is the middleware.

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
