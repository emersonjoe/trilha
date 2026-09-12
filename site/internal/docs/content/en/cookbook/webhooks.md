---
title: Telling another application
description: trilha/webhook — signed deliveries, retries with backoff, a record of every attempt, and the address check that stops your own server reading its cloud credentials.
---

Telling another application that something happened is what every internal app ends up needing,
and what gets written for it is an `http.Post` inside the handler. That has four problems, and
none of them shows up on the day the code is written.

The visitor's request waits for somebody else's server — another company, another network,
sometimes another decade. There is no signature, so the receiver cannot tell your call from
whoever found the URL. There is no retry: the partner restarted at three in the morning and that
event simply never existed. And there is no record, so when somebody asks "did you send it?" the
answer is a grep.

There is a fifth, and it is uglier. The URL belongs to the partner, but the person typing it
works for you. A webhook pointed at `http://169.254.169.254/` is your own server fetching the
machine's cloud credentials and posting them to whoever registered the address.

## Or: `trilha add webhooks`

```bash
trilha add webhooks --dry-run
```

```text
  + internal/avisos/avisos.go
  + internal/avisos/avisos_test.go
  + app/webhooks/page.go
  + webhooks_test.go
  ~ app/setup.go (one line added)

--dry-run: nothing was written
```

That is the closed list of events, the deliverer wired to the app's `Env`, and the
`ui.WebhooksPanel` screen — signed delivery, retry and the record of every attempt, running
before you write a line. What it writes no middleware for is guarding the screen, because it
cannot know how your project authenticates; the command's own last line says so. What this
page adds is everything below the recipe's floor: what actually goes on the wire and why the
timestamp is inside the signature, the address check and why it runs twice, and the SQL
schema for when the retry clock has to survive a restart.

## Emitting

```go
	hooks := webhook.New(webhook.Options{
		Events: []string{"documento.processado", "documento.falhou"},
		Env:    a.Env(),
	})
	trilha.Provide(a, hooks)
	if err := hooks.Setup(a); err != nil {
		return err
	}
```

`Events` is a closed list, and that is the point of it: a typo in an `Emit` becomes an error
where it is written, rather than an event nobody subscribed to — which, from outside, looks
exactly like a partner who is not listening.

`Setup` starts the workers and the clock. The clock is what makes the backoff real: a retry in
twelve hours is a row with a time on it, not a sleeping goroutine a deploy would forget.

Then one line, wherever the thing happens:

```go
func (m *motor) avisa(evento string, doc documentos.Documento) {
	if m.hooks == nil {
		return
	}
	if err := m.hooks.Emit(nil, evento, map[string]any{
		"id": doc.ID, "nome": doc.Nome, "tipo": doc.Tipo, "bytes": doc.Bytes,
	}); err != nil {
		slog.Default().Error("webhook", "evento", evento, "documento", doc.ID, "err", err)
	}
}
```

`Emit` records one delivery per subscription listening and returns. It does not wait for the
network, which is the whole reason it exists. Its error is about *this* application — an unknown
event, a store that would not write — and never about the partner: whether the partner answered
is a delivery record, and a handler has no business waiting to find out.

The `nil` context is a background job, where there was never a request. From a handler, pass `c`
and the tenant and the audit line come with it.

## What goes on the wire

```
POST https://parceiro/hook
Content-Type: application/json
X-Webhook-Id: dlv_…
X-Webhook-Event: documento.processado
X-Webhook-Timestamp: 1757343845
X-Webhook-Signature: sha256=…
```

The signature is HMAC-SHA256 of `timestamp.body`. The timestamp is **inside** the signed string
and not merely beside it: signing the body alone would make every delivery of the same event
byte-identical forever, so anybody who captured one could replay it a year later and it would
still check out.

2xx within ten seconds is delivered. Anything else waits and tries again — 1 min, 5, 30, 2 h,
12 h — and gives up on the sixth, keeping the last status and the first kilobyte of the body.
**That body is the point of keeping it**: "422 field destinatario required" solves in a minute
what "failed" does not solve in an afternoon.

## Receiving

The other side, in Go:

```go
func Receber(c *trilha.Ctx) error {
	corpo, err := webhook.Verify(c.Request(), hookSecret())
	if err != nil {
		// Uma resposta só para todas as formas de falhar: dizer qual delas é
		// dizer a um estranho o quanto ele chegou perto.
		return trilha.Errorf(http.StatusUnauthorized, "assinatura inválida")
	}
	var ev struct {
		ID   string `json:"id"`
		Nome string `json:"nome"`
	}
	if err := json.Unmarshal(corpo, &ev); err != nil {
		return trilha.Errorf(http.StatusBadRequest, "corpo inválido")
	}
	// O id da entrega é estável entre as tentativas da mesma entrega: guardá-lo
	// e recusar repetido é o que transforma o "pelo menos uma vez" que o
	// remetente promete no "exatamente uma vez" que você decide.
	if visto(c.Request().Header.Get(webhook.HeaderID)) {
		return c.Text(http.StatusOK, "ok")
	}
	enfileira(ev.ID)
	return c.Text(http.StatusOK, "ok")
}
```

Three lines and an order, and the order is the part people get wrong. **Answer 2xx before you do
the work.** A receiver that processes and only then replies is a receiver the sender considers
down — so it retries, and the work has happened twice.

`Verify` reads the body and hands the bytes back, which is why it has that signature: leaving
the caller with a consumed reader is the mistake it exists to make impossible. It checks the age
in both directions — a timestamp from the future is a clock that is wrong or a signature
somebody is preparing to use later — and compares in constant time.

## The address check

This is the part nobody writes on their own, and the reason is that the dangerous case does not
look dangerous: the URL belongs to a partner, but the form is filled in by one of your own
people.

- `https` always; `http` only when `Env` is `Dev`.
- Every address the name resolves to has to be public. **Every** address, because a name that
  answers with one public IP and one loopback address would otherwise pass.
- Checked when it is registered **and again when it is delivered to**, because a name that
  resolved to the partner at registration and resolves to `169.254.169.254` now is the attack;
  one check would be the accident.
- In dev, loopback and private addresses are fine — a receiver on `localhost:4000` is how
  anybody tries this the first time, and a check that refuses it is a check somebody turns off
  wholesale. Link-local is refused in every environment, because it is nobody's development
  setup and a dev config that gets promoted is exactly how that one bites.

The client follows no redirects either: a 302 from the partner's host to somewhere else is a
destination nobody checked.

## The screen

`ui.WebhooksPanel` is register, list, retry and test. It draws plain data, so the application
maps what the module stores to what the screen shows — and that mapping is where "what a screen
shows" is decided:

```go
func linhas(subs []webhook.Subscription) []ui.WebhookRow {
	out := make([]ui.WebhookRow, 0, len(subs))
	for _, s := range subs {
		out = append(out, ui.WebhookRow{ID: s.ID, Label: s.Label, URL: s.URL,
			Events: s.Events, Created: s.Created, Revoked: s.Revoked})
	}
	return out
}
```

The secret is not in that row and cannot be. It is shown once, when the subscription is created,
and crosses the redirect in a cookie of its own — `webhook.TakeSecret(c)` reads it and clears
it. What is stored is a `trilha.Secret`: encrypted in the column, masked in the log.

```go
func POST(c *trilha.Ctx) error {
	return trilha.Use[*webhook.Hooks](c).Handle(c)
}
```

One POST for register, revoke, retry and test, dispatched by a hidden `action` field, because
they are one screen — and a form that posts to itself comes back to itself when something is
wrong.

Revoking is not deleting. The deliveries already made point at that subscription, and a screen
that cannot say which endpoint a failure belonged to is a screen nobody can use afterwards.

## Keeping it in a table

`Memory()` is the default. A table buys the history and, more importantly, makes the retry
schedule survive a restart:

```go
const WebhookSchema = `
CREATE TABLE IF NOT EXISTS trilha_webhooks (
	id       TEXT PRIMARY KEY,
	tenant   TEXT NOT NULL DEFAULT '',
	url      TEXT NOT NULL,
	events   TEXT NOT NULL DEFAULT '',
	secret   BYTEA NOT NULL,
	label    TEXT NOT NULL DEFAULT '',
	created  TIMESTAMPTZ NOT NULL,
	revoked  BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE IF NOT EXISTS trilha_webhook_deliveries (
	id          TEXT PRIMARY KEY,
	webhook_id  TEXT NOT NULL REFERENCES trilha_webhooks(id),
	tenant      TEXT NOT NULL DEFAULT '',
	event       TEXT NOT NULL,
	payload     BYTEA NOT NULL,
	state       TEXT NOT NULL,
	attempt     INTEGER NOT NULL DEFAULT 0,
	next_try    TIMESTAMPTZ,
	status      INTEGER NOT NULL DEFAULT 0,
	err         TEXT NOT NULL DEFAULT '',
	response    TEXT NOT NULL DEFAULT '',
	retry_of    TEXT NOT NULL DEFAULT '',
	created     TIMESTAMPTZ NOT NULL,
	ended       TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS trilha_webhook_due ON trilha_webhook_deliveries (next_try) WHERE state = 'pending';
CREATE INDEX IF NOT EXISTS trilha_webhook_recent ON trilha_webhook_deliveries (id DESC);
`
```

```go
func (s WebhookSQL) Due(ctx context.Context, at time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id FROM trilha_webhook_deliveries
		WHERE state = 'pending' AND next_try <= $1
		ORDER BY next_try LIMIT $2`, at, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
```

`Due` is the one query that runs all the time; the partial index exists for that line. The whole
file is in `examples/cookbook/webhooks.go`, Postgres flavoured.

:::warning
With two replicas, two clocks find the same due delivery and the partner gets it twice. Add
`FOR UPDATE SKIP LOCKED` to `Due` inside a transaction that also moves the row out of `pending`,
or run the sender on one instance. The module is honest about this: it is one process, and the
[task module](/cookbook/tasks) says the same thing for the same reason.
:::

## What is tested

The signature is checked by running the real `Verify` against what the real sender produced —
not by recomputing the HMAC in the test, which would prove only that a formula matches itself.

The backoff is tested against a clock the test controls, so proving a twelve-hour wait does not
take twelve hours. The address check is tested against a resolver the test controls, including
the case a careless check would pass: a name answering with one good address and one bad one.
