package webhook

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Tolerance is how old a signed delivery may be and still be accepted. Five
// minutes is enough for a clock that drifts and a retry that queued, and short
// enough that a captured request stops working before anybody could use it.
var Tolerance = 5 * time.Minute

// ErrSignature is every way a delivery can fail to be yours: no signature,
// the wrong one, a body that changed, a timestamp too old. It is one error on
// purpose — telling a caller which part failed tells an attacker how close
// they are.
var ErrSignature = errors.New("webhook: signature does not check out")

// Verify is the other side: the handler that receives a webhook, in Go. It
// reads the body, checks the signature and the age, and hands the bytes back.
//
//	func POST(c *trilha.Ctx) error {
//		body, err := webhook.Verify(c.Request(), os.Getenv("HOOK_SECRET"))
//		if err != nil {
//			return trilha.Errorf(http.StatusUnauthorized, "assinatura inválida")
//		}
//		var ev Evento
//		if err := json.Unmarshal(body, &ev); err != nil {
//			return err
//		}
//		…
//	}
//
// It reads the body, so the handler must not have read it first — and it hands
// the bytes back rather than leaving the caller to read a consumed reader,
// which is the mistake this signature exists to make impossible.
//
// **Answer 2xx before you do the work.** A receiver that finishes processing
// before replying is a receiver the sender gives up on, and then retries — and
// then the work has happened twice. Read, verify, queue, answer.
//
// The id in X-Webhook-Id is stable across retries of the same delivery: keep
// it and refuse a repeat, because at-least-once is what a sender that retries
// can promise, and exactly-once is what the receiver decides.
func Verify(r *http.Request, secret string) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, ErrSignature
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("webhook: reading the body: %w", err)
	}
	if secret == "" {
		return nil, ErrSignature
	}
	ts := r.Header.Get(HeaderTimestamp)
	sig := r.Header.Get(HeaderSignature)
	if ts == "" || sig == "" {
		return nil, ErrSignature
	}
	segundos, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, ErrSignature
	}
	// Both directions: a timestamp from the future is a clock that is wrong or
	// a signature somebody is preparing to use later.
	if idade := time.Since(time.Unix(segundos, 0)); idade > Tolerance || idade < -Tolerance {
		return nil, ErrSignature
	}
	// Constant time, because a comparison that returns early on the first
	// wrong byte is a comparison that can be measured.
	if !hmac.Equal([]byte(sig), []byte(Sign(secret, ts, body))) {
		return nil, ErrSignature
	}
	return body, nil
}
