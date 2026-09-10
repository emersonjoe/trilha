---
title: webhook
description: Hooks, Options, Emit, Subscribe, Verify, Store and the panel — the API of the webhook package, with the defaults and what each field changes.
---

`import "github.com/emersonjoe/trilha/webhook"` — telling another application that something
happened, without making the visitor wait for somebody else's server, without leaving the
receiver unable to tell your call from a stranger's, and without losing the event because the
partner restarted at three in the morning.

## The sender

```go
func New(o Options) *Hooks                        // starts nothing
func (h *Hooks) Setup(a *trilha.App) error        // workers + clock + Shutdown on the app
func (h *Hooks) Shutdown(ctx context.Context) error
func (h *Hooks) Events() []string
```

`Setup` starts the workers and the clock. The clock is what turns "try again in twelve hours"
into a row with a time on it rather than a sleeping goroutine a deploy would forget. `Shutdown`
waits for the attempts in flight; what is pending stays pending, because it is a row and not a
goroutine.

| `Options` | Default | What it does |
|---|---|---|
| `Store Store` | `Memory()` | where subscriptions and deliveries live |
| `Events []string` | — | the closed list this application emits |
| `Env trilha.Env` | `Prod` | whether `http://` is allowed; pass `a.Env()` |
| `Workers int` | 2 | how many deliveries go at once |
| `Timeout time.Duration` | 10 s | one attempt |
| `Backoff []time.Duration` | 1 m, 5 m, 30 m, 2 h, 12 h | the waits; its length is how many attempts there are |
| `Tick time.Duration` | 15 s | how often the clock looks for what is due |
| `HTTP *http.Client` | no redirects | a 302 is a destination nobody checked |
| `AllowPrivateURL bool` | false | turns the address check off — see below |

The zero value of `Env` is `Prod`, which is the safe way round for a field somebody forgets.

## Emitting

```go
func (h *Hooks) Emit(c *trilha.Ctx, event string, payload any) error
```

Records one delivery per subscription listening to the event, and returns. **It does not wait
for the network**, which is the whole reason it exists. Its error is about this application — an
unknown event, a store that would not write — and never about the partner.

`Events` is closed: a typo in an `Emit` is an error where it is written rather than an event
nobody subscribed to, which from outside is indistinguishable from a partner who is not
listening. A `nil` context is a background job; from a handler, pass `c` and the tenant and the
audit line come with it.

`trilha openapi` reads these calls: an `Emit` with the event name written in it becomes an entry
in the document's `webhooks` section, with the payload's schema as the body and the four delivery
headers described. A struct that is already a component of a route is referenced there, not
copied — so whoever integrates from the other side reads one document instead of your code. An
`Emit` whose name comes from a variable is not documented: a document cannot state a name that
does not exist until the program runs.

## What goes on the wire

```
X-Webhook-Id: dlv_…              stable across the retries of one delivery
X-Webhook-Event: documento.processado
X-Webhook-Timestamp: 1757343845
X-Webhook-Signature: sha256=…    HMAC-SHA256 of "timestamp.body"
```

```go
func Sign(secret, timestamp string, body []byte) string
```

The timestamp is **inside** the signed string. Signing the body alone would make every delivery
of an event byte-identical forever, so a captured request could be replayed a year later and
still check out.

2xx within `Timeout` is `Delivered`. Anything else waits and tries again; after the last backoff
it is `Failed`, keeping the last status and the first kilobyte of the response — which is what
turns "it failed" into "422, field destinatario required".

## Managing

```go
func (h *Hooks) Subscribe(c *trilha.Ctx, s Subscription) (trilha.Secret, error)
func (h *Hooks) Revoke(c *trilha.Ctx, id string) error
func (h *Hooks) Retry(c *trilha.Ctx, deliveryID string) (string, error)
func (h *Hooks) Ping(c *trilha.Ctx, subscriptionID string) (string, error)
func (h *Hooks) Handle(c *trilha.Ctx) error       // one POST, dispatched by "action"
func TakeSecret(c *trilha.Ctx) string             // once, right after Subscribe
```

`Subscribe` answers the secret **once**. What is stored is a `trilha.Secret` — encrypted in the
column, masked in the log — and an application that could show it again would be one that keeps
it readable, which makes the secret worth exactly what the database backup is worth. It crosses
the redirect in a signed cookie of its own; `TakeSecret` reads and clears it.

`Revoke` is not a delete: the deliveries already made point at that subscription, and a screen
that cannot say which endpoint a failure belonged to is useless afterwards. `Subscriptions`
returns revoked ones for that reason; `Subscription.Wants` is where being revoked stops a
delivery.

`Retry` answers a **new** id pointing at the old one, and sends the same bytes — so a partner
checking `X-Webhook-Id` sees another delivery of the same event rather than a body that changed
under the same name. The failed attempt stays.

`Ping` goes through everything a real delivery does, because a test that takes a shortcut passes
for a setup that will not work.

## Receiving

```go
func Verify(r *http.Request, secret string) ([]byte, error)
var Tolerance = 5 * time.Minute
var ErrSignature = errors.New("webhook: signature does not check out")
```

Reads the body, checks the signature and the age, hands the bytes back — that signature exists
so the caller is never left with a consumed reader. The age is checked in both directions: a
timestamp from the future is a clock that is wrong or a signature somebody is preparing. The
comparison is constant time, and `ErrSignature` is one error for every way of failing, because
telling a caller which part failed tells an attacker how close they are.

**Answer 2xx before you do the work.** A receiver that processes first is a receiver the sender
gives up on and retries. `X-Webhook-Id` is stable across the retries of one delivery: keep it and
refuse a repeat, because at-least-once is what a sender can promise and exactly-once is what the
receiver decides.

## The address check

`https` always; `http` only in `Dev`. Every address the name resolves to must be public —
*every* address, because a name answering with one public IP and one loopback would otherwise
pass — and it is checked when the subscription is registered **and again at delivery**, because
a name that resolved to the partner then and to `169.254.169.254` now is the attack rather than
the accident.

In `Dev`, loopback and private addresses pass: a receiver on `localhost:4000` is how anybody
tries this first, and a check that refuses it is one somebody turns off wholesale. Link-local
(`169.254.0.0/16`, where cloud metadata lives), unspecified, multicast and reserved are refused
in every environment.

`AllowPrivateURL` turns the check off. It is spelled out because turning it on is turning off
the defence against a webhook pointed at your own metadata service; it exists for this package's
own tests and for an application whose partners really are on the same private network.

## Store

```go
type Store interface {
	SaveSubscription(ctx context.Context, s Subscription) error
	Subscription(ctx context.Context, id string) (Subscription, error)
	Subscriptions(ctx context.Context, tenant string) ([]Subscription, error)
	SaveDelivery(ctx context.Context, d Delivery) error
	Delivery(ctx context.Context, id string) (Delivery, error)
	Deliveries(ctx context.Context, p ListParams) ([]Delivery, error)
	Due(ctx context.Context, at time.Time, limit int) ([]string, error)
}

func Memory() Store
```

`Due` is the one the clock calls, and the one to put an index behind. There is no SQL
implementation here — the same choice every store in this framework makes — and the
[recipe](/cookbook/webhooks) carries the whole file.

## The screen

```go
func ui.WebhooksPanel(c *trilha.Ctx, subs []ui.WebhookRow, deliveries []ui.DeliveryRow, o ui.WebhooksOpts) h.Node
```

`WebhooksOpts` takes `Action` (where the forms post; empty draws a read-only panel), `CSRF`,
`Events` and `Secret`. It draws plain rows, so the application maps what the module stores to
what the screen shows — and `ui.WebhookRow` has no field for the secret, which is why it cannot
leak into a listing.

## Not here

Asymmetric signatures and more than one active secret per subscription — rotation is real and is
the next thing; today a second `Subscribe` is the path. Delivery across processes: this is one
sender, and two replicas send everything twice.
