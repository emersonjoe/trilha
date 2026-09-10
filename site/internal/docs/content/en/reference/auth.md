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

Those claims are read from the `id_token` **and from the access token**, when the access
token happens to be a JWT from the same issuer — the `id_token` wins wherever both carry the
same claim. This is not a nicety: **Keycloak puts `realm_access` and `resource_access` in the
access token only**, so reading the `id_token` alone gives a login that works and a `Roles`
that is empty, and every `RequireRole` answers 403 for no visible reason. An opaque access
token — what most providers issue — changes nothing; one that does not verify grants nothing.

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
func (a *Auth) LoginPath() string            // Options.LoginPath, or its default
func (a *Auth) Sessions(c *trilha.Ctx) ([]User, error) // every session of whoever is logged in, this one first
func (a *Auth) LogoutOthers(c *trilha.Ctx) error       // ends the others, keeps this one

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

### Sessions by owner

```go
type SessionLister interface {
	Sessions(subject string) []*User // the live sessions of one subject
}

var ErrNoSessionList error
```

A store that also implements `SessionLister` gives the account screen two things:
`Sessions(c)` — every session of whoever is logged in, the current one first, each with
`IssuedAt`, `Seen` and `SessionID` — and `LogoutOthers(c)`, which deletes every session of
theirs except this one and writes `auth.logout_others` to the audit trail with the count.
`MemoryStore` implements it. A store that does not gets `ErrNoSessionList` from both, which
is what the screen shows instead of an empty list: an empty list would say "nowhere else",
and that would not be known.

A password change is where `LogoutOthers` belongs: a password changes because somebody may
have the old one, and that somebody may be signed in right now.

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

## API keys

`auth.Sessions` is a cookie and OIDC is somebody else's bearer. `auth.APIKeys` is the third one:
the key **this** application issues to its own callers. It is a pile of rules a beginner does not
know — store only the hash, show the secret once, keep a handle so a key can be found without
opening the hash, a scope per route, a limit per key rather than per address, revocation that
takes effect now, a record of use — and the first version always stores the key in the clear.

```go
var Keys = auth.APIKeys(auth.KeyOptions{
	Store:     chaves.NewStore(db),        // nil keeps them in memory
	Prefix:    "ak",                       // keys read "ak_<handle>_<secret>"
	Scopes:    []string{"docs:read", "docs:write"},
	RateLimit: trilha.RateLimit{RPS: 10, Burst: 30},
})

// app/api/v1/middleware.go
var exige = Keys.Require("docs:read")
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
```

| Symbol | Role |
|---|---|
| `APIKeys(KeyOptions)` | the set of keys of an application |
| `Issue(c, name, scopes, ttl)` | creates one and answers the secret — the only time it exists |
| `Require(scopes...)` | the middleware: bearer, hash, revocation, expiry, scope, limit |
| `Revoke(c, id)` / `All()` | ends one now; lists them for the screen |
| `User(c)` | the caller, as an `auth.User` with the scopes as roles |
| `KeyStore` / `MemoryKeyStore()` | five methods over whatever the app runs; memory for tests |
| `ui.SecretOnce(c, secret)` | the card that shows it once, with the sentence that has to be there |
| `ui.APIKeysTable(c, rows, opts)` | the list, with the handle and never the key |

**Only the hash is stored**, peppered with `trilha.Pepper` — HMAC-SHA256 under a key derived
from the app's secret. A stolen table of digests is not a list anybody can attack offline, and
without a secret `Issue` refuses rather than writing an unkeyed hash that would look like it
worked.

**The handle is the half that identifies without opening the hash.** It is what a screen shows
and how a request finds its key; the secret is compared in constant time, and revocation and
expiry are checked *after* that comparison — answering faster for a revoked key than for a wrong
one says which of the two happened.

**A key is an actor.** `Require` puts an `auth.User` in the request with the scopes as roles and
`via: "api_key"`, so `c.Audit`, the log and the policy all see a caller instead of a hole.

**The limit is per key**, not per address: one caller behind one key is one budget, whatever
their IP is doing. **Use is recorded once a minute** — writing on every request turns a read-only
endpoint into a write per call, and "is this key still in use?" does not need the second.

**A scope the application never declared is a panic at wiring time.** The alternative is a route
that guards nothing because of a typo, and nobody finding out until they read it in a breach
report.

### Who is using this key, where, and when did they stop

That is the first question after a key is handed to a partner, and it is not a security question.
The key already knows *who* called and when it was last seen; what `KeyOptions.Usage` adds is the
shape of the use.

```go
var Chaves = auth.APIKeys(auth.KeyOptions{
	Usage:     auth.UsageMemory(),          // or a table behind the same three methods
	UsageKeep: 400 * 24 * time.Hour,        // retention, swept on the same loop
})

func main() { … ; if err := Chaves.Setup(a); err != nil { … } ; … }
```

```go
rel, _ := Chaves.Usage(c.Context(), auth.UsageQuery{Key: id, Since: time.Now().AddDate(0, 0, -30)})
rel.Total, rel.Errors, rel.Last
rel.ByRoute   // []UsageRoute{Method, Route, Count, Errors, Last}, busiest first
rel.ByDay     // []UsageDay, oldest first — what a chart draws
rel.ByKey     // []UsageKeyTotal, when the query was not about one key
```

**Counting must not cost the request.** `Require` increments a bucket in memory — one map write
under a mutex, nothing that can block on a network — and the buckets go to the store in batches.
`Setup` flushes on a timer and on shutdown; without it, `Flush` is yours to call. A store that
fails on a flush gets its counts back in the buffer, because losing them because a database was
restarting is the exact failure this design is avoiding.

**The route is the pattern.** `/documents/{id}`, never `/documents/8f2c…`: a counter per concrete
path is a counter with one row per request. A request the fallback answered has no pattern, and it
is not counted — the address there is user input, and whoever asks for enough addresses that do
not exist would be writing rows.

**Without `Usage`, nothing changes**, byte for byte. `Keys.Usage` on such an application answers
`ErrNoUsage` rather than an empty report: no data and no counting are different answers.

**A revoked key keeps its history.** "Who was using this?" arrives after the revocation, not
before.

```go
ociosas, _ := Chaves.Idle(c.Context(), time.Now().AddDate(0, 0, -90))
```

`Idle` is the keys with no recorded call since that moment. The issue that asked for this wanted it
in `trilha audit`; it is here instead, because that command reads code on somebody's laptop and the
counters live in production — a check that cannot see them is a check that always says everything
is fine.

The screens are [`ui.APIUsage`](/reference/ui) and the calls column of `ui.APIKeysTable`, and
`trilha add api-keys` writes both.

## Multi-tenant by column

One column is the most common shape of multi-tenant, and forgetting that column in one query is
the most common bug of multi-tenant: the report that shows another organisation's rows, found by
a customer.

**The framework carries the value and points at the query that forgot it. The query is yours.**
There is no ORM here, and a `WHERE` this package generated would be a `WHERE` nobody could read
in a review — which is the opposite of what a tenant filter needs.

```go
// at login, or in OnLogin
u.Tenant = row.TenantID

// in a repository
rows, err := db.QueryContext(c, `SELECT … FROM documents WHERE tenant_id = $1`, auth.Tenant(c))
```

| Symbol | Role |
|---|---|
| `auth.User.Tenant` | a field of its own, next to Roles; travels wherever the session travels |
| `auth.Tenant(c)` | the organisation of the current session, or "" |
| `sso.RequireTenant()` | refuses a session with no organisation chosen |
| `Options.ChooseTenantPath` | where a browser goes to pick one; empty answers 403 |
| `sso.SwitchTenant(c, id)` | moves the session and writes both sides down |

It is a **field and not one more entry in `Extra`** because everything the framework does with it
has to find it in the same place in every application: it goes into the audit trail as
`actor.tenant` and onto the access record as `tenant` — which is the first filter of any support
question, and the field that says whether "they saw the wrong rows" is about a query or about
somebody having changed organisation.

`RequireTenant` treats "logged in, no organisation" as what it is: an administrator who has not
picked one, or a first login. A browser is redirected; anything else gets 403, because a redirect
to a screen is not an answer an API can use.

**Checking that somebody may enter an organisation is the application's job.** `SwitchTenant`
moves the session and audits it; it does not know what a membership is, and pretending to would
be a check that looks like a guarantee and is not one.

### The query that forgot

`trilha audit` counts, per table, how many queries filter it by tenant — and names the ones that
do not:

```
warn  queries that may be missing the tenant filter
      internal/docs/repo.go:88: documents is filtered by tenant in 7 of 8 queries, and not in this one
```

It is a text heuristic and says so: no SQL parser, no verdict, a place to look. A query that is
right on purpose — a global report, an admin listing — is worth a comment saying so, for the next
person as much as for the tool.
