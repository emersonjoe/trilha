package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/emersonjoe/trilha"
)

// Subscribe registers a partner and answers the secret **once**.
//
// Once is the whole design. What goes in the store is a trilha.Secret, so the
// column is encrypted and the log is masked; what comes back here is the plain
// value, and the screen shows it with ui.SecretOnce. An application that could
// show it again would be an application that keeps it readable, and then the
// secret is worth exactly as much as the database backup.
//
// The URL is checked before anything is written: https outside dev, and no
// address that resolves onto your own network.
func (h *Hooks) Subscribe(c *trilha.Ctx, s Subscription) (trilha.Secret, error) {
	if err := h.check(s.URL); err != nil {
		return "", err
	}
	for _, e := range s.Events {
		if !h.known(e) {
			return "", fmt.Errorf("%w: %q", ErrUnknownEvent, e)
		}
	}
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
		if s.Tenant == "" {
			s.Tenant = c.Actor().Tenant
		}
	}
	plain := newSecret()
	s.ID, s.Secret, s.Created, s.Revoked = newID("whk"), trilha.Secret(plain), h.now(), false
	if err := h.store.SaveSubscription(ctx, s); err != nil {
		return "", err
	}
	if c != nil {
		c.Audit("webhook.assinou", s.ID, trilha.Fields{"url": s.URL, "eventos": s.Events})
	}
	return trilha.Secret(plain), nil
}

// Revoke stops a subscription. It is not a delete: the deliveries already made
// point at it, and a screen that cannot say which endpoint a failure belonged
// to is a screen nobody can use afterwards.
func (h *Hooks) Revoke(c *trilha.Ctx, id string) error {
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
	}
	s, err := h.store.Subscription(ctx, id)
	if err != nil {
		return err
	}
	s.Revoked = true
	if err := h.store.SaveSubscription(ctx, s); err != nil {
		return err
	}
	if c != nil {
		c.Audit("webhook.revogou", id, trilha.Fields{"url": s.URL})
	}
	return nil
}

// Retry sends the same payload again, as a **new** delivery pointing at the
// old one. The failed attempt stays: it is the reason somebody pressed the
// button, and a screen that swallows it loses the only account of what the
// partner said.
//
// The bytes are the ones that were signed the first time, so a partner
// checking X-Webhook-Id sees a different delivery of the same event — which is
// what a retry is — rather than a body that changed under the same name.
func (h *Hooks) Retry(c *trilha.Ctx, id string) (string, error) {
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
	}
	old, err := h.store.Delivery(ctx, id)
	if err != nil {
		return "", err
	}
	if old.State == Pending {
		return old.ID, nil // ainda está indo; reenviar é esperar
	}
	novo := Delivery{
		ID: newID("dlv"), SubscriptionID: old.SubscriptionID, Tenant: old.Tenant,
		Event: old.Event, Payload: old.Payload, State: Pending,
		RetryOf: old.ID, Created: h.now(), NextTry: h.now(),
	}
	if err := h.store.SaveDelivery(ctx, novo); err != nil {
		return "", err
	}
	h.enqueue(novo.ID)
	if c != nil {
		c.Audit("webhook.reenviou", novo.ID, trilha.Fields{"original": old.ID, "evento": old.Event})
	}
	return novo.ID, nil
}

// Ping sends a test delivery, which is what somebody presses right after
// pasting a URL. It goes through everything a real one does — the same
// signature, the same headers, the same record — because a test that takes a
// shortcut is a test that passes for a setup that will not work.
func (h *Hooks) Ping(c *trilha.Ctx, id string) (string, error) {
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
	}
	s, err := h.store.Subscription(ctx, id)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]any{
		"event": "webhook.ping", "at": h.now().UTC().Format(time.RFC3339),
		"subscription": s.ID,
	})
	d := Delivery{
		ID: newID("dlv"), SubscriptionID: s.ID, Tenant: s.Tenant, Event: "webhook.ping",
		Payload: body, State: Pending, Created: h.now(), NextTry: h.now(),
	}
	if err := h.store.SaveDelivery(ctx, d); err != nil {
		return "", err
	}
	h.enqueue(d.ID)
	return d.ID, nil
}

// Subscriptions is what the screen lists.
func (h *Hooks) Subscriptions(ctx context.Context, tenant string) ([]Subscription, error) {
	return h.store.Subscriptions(ctx, tenant)
}

// Deliveries is the other half of the screen.
func (h *Hooks) Deliveries(ctx context.Context, p ListParams) ([]Delivery, error) {
	return h.store.Deliveries(ctx, p)
}

// SecretCookie carries a new subscription's secret across the redirect that
// follows creating it. TakeSecret reads it and clears it; nothing else should
// touch it.
const SecretCookie = "trilha_webhook_secret"

// TakeSecret answers the secret of a subscription just created, once, and
// clears it. An empty string is the normal case — every render but the one
// right after Subscribe.
//
//	if s := webhook.TakeSecret(c); s != "" {
//		painel = append(painel, ui.SecretOnce(c, s))
//	}
func TakeSecret(c *trilha.Ctx) string {
	v, ok := c.Signed(SecretCookie)
	if !ok {
		return ""
	}
	c.ClearCookie(SecretCookie)
	return v
}

// Handle is the one POST the panel needs, dispatched by the form's action
// field: subscribe, revoke, retry, ping.
//
// One route and not four because they are one screen, and a form that posts to
// itself is a form that comes back to itself when something is wrong. The
// secret of a new subscription crosses the redirect in a cookie of its own —
// see TakeSecret.
//
//	func POST(c *trilha.Ctx) error { return app.Hooks.Handle(c) }
func (h *Hooks) Handle(c *trilha.Ctx) error {
	switch c.Form("action") {
	case "subscribe":
		eventos := c.Request().Form["events"]
		secret, err := h.Subscribe(c, Subscription{
			URL: strings.TrimSpace(c.Form("url")), Events: eventos,
			Label: strings.TrimSpace(c.Form("label")),
		})
		if err != nil {
			c.Flash("error", err.Error())
			break
		}
		// The plain secret crosses the redirect in a signed cookie of its own
		// and is cleared by the first render — the same mechanism a flash
		// uses, with a channel of its own because the layout's toaster fades
		// its messages, and a secret that fades while somebody is copying it
		// is a secret they have to go and recreate.
		//
		// Not in the URL: a query string is in the browser history, in the
		// proxy log and in the referrer of the next click.
		if err := c.SetSigned(SecretCookie, secret.Reveal(), 5*time.Minute); err != nil {
			c.Flash("error", "A assinatura foi criada, mas o segredo não pôde ser mostrado (falta TRILHA_SECRET). Revogue e crie de novo.")
			break
		}
		c.Flash("success", "Assinatura criada. Copie o segredo agora.")
	case "revoke":
		if err := h.Revoke(c, c.Form("id")); err != nil {
			c.Flash("error", err.Error())
			break
		}
		c.Flash("success", "Assinatura revogada.")
	case "retry":
		if _, err := h.Retry(c, c.Form("id")); err != nil {
			c.Flash("error", err.Error())
			break
		}
		c.Flash("success", "Reenviando.")
	case "ping":
		if _, err := h.Ping(c, c.Form("id")); err != nil {
			c.Flash("error", err.Error())
			break
		}
		c.Flash("success", "Teste enviado.")
	default:
		return trilha.Errorf(http.StatusBadRequest, "ação desconhecida")
	}
	return c.Redirect(c.Request().URL.Path)
}
