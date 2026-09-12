---
title: approval
description: Approvals, Request, Decide, On, Inbox and the states — the API of the approval package, the queue that waits for a person.
---

`import "github.com/emersonjoe/trilha/approval"` — the other half of the work an application
does: the part that waits for a person. The [task](/reference/task) package runs what a machine
can finish on its own; this one is the queue every business application grows anyway — things
somebody has to approve, reject or let expire.

It answers the four things such a queue always needs and that nobody writes the first time: **an
owner, a deadline, the reason written down, and who decided.**

```go
var Fila = approval.New(approval.Options{
	Roles: func(c *trilha.Ctx) []string { return sessao.Atual(c).Roles },
})

id, err := Fila.Open(c, approval.Request{
	Kind:    "eliminacao",           // groups the queue, and what On is registered for
	Subject: "Listagem 2024/07",     // the line somebody reads
	Target:  "/admin/retencao/123",  // where the thing being decided lives
	Assign:  approval.Role("cpad"),  // or approval.User(id)
	Due:     time.Now().Add(72 * time.Hour),
})

err = Fila.Decide(c, id, approval.Approved, "ok pelo quórum")
```

## What it decides for you, and what it does not

**Who may decide is the package's answer**, checked inside `Decide` and not on the screen: a
screen that hides a button is a screen, and the address behind it is still an address. `Roles` is
how a request assigned to a role finds its people — a function you supply, because this package
does not know how you authenticate, and a check it guessed would look like a guarantee without
being one. `MayDecide(c, rec)` is that same check, exported so the screen asks the package
instead of reimplementing the rule: the buttons it draws and the decision `Decide` accepts can
never disagree. When they do, `Decide` answers `approval.ErrNotYours`; an id that is not a
request answers `approval.ErrUnknown`. Who a request is assigned to is an `approval.Assignee`
— what `approval.Role(name)` and `approval.User(id)` build.

**What a decision means is yours.** `On(kind, fn)` runs after the decision is written, and that is
where the application deletes the thing, sends the mail or emits the webhook. Its error **does not
undo the decision**: a person chose, and it is recorded; a mail server being down is not a reason
to pretend they did not. Making that work survive a failure is the handler's job — a
[task](/reference/task), a [webhook](/reference/webhook).

**The deadline expires on its own**, on a clock in this process, for the same reason the task
package sweeps its own: an application that needs a cron to be correct is an application that is
wrong on the day the cron does not run. The sweep is `Expire(ctx)`, exported and returning how
many it closed, so a test moves the deadline by hand and asserts the number instead of waiting
for a tick.

## The states

`pending`, `approved`, `rejected`, `withdrawn`, `expired` — a registered `trilha.Enum`
(`approval.States`), so [`ui.Status`](/reference/ui) colours them and the `enum=` tag validates
them without the application declaring the list a second time.

In Go they are constants: `approval.Pending`, `approval.Approved`, `approval.Rejected`,
`approval.Withdrawn` and `approval.Expired`. The first four are a decision somebody made and
travel into `Decide`; `Expired` is the only state the package writes on its own, which is why
it is not a decision you can pass.

## The screen

```go
ui.Inbox(c, rows, ui.InboxOpts{Decide: "/admin/aprovacoes", CSRF: trilha.CSRFInput(c)})
ui.InboxBadge(len(pending))   // the number on a menu item; zero draws nothing
```

A table and two forms, no JavaScript. The deadline is written by `ui.Relative`, so "in 3 days" and
"2 days ago" are the same sentence in the reader's language, and a late row carries `ui-late`. The
decision and the reason travel in **the same form**: a reason typed into a field that a second
click discards is a reason nobody wrote.

`trilha add approvals` writes the package wired up, the screen and the test.

## Store

`Memory()` is the default and the right one for a single process. A table behind the same three
methods — `Save`, `Get`, `List` — is the next step, and no screen changes: the framework does not
own your schema.
