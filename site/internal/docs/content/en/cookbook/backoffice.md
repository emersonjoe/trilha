---
title: A backoffice that denies by default
description: trilha add admin — app/admin/ closed to every role but admin, with users and roles, the audit trail, the approvals inbox and search, and every decision in the trail with who and to whom.
---

A backoffice is four screens every application has — people and their roles, the trail of who
did what, the decisions waiting for somebody, a search — behind one door. The screens are the
easy half. The door is the half that goes wrong: a new screen added next month under the same
folder, and nobody remembered to guard it.

## The command

```bash
trilha add login admin
```

`trilha add admin` is **made of recipes**: it applies [`users`](/reference/cli#trilha-add), `audit`,
`approvals` and `search` under `app/admin/` — the same recipes, with the same tests, not copies of
their screens — and adds what none of them has: the door and the landing screen.

## What it installs

| Where | What |
|---|---|
| `app/admin/middleware.go` | the door: only the `admin` role passes |
| `app/admin/page.go` | the landing screen, with the decisions waiting |
| `app/admin/usuarios/` | invite, change a role, deactivate, reset |
| `app/admin/auditoria/` | the trail, drawn with `ui.AuditTable` |
| `app/admin/aprovacoes/` | the inbox, drawn with `ui.Inbox` over `approval` |
| `app/admin/busca/` | search over what the application indexed |
| `admin_test.go` | the door by table, and the decisions in the trail |

## Price

`trilha ctx --pack admin` costs **~104 tokens (est.)**, measured on a minimal project by
`TestRecipeCtxPackCost`. The recipes it is made of have their own prices in
`trilha add --list`.

## Deny by default

The middleware guards the folder and everything below it — the screen written there tomorrow
included. A new screen is closed until somebody opens it on purpose, not open until somebody
remembers to close it. Somebody signed in without the role gets `403`, not the login again:
they are known, just not permitted.

```go
// exige is the rule, and Middleware is what the scanner reads: middleware.go
// has to export a function with that signature, and a var of the right type
// is not one.
var exige = sessao.Flow.RequireRole("admin")

// Middleware guards this folder and everything below it — the screens written
// tomorrow included. That is what deny by default means here: a new screen is
// closed until somebody opens it on purpose, not open until somebody
// remembers to close it.
//
// Somebody signed in without the role gets 403 and not a redirect to the
// login: they are known, just not permitted, and sending them back to a login
// they already passed is a loop with no exit.
func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
```

The test that holds the door is a table: every screen under `/admin` against anonymous, a
signed-in person without the role, and an administrator. A screen added to the list is one row.

```go
// telasDoAdmin is every screen under /admin. A screen written there tomorrow
// is one more row, and the table says who may open it.
var telasDoAdmin = []string{"/admin", "/admin/usuarios", "/admin/auditoria", "/admin/aprovacoes", "/admin/busca"}
```

## Every decision in the trail

Changing somebody's role, deactivating an account, resetting a password, inviting, approving a
request: each one is written with `c.Audit`, with the administrator as the actor and the person
or the request as the target, and `/admin/auditoria` shows it. `TestAdminAuditTrail` and
`TestAdminApprovalFlow` hold that in your project.

## Extending it

Put the next screen under `app/admin/` and it is guarded. If the project already had `users`,
`audit`, `approvals` or `search` at the root, their screens now exist under `/admin` too —
delete the old folders to keep one door. [The management app](/cookbook/admin-app) is the same
set, written at creation by `trilha new --template app`.
