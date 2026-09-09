package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Headers this package writes. They are the ones every webhook implementation
// converged on, spelled the way most partners already expect.
const (
	HeaderID        = "X-Webhook-Id"
	HeaderEvent     = "X-Webhook-Event"
	HeaderTimestamp = "X-Webhook-Timestamp"
	HeaderSignature = "X-Webhook-Signature"
)

// work takes deliveries off the queue.
func (h *Hooks) work(stop chan struct{}) {
	defer h.done.Done()
	for {
		select {
		case <-stop:
			return
		case id := <-h.queue:
			h.attempt(id)
		}
	}
}

// clock looks for deliveries whose next attempt is due. It is what makes the
// backoff real: a retry in twelve hours is a row with a time on it, not a
// sleeping goroutine that a deploy would forget.
func (h *Hooks) clock(stop chan struct{}) {
	defer h.done.Done()
	t := time.NewTicker(h.tick)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			due, err := h.store.Due(ctx, h.now(), 100)
			cancel()
			if err != nil {
				h.log.Error("webhook", "op", "due", "err", err)
				continue
			}
			for _, id := range due {
				h.enqueue(id)
			}
		}
	}
}

// attempt is one POST, from the store back to the store.
func (h *Hooks) attempt(id string) {
	defer func() {
		if r := recover(); r != nil {
			// A panic in a delivery worker would take the web server with it,
			// and a partner's malformed answer is not worth the process.
			h.log.Error("webhook panic", "delivery", id, "panic", r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout+5*time.Second)
	defer cancel()

	d, err := h.store.Delivery(ctx, id)
	if err != nil {
		return
	}
	if d.State != Pending || d.NextTry.After(h.now()) {
		return // já entregue, desistida, ou ainda não é a hora
	}
	sub, err := h.store.Subscription(ctx, d.SubscriptionID)
	if err != nil || sub.Revoked {
		h.give(ctx, d, 0, "", "the subscription is gone")
		return
	}

	d.Attempt++
	status, corpo, erro := h.post(ctx, sub, d)
	switch {
	case status >= 200 && status < 300:
		d.State, d.Status, d.Err, d.Response, d.Ended = Delivered, status, "", corpo, h.now()
		if err := h.store.SaveDelivery(ctx, d); err != nil {
			h.log.Error("webhook", "delivery", d.ID, "err", err)
		}
		h.log.Info("webhook", "delivery", d.ID, "event", d.Event, "status", status,
			"attempt", d.Attempt)
		return
	case d.Attempt >= len(h.backoff)+1:
		// Out of patience. The status and the beginning of the body are what
		// say why, and they are the difference between one minute of work and
		// an afternoon of it.
		h.give(ctx, d, status, corpo, erro)
		return
	}
	espera := h.backoff[d.Attempt-1]
	d.Status, d.Err, d.Response, d.NextTry = status, erro, corpo, h.now().Add(espera)
	if err := h.store.SaveDelivery(ctx, d); err != nil {
		h.log.Error("webhook", "delivery", d.ID, "err", err)
	}
	h.log.Warn("webhook", "delivery", d.ID, "event", d.Event, "status", status,
		"attempt", d.Attempt, "retry_in", espera, "err", erro)
}

func (h *Hooks) give(ctx context.Context, d Delivery, status int, corpo, erro string) {
	d.State, d.Status, d.Response, d.Err, d.Ended = Failed, status, corpo, erro, h.now()
	if err := h.store.SaveDelivery(ctx, d); err != nil {
		h.log.Error("webhook", "delivery", d.ID, "err", err)
	}
	h.log.Error("webhook gave up", "delivery", d.ID, "event", d.Event,
		"attempts", d.Attempt, "status", status, "err", erro)
}

// post signs and sends. It answers the status, the first kilobyte of the body
// and the transport error, which are the three things the record keeps.
func (h *Hooks) post(ctx context.Context, sub Subscription, d Delivery) (int, string, string) {
	// The address is checked again here, and not only when it was registered.
	// A name that resolved to the partner at registration and resolves to
	// 169.254.169.254 now is the attack; one check would be the accident.
	if err := h.check(sub.URL); err != nil {
		return 0, "", err.Error()
	}
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(d.Payload))
	if err != nil {
		return 0, "", err.Error()
	}
	ts := strconv.FormatInt(h.now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "trilha-webhook/1")
	req.Header.Set(HeaderID, d.ID)
	req.Header.Set(HeaderEvent, d.Event)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, Sign(sub.Secret.Reveal(), ts, d.Payload))

	res, err := h.client.Do(req)
	if err != nil {
		return 0, "", err.Error()
	}
	defer res.Body.Close()
	corpo, _ := io.ReadAll(io.LimitReader(res.Body, 1<<10))
	return res.StatusCode, string(corpo), ""
}

// Sign is the signature both sides compute: HMAC-SHA256 of "timestamp.body",
// hex, behind "sha256=".
//
// The timestamp is inside the signed string and not merely beside it. Signing
// the body alone would make every delivery of the same event byte-identical
// forever, so anybody who captured one could replay it a year later and the
// signature would still check out.
func Sign(secret, timestamp string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(timestamp))
	m.Write([]byte("."))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}
