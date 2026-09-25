---
title: Billing without a coupled provider
description: trilha add billing — plans, subscriptions on a closed state machine, a signed idempotent webhook, dunning by e-mail, and the screens behind two roles. The provider charges the card; the application decides what each event means.
---

Every product that charges ends up writing the same five things: a table of plans, a
subscription that moves between states, the endpoint the payment provider calls, the e-mail
that goes out when a card fails, and a screen for whoever looks after the money. None of it is
the provider's to decide — the provider charges the card and says what happened. What
*past due* means for your application, when a subscription is over, who may export the
invoices: that is the application's, and it is what `trilha add billing` writes.

## The command

```bash
trilha add login connections billing
```

`billing` stands on two recipes and refuses without them, by name, before writing anything:
`login` for the session and the roles, `connections` for the sealed secret the provider signs
with.

## What it installs

| Where | What |
|---|---|
| `internal/cobranca/` | plans, subscriptions, invoices, the state machine, the webhook, the reminders |
| `migrations/0100_billing.sql` | the tables `billing_plans`, `billing_subscriptions`, `billing_invoices`, `billing_events`, in the convention of [`trilha add store`](/cookbook/database) |
| `app/webhooks/billing/` | `POST /webhooks/billing`, the address the provider calls — fixed, because it is typed into the provider's dashboard |
| `app/billing/` | subscriptions filtered by state, `planos/` (create and delete), `faturas/` and `faturas/csv/` |
| `billing_test.go` | the tests below, in your project, run by `trilha check` |

Two roles, declared once as an `auth.Policy`: `billing:reader` reads the screens,
`billing:admin` edits plans and exports invoices. `admin` gets the second level too.

```go
// Politica is who may do what with money. "admin" is here because the person
// who administers the application should not need a second role to look at
// what it charges.
var Politica = auth.Policy{
	Modules: []string{"billing"},
	Levels:  auth.Levels{"ler", "administrar"},
	Roles: map[string]auth.Grants{
		PapelLeitor: {"billing": "ler"},
		PapelAdmin:  {"billing": "administrar"},
		"admin":     {"billing": "administrar"},
	},
}
```

## Price

`trilha ctx --pack billing` costs **~338 tokens (est.)** — what an agent reads to know what the
recipe brought, measured on a minimal project by `TestRecipeCtxPackCost`. `trilha add --list`
shows the same number, and the badge on this page prices the documentation it links to.

## The machine

Nothing changes a subscription's state without passing through `Mover`, and `Mover` only allows
what this table says. A canceled subscription does not come back to life because an old event
arrived late.

```go
// Transicoes is the whole machine. Nothing changes a subscription's state
// without passing through Mover, and Mover only allows what is written here —
// a canceled subscription does not come back to life because an old event
// arrived late.
var Transicoes = map[Estado][]Estado{
	Trial:     {Ativa, Atrasada, Cancelada},
	Ativa:     {Atrasada, Cancelada},
	Atrasada:  {Ativa, Cancelada},
	Cancelada: nil,
}
```

A failed charge moves the subscription to `past_due` and counts one attempt; a paid invoice
puts it back to `active` and forgets the failures; the third failure in a row cancels it
(`MaxTentativas`). Each failure queues a reminder as a [task](/cookbook/tasks) — the provider is
waiting for the answer, and an e-mail server is not a reason to make it wait — and the third
reminder says the subscription is over.

## The endpoint

The provider signs `timestamp.body` with the secret of the `billing-webhook` connection,
in `X-Webhook-Timestamp` and `X-Webhook-Signature` — the scheme of
[`webhook.Verify`](/reference/webhook), which checks it in constant time and refuses a
timestamp more than five minutes away from now. Every way of failing is the same `401` with
the same code, [`E_BILLING_WEBHOOK_UNSIGNED`](/docs/errors/E_BILLING_WEBHOOK_UNSIGNED): saying
which part failed tells a stranger how close they got.

```go
// Receber is the whole endpoint: signature over timestamp.body, the window,
// the event seen once, the change, the trail, the answer.
//
// webhook.Verify checks the signature in constant time and refuses a
// timestamp more than webhook.Tolerance (five minutes) away from now, in
// either direction — a captured request stops working before anybody could
// replay it. Every way of failing is the same 401 with the same code: saying
// which part failed tells a stranger how close they got.
func (b *Billing) Receber(c *trilha.Ctx) error {
	segredo, err := Segredo(c)
	if err != nil {
		c.Log().Warn("cobranca: webhook refused", "code", CodigoNaoAssinado, "why", "no connection")
		return naoAssinado()
	}
	corpo, err := webhook.Verify(c.Request(), segredo)
	if err != nil {
		c.Log().Warn("cobranca: webhook refused", "code", CodigoNaoAssinado)
		return naoAssinado()
	}
	var ev Evento
	if err := json.Unmarshal(corpo, &ev); err != nil || ev.ID == "" {
		return trilha.Errorf(http.StatusBadRequest, "invalid event")
	}
	// Signed and not seen before, and only then: an unsigned request must not
	// be able to burn an id a real event will carry later.
	agora := time.Now().UTC()
	if b.Store.Visto(ev.ID, agora) {
		return c.JSON(http.StatusOK, map[string]string{"status": "duplicate"})
	}
	if err := b.aplicar(c, ev, agora); err != nil {
		// The event was understood and refused by the machine — an old event
		// arriving after a cancel, say. It is logged with the reason, and the
		// answer is still 200: retrying it would not make it valid.
		c.Log().Warn("cobranca: event not applied", "event", ev.ID, "type", ev.Tipo, "err", err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

The event id is recorded only after the signature checks out — an unsigned request must not
be able to burn an id a real event will carry later — and a second delivery of the same id
answers `200` without applying anything. Every change that touches money goes to the audit
trail with the event id and the transition.

## Extending it

- **Another provider**: the struct `Evento` and the four event names in
  `internal/cobranca/webhook.go` are the only provider-shaped code. A provider that signs
  another way is `webhook.VerifyHMAC` in place of `webhook.Verify` in `Receber`.
- **A database**: the store is memory behind the `Store` methods; the migration is the same
  shape as tables. With `trilha add store`, a store over `billing_*` replaces it and no screen
  changes.
- **Production**: the secret lives in `connections`, and `trilha audit` warns while those are
  kept in memory — a restart would forget the secret and every event would be refused.
