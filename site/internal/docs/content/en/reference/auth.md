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

## The permission matrix

A role list answers "is this person an admin". What an application actually asks is "may this
person edit documents", and the answer is a matrix: role × module × level.

```go
var Policy = auth.Policy{
	Modules: []string{"docs", "processes", "hr"},
	Levels:  auth.Levels{"view", "edit", "manage"},   // ordered: manage ⊇ edit ⊇ view
	Roles: map[string]auth.Grants{
		"admin":   auth.All("manage"),
		"analyst": {"docs": "edit", "processes": "view"},
		"reader":  auth.All("view"),
	},
}
```

The order of `Levels` is the whole meaning: `manage` covers `edit` covers `view`, so a cell
holds one value instead of three booleans. A module a role does not name is a module it cannot
reach — the absence is a denial, never an inheritance — and `Default` is what a signed-in user
with no known role may do, which is nothing until you say otherwise. A role that was deleted
should lose access, not inherit somebody else's.

| Symbol | What it does |
|---|---|
| `Policy.Can(u, module, level) bool` | may this user do this |
| `Policy.Level(u, module) string` | what they have, or `""` |
| `(*Auth) RequirePolicy(p, module, level)` | the guard |
| `auth.All(level) Grants` | the same level on every module |
| `auth.BindPolicy(c, p)` | read back what the grid posted |
| `auth.PolicyStore`, `auth.PolicyFrom` | a matrix people edit |
| `ui.PolicyGrid(p, opts)` | the screen that edits it |

### The guard

```go
// app/docs/middleware.go
var see = acesso.Auth.RequirePolicy(acesso.Policy, "docs", "view")

func Middleware(c *trilha.Ctx, next trilha.Next) error { return see(c, next) }
```

Two lines and not one: `middleware.go` has to export a *function* with that signature, and a
var of the right type is not one. `MiddlewarePOST` guards a single method, so a folder can be
readable by one level and writable by another.

Anonymous goes to the login. A signed-in user who is not allowed gets **403** — they are known,
just not permitted, and sending them to the login would loop — and the message says what was
missing: `needs edit on docs`. That sentence is what the person repeats to whoever administers
the application; a bare "forbidden" turns a two-minute fix into a support thread.

### Hiding a button is not a rule

```go
if acesso.Policy.Can(acesso.Auth.User(c), "docs", "edit") { … }
```

Correct and cosmetic. The rule that holds is the middleware, because a hidden button is still
an address somebody can type.

`trilha audit` warns when the policy declares a module and no route requires it: the matrix
says the area is protected, and if nothing asks, the protection is a sentence in a file.

### A matrix people edit

`Policy` is data in the code, which is where it belongs when only deploys change it. An
application whose administrators invent roles keeps the roles in a table:

```go
type PolicyStore interface {
	Load(ctx context.Context) (map[string]Grants, error)
	Save(ctx context.Context, roles map[string]Grants) error
}

policy, err := auth.PolicyFrom(ctx, store, defaults)   // empty store keeps the defaults
```

`PolicyFrom` is a snapshot on purpose: a `Policy` is a value, and a value that changed
underneath a request would let one request answer twice — allowed at the middleware, denied at
the button. Call it again after saving.

The screen comes ready:

```go
ui.PolicyGrid(policy, ui.PolicyGridOpts{
	Action: "/admin/permissions",
	CSRF:   trilha.CSRFInput(c),
	Labels: map[string]string{"docs": "Documents"},
})
```

One row per role, one column per module, a select of levels per cell, fields named
`grant.<role>.<module>` — which is what `auth.BindPolicy` reads on the other side. No
JavaScript: it is a form, it posts, the handler saves and redirects. A cell naming a module or
a level the policy does not declare is dropped in silence, because answering 400 would only
tell whoever forged it which name to try next.

Guard that screen with the module that administers the application. It is the most valuable
screen there is.

### What this does not express

A rule about one record — the owner of a document, a row of a tenant — stays `RequireFunc`,
written by hand. A policy that reached into the data would need the data, and then it would be
a query and not a declaration.

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

`CheckPBKDF2` reads two spellings, told apart by the prefix, because the table it has to keep
working is not always Django's:

| Written by | Shape |
|---|---|
| Django, passlib | `pbkdf2_sha256$<iterations>$<salt>$<digest base64>` |
| `hashlib`, by hand | `pbkdf2$<iterations>$<salt hex>$<digest hex>` |

The second one exists because `hashlib.pbkdf2_hmac` returns bytes and no format at all, so an
app that uses neither Django nor passlib picks one, and hex is what it picks. There the salt is
hex that was decoded to bytes before the derivation, which is why reading it as text gives the
wrong answer for the right password. SHA-256 only; `pbkdf2_sha512` is `false`.

`HashPBKDF2` keeps writing the first: one spelling to write, two to read. So an existing users
table stays valid without a password migration — the iteration count travels inside each hash,
so raising `DefaultPBKDF2Iterations` locks nobody out. `bcrypt` and `argon2` are better at this
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
