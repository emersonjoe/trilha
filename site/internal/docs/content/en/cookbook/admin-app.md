---
title: The management app in one afternoon
description: trilha new --template app writes a whole administrable application — login, users, permissions, profile, audit, API keys, settings, organisations. This is the afternoon from the command to production.
---

Every internal application is the same application before it is yours: somebody signs in,
somebody else gets invited, a role decides what each of them may open, an account screen, a
trail of who did what, a key for the script that calls the API, a settings screen so a value
does not need a deploy, and — sooner than anybody plans for — a second organisation.

That is three weeks of work that nobody ever remembers deciding to do. `trilha new --template
app` writes it, and what it writes is not a special template: it is the same
[`trilha add` recipes](/reference/cli#trilha-add) you would run one by one, applied at
creation. This page is the afternoon that follows — the command, the first login, the first
role, your own entity, the second organisation, and production.

## 1. The command

```bash
trilha new empresa --template app
cd empresa
```

```text
  + app/admin/middleware.go
  + app/error.go
  + app/items/id_/page.go
  + app/items/new/page.go
  + app/items/page.go
  + app/layout.go
  + app/middleware.go
  + app/not_found.go
  + app/page.go
  + app/setup.go
  + app_test.go
  + internal/store/store.go
  + public/style.css
  + .gitignore
  + go.mod
  + public/ui.theme.css
  + public/ui.css
  + public/ui.js
  + public/ui.nav.js
  + public/ui.upload.js
  + public/ui.live.js
  + public/ui.chat.js
  + public/ui.island.js
  + public/ui.tree.js
  + internal/usuarios/usuarios.go
  + internal/usuarios/usuarios_test.go
  + internal/sessao/sessao.go
  + app/entrar/page.go
  + app/sair/route.go
  + internal/sessao/sessaotest/sessaotest.go
  + login_test.go
  + internal/auditoria/store.go
  + app/admin/auditoria/page.go
  + app/admin/chaves/page.go
  + internal/config/config.go
  + app/admin/config/page.go
  + internal/usuarios/convites.go
  + internal/usuarios/convites_test.go
  + internal/usuarios/papeis.go
  + app/admin/usuarios/page.go
  + app/admin/usuarios/middleware.go
  + app/convite/token_/page.go
  + usuarios_test.go
  + internal/acesso/acesso.go
  + internal/acesso/acesso_test.go
  + app/admin/permissoes/page.go
  + app/admin/permissoes/middleware.go
  + permissoes_test.go
  + internal/usuarios/perfil.go
  + internal/usuarios/perfil_test.go
  + app/perfil/page.go
  + app/perfil/middleware.go
  + app/perfil/email/token_/route.go
  + perfil_test.go
  + internal/organizacoes/organizacoes.go
  + internal/organizacoes/organizacoes_test.go
  + app/organizacoes/page.go
  + app/organizacoes/middleware.go
  + organizacoes_test.go

✓ project created in empresa

  cd empresa
  trilha dev
```

The list is in two halves, and telling them apart is worth the minute it takes:

```text
empresa/
├── app/
│   ├── middleware.go        ← the session guard over the whole tree: this is what keeps people out
│   ├── layout.go            ← ui.Shell: sidebar, top bar, user menu — the menu is yours to edit
│   ├── page.go              ← the dashboard, drawn over the demo store
│   ├── items/               ← the demo listing. Delete it when your own entity arrives
│   ├── entrar/, sair/       ← login recipe
│   ├── perfil/, convite/    ← profile and users recipes
│   ├── organizacoes/        ← tenant recipe
│   └── admin/               ← one folder per administration screen, behind admin/middleware.go
├── internal/
│   ├── store/store.go       ← the demo's in-memory store (not the SQL one; see step 5)
│   ├── usuarios/            ← the users table, invitations, roles, profile
│   ├── sessao/              ← auth.Sessions and the test helper
│   ├── acesso/              ← the permission matrix, as data
│   ├── auditoria/, config/  ← the audit trail and the settings section
│   └── organizacoes/        ← organisations and who belongs to which
├── *_test.go                ← one integration test per recipe, at the root, over the real app
└── trilha_gen.go            ← generated; commit it, never edit it
```

Everything from `internal/usuarios/usuarios.go` down in the output above came from a recipe,
and `app/setup.go` says so — the ones with something to wire left a marked line where they
wired it:

```text
// trilha:add tenant
// trilha:link users-permissions
// trilha:add settings
// trilha:add api-keys
// trilha:add audit
// trilha:add login
```

Those markers are how the next `trilha add` finds its place, so leave them where they are; the
code around them is yours.

:::note
The generated folders and identifiers are in Portuguese (`entrar` is "sign in", `usuarios` is
"users", `acesso` is "access", `organizacoes` is "organisations") — the same choice the
[example apps](/learn/examples) make. `--lang en` translates the on-screen texts; the paths
stay. They are your project's now: rename them on day one if you would rather read them in
English.
:::

## 2. The first run

Three environment variables and nothing else:

```bash
export TRILHA_SECRET=$(trilha secret)      # signs the session cookie
export ADMIN_EMAIL=ana@empresa.com         # the first user
export ADMIN_PASSWORD=a-password-nobody-guesses
trilha dev
```

`TRILHA_SECRET` is not optional here: the session is a signed cookie, and without a key the
login refuses to keep its state rather than pretending to. `trilha secret` prints one.

The first administrator is seeded from the environment at boot, once, with the role `admin` —
an application whose first user is a password in a source file is an application with a door
somebody forgets. With the two variables unset the app still starts, the login screen still
answers, and it says so, on screen and in the log:

```text
WARN usuarios: nobody can sign in yet fix="set ADMIN_EMAIL and ADMIN_PASSWORD and restart"
```

Open `http://localhost:3000`, get redirected to `/entrar`, sign in, and every screen of the
list below answers 200. The first thing to do from inside is `/organizacoes`: create the
organisation, which puts it in your session and turns on everything in step 6.

:::warning
**Every store in a fresh project is in memory.** Users, audit lines, API keys, settings,
organisations — all of it is lost on restart, and the log says so once per section. That is
deliberate: the skeleton runs with no database on the first afternoon, and each module takes a
real store when you have one (`trilha add store`, then the `Store`/`SettingsStore` field each
package documents). Do not put it in front of a customer before that.
:::

### The e-mail nobody vouched for

`auth.Options.RequireVerifiedEmail` belongs to a day that has not arrived yet in this project.
It reads the provider's `email_verified` claim, so it has something to read only when there is
a provider; the login this template ships is your own table, and the address in it is whatever
the invitation was sent to. The local equivalent is already on the profile screen: changing an
e-mail sends a link to the *new* address, and only the route that link opens changes the row.

The day the login becomes a provider's, that option is the switch, and it refuses **before**
`OnLogin` — so a rule of your own never runs on an address nobody vouched for:

```go
func AdminSSO() *auth.Auth {
	p := auth.OIDC(
		os.Getenv("OIDC_ISSUER"),
		os.Getenv("OIDC_CLIENT_ID"),
		os.Getenv("OIDC_CLIENT_SECRET"),
		"https://empresa.example.com/auth/callback",
	)
	return auth.New(p, auth.Options{
		Store:                auth.NewMemoryStore(),
		RequireVerifiedEmail: true,
		LoginPath:            "/entrar",
		AfterLogin:           "/",
	})
}
```

The screens do not change: they ask `sessao.Atual(c)` who is there, and that answer has the
same shape either way. [Authentication](/learn/authentication) is the whole story.

## 3. Permissions, as data

`internal/acesso/acesso.go` is the matrix: modules are what the application protects, levels
are ordered and each one implies the ones below, and a role is a row. Adding your first module
is one line in one file:

```go
var AdminPolicy = auth.Policy{
	Modules: []string{"relatorios", "usuarios", "pedidos"},
	Levels:  auth.Levels{"ver", "editar", "administrar"},
	Roles: map[string]auth.Grants{
		"admin":     auth.All("administrar"),
		"comercial": {"pedidos": "editar", "relatorios": "ver"},
		"leitor":    {"relatorios": "ver"},
	},
}
```

`/admin/permissoes` is that value on a screen: `ui.PolicyGrid` draws a role per row, a module
per column and a level in each cell; below the grid are the roles themselves — create one,
remove one, see who has each — and below that, what the person looking at the page may do.
The screen that edits the matrix is guarded by the matrix (`usuarios`/`administrar`), not by a
role written into its middleware: an exception living outside the matrix is an exception
nobody sees on the grid.

A folder of your own asks for a level the same way:

```go
var adminOrders = adminSessions.RequirePolicy(AdminPolicy, "pedidos", "editar")
```

```go
func AdminOrdersMiddleware(c *trilha.Ctx, next trilha.Next) error { return adminOrders(c, next) }
```

A level is about a module, so the rules a level cannot express — the owner of a record, the
rows of one organisation — stay a function, deliberately:

```go
var AdminOwnRows = adminSessions.RequireFunc(func(u *auth.User, c *trilha.Ctx) bool {
	return u.Tenant != "" && u.Tenant == c.Param("org")
})
```

Everything else in the application asks the matrix and never the role name, which is why a new
role is a row on a screen and not a deploy. [`auth`](/reference/auth) has the details.

## 4. The screens that came with it

| Screen | URL | Recipe | What it is |
|---|---|---|---|
| Sign in / out | `/entrar`, `POST /sair` | `login` | your own users table, PBKDF2, session with 30 min idle |
| My account | `/perfil` | `profile` | name, password, e-mail confirmed at the new address, and the list of open sessions with "end the others" (`LogoutOthers`) |
| Users | `/admin/usuarios`, `/convite/{token}` | `users` | invite (no password: the person sets it at the link), role, deactivate, reset |
| Permissions | `/admin/permissoes` | `permissions` | the grid above, the roles below it |
| Audit | `/admin/auditoria` | `audit` | who did what, on what, and from where — `c.Audit(...)` is one line and the screen reads it |
| API keys | `/admin/chaves` | `api-keys` | issue (shown once), revoke, scopes — and usage per key, route and day over the last 30 |
| Settings | `/admin/config` | `settings` | a struct with tags is the screen; the defaults are what the app runs on before anybody saves |
| Organisations | `/organizacoes` | `tenant` | create, switch, activate, members, and per-organisation settings |

Changing a password ends every other session, including the one that did it: a password is
changed because somebody may have the old one, and that somebody may be signed in right now.

Four recipes exist and are **not** in this template. They are one command away, and each one
lands its screens where you point it:

```bash
trilha add approvals    # a queue that waits for a person: open, decide, deadline
trilha add search       # one box over several kinds of thing, grouped by kind
trilha add connections  # external services: name, URL, sealed secret, and a Test button
trilha add share-link   # a signed link with a deadline, for somebody with no account
```

A recipe that needs another one refuses and says which (`Needs`), so ordering them wrong costs
a message and not a broken project.

### The menu does not write itself

`app/layout.go` is yours, so no recipe edits it — which means the screens above exist and are
not in the sidebar yet. Paste them:

```go
func AdminNav(admin bool) []ui.NavGroup {
	return []ui.NavGroup{
		{Label: "Work", Items: []ui.NavItem{
			{Href: "/", Label: "Dashboard", Icon: "house"},
			{Href: "/admin/pedidos", Label: "Orders", Icon: "search"},
			{Href: "/perfil", Label: "My account", Icon: "user"},
			{Href: "/organizacoes", Label: "Organisations", Icon: "building"},
		}},
		{Label: "Admin", Hide: !admin, Items: []ui.NavItem{
			{Href: "/admin/usuarios", Label: "Users", Icon: "users"},
			{Href: "/admin/permissoes", Label: "Permissions", Icon: "lock"},
			{Href: "/admin/auditoria", Label: "Audit", Icon: "list"},
			{Href: "/admin/chaves", Label: "API keys", Icon: "key"},
			{Href: "/admin/config", Label: "Settings", Icon: "settings"},
		}},
	}
}
```

`Hide` is cosmetics. What keeps somebody out of `/admin` is `app/admin/middleware.go`; hiding
the link only saves them the 403.

## 5. Your own entity

Write the struct first — the tags are the screen — then generate the three pages around it:

```bash
trilha generate crud pedidos.Pedido --at app/admin/pedidos
```

```text
  + internal/pedidos/pedido_store.go
  + app/admin/pedidos/page.go
  + app/admin/pedidos/new/page.go
  + app/admin/pedidos/id_/page.go
  + pedido_crud_test.go
  + app/setup.go
  app/admin/middleware.go closes this folder: the generated test opens a session first, with empresa/internal/sessao/sessaotest

✓ /admin/pedidos answers now; trilha_gen.go is up to date

✓ /admin/pedidos/new answers now; trilha_gen.go is up to date

✓ /admin/pedidos/{id} answers now; trilha_gen.go is up to date
```

Two things in that output are the template paying off. The destination is inside `app/admin/`,
so the generated test opens a session with the helper the login recipe wrote instead of
asserting a 401 nobody wanted. And `app/setup.go` gained the line that provides the store —
an interface, with an in-memory implementation, so the screens do not change the day SQL
arrives. Add the entity to the menu (above) and the afternoon has a working application:

```bash
trilha check
```

```text
✓ gen
✓ gofmt
✓ vet
✓ test
✓ audit
– openapi (skipped)
ok
```

### And with a database

`--store postgres` (or `sqlite`) writes the SQL implementation of that same interface plus a
migration, and it needs the [`store`](/reference/store) recipe, which owns `internal/store`:

```bash
trilha add store
trilha generate crud pedidos.Pedido --store postgres --at app/admin/pedidos
```

Two things to know before running the first of those in *this* template:

- `internal/store/store.go` is already taken by the demo listing's in-memory store. Retire the
  demo before the recipe arrives — `rm -rf app/items internal/store/store.go app_test.go`,
  drop the `store.New()` lines from `app/setup.go` and point the dashboard at your own entity
  — or the two packages collide and `go vet` is what tells you.
- From then on the app needs `DATABASE_URL` to boot, tests included, because `store.Setup`
  opens the pool and applies the migrations before the first request. Pick a driver and blank
  import it in `internal/store/driver.go`; [Database](/cookbook/database) is the rest.

## 6. The second organisation

Multi-tenant by one column is the common shape, and forgetting that column in one query out of
forty is its common bug: the report that shows somebody else's rows, found by a customer.

The framework carries the organisation in the session and refuses a session without one. The
query is yours:

```go
func AdminOrdersOf(c *trilha.Ctx, db *sql.DB) ([]AdminOrder, error) {
	rows, err := db.QueryContext(c.Context(), `
		SELECT id, cliente, total
		  FROM pedidos
		 WHERE org_id = $1
		 ORDER BY criado_em DESC
		 LIMIT 50`, auth.Tenant(c))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminOrder
	for rows.Next() {
		var o AdminOrder
		if err := rows.Scan(&o.ID, &o.Cliente, &o.Total); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
```

`auth.Tenant(c)` reads the session — never the URL, never a hidden field, because a value the
browser sends is a value the browser chooses. Below the session guard, in the folders that have
rows, goes the other half:

```go
var AdminTenantGuard = adminSessions.RequireTenant()
```

Not on `/organizacoes`: guarding the screen where the organisation is chosen with the rule that
one must already be chosen is a loop with no way out.

And `trilha audit` counts. It has no SQL parser and says so — what it reports is a place to
look, not a verdict — but "this table is filtered by tenant in seven queries and not in this
one" is a sentence somebody can act on:

```text
✓ queries that may be missing the tenant filter
```

## 7. Production

The [production checklist](/cookbook/production-checklist) is the long form. On a project that
came out of this template, the first day looks like this:

```bash
trilha audit --no-vuln
```

```text
✗ TRILHA_SECRET not set in this environment
    signed cookies (sessions) do not work in production; generate one with: openssl rand -base64 32
! TRILHA_TRUSTED_PROXIES not set
    behind a proxy (nginx, load balancer) set the CIDRs so HSTS, client IP and rate limit are right
! AllowedHosts not set
    list the hosts the app answers for (Config.AllowedHosts or TRILHA_ALLOWED_HOSTS); without it a forged Host header poisons caches and reset links
✓ metrics not exposed
! 2 string field(s) come from outside with no size limit
    add max= to the validate tag: without it the column is what refuses, in production, with the driver's message (Amount, Suporte)
✓ readiness checks registered
! the policy declares relatorios and no route requires it
    add auth.RequirePolicy(Policy, module, level) to the middleware.go of that area, or take the module out of the policy
✓ OIDC client secret outside the code
! 1 write route(s) in route.go without CSRF
    a route.go is an API, and an API does not check the token: a form on another site can post to /sair. Say the branch is pages with `var Kind = trilha.KindPage` in a kind.go above them, or set Config.CSRFForAPI if the API really is the client
✓ trilha_gen.go up to date
✓ Go 1.25.14
✓ .gitignore covers .trilha/ and bin/
✓ queries that may be missing the tenant filter
✓ go vet clean
error: 1 critical item(s)
```

Four of those are yours to answer before the first deploy:

1. `TRILHA_SECRET` in the environment — a real one, the same across replicas, rotated like any
   other secret. Without it the session does not work at all.
2. `Config.AllowedHosts` (or `TRILHA_ALLOWED_HOSTS`) with the hostnames the app answers for.
3. The `relatorios` module the matrix declares and no route requires: either guard that area
   with it or take the module out — a permission that grants nothing is worse than none,
   because somebody will grant it.
4. The `POST /sair` without a CSRF check: it is a `route.go`, and a `route.go` is an API. The
   fix is one `kind.go` saying that branch is pages, and the message names it.

Then `trilha check` is the gate — gen, gofmt, vet, test, audit, openapi in one command — and
[Docker](/cookbook/docker) is the image: `trilha build` makes a single static binary with
`public/` embedded, and the container is that file plus the environment.

## Where to go from here

- [`trilha add`](/reference/cli#trilha-add) — the seventeen recipes, of which this template
  starts with eight.
- [Authentication](/learn/authentication) and [`auth`](/reference/auth) — sessions, policy,
  tenant, API keys, in depth.
- [Ready-made screens](/learn/ui-screens) — what `ui` draws, and the live demos.
- [Database](/cookbook/database), [Background tasks](/cookbook/tasks),
  [E-mail](/cookbook/email) — the three things this app asks for next.
