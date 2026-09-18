package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
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

// HMACOpts is how somebody else signs what they send you: which header
// carries the MAC, what sits in front of the hex, and the secret they gave
// you. The zero value is Meta's scheme — `X-Hub-Signature-256: sha256=<hex>`
// — which is also GitHub's; Stripe's differs only in the header and the
// prefix.
type HMACOpts struct {
	// Header is the request header that carries the signature. Empty is
	// X-Hub-Signature-256.
	Header string
	// Prefix is what precedes the hex digest in that header. Empty is
	// "sha256="; a provider that sends the bare hex sets HMACNoPrefix.
	Prefix string
	// Secret is the shared secret, read from wherever the application seals
	// it — a Connection, never an environment variable in a handler.
	Secret string
	// MaxBody is how many bytes the body may have. Zero is one MiB; a body
	// longer than this fails the way a wrong signature fails.
	MaxBody int64
	// Hash is the digest under the MAC. Nil is sha256.New.
	Hash func() hash.Hash
}

// HMACNoPrefix is the Prefix for a provider that sends the bare hex digest,
// with nothing in front of it. It exists because an empty Prefix means the
// default, and there had to be a way to say "none".
const HMACNoPrefix = "\x00"

// VerifyHMAC is Verify for a webhook that is not Trilha's: Meta's WhatsApp
// Cloud API, GitHub, Stripe — anybody who signs the raw body with an HMAC and
// puts the hex in a header. It reads the body once, up to MaxBody, compares
// in constant time and hands the bytes back, so a handler never has to write
// hmac.Equal by hand — and never gets it subtly wrong.
//
//	func POST(c *trilha.Ctx) error {
//		body, err := webhook.VerifyHMAC(c.Request(), webhook.HMACOpts{Secret: appSecret})
//		if err != nil {
//			return trilha.Errorf(http.StatusUnauthorized, "assinatura inválida")
//		}
//		…
//	}
//
// Every way to fail is ErrSignature: no header, a header that is not hex, a
// body that changed, a body longer than MaxBody, an empty secret. One error
// on purpose, for the reason Verify gives.
//
// There is no timestamp here because these providers do not sign one; what
// stops a replay is the provider's message id, which the receiver keeps and
// refuses a second time — the same rule as X-Webhook-Id.
func VerifyHMAC(r *http.Request, o HMACOpts) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, ErrSignature
	}
	if o.Header == "" {
		o.Header = "X-Hub-Signature-256"
	}
	switch o.Prefix {
	case "":
		o.Prefix = "sha256="
	case HMACNoPrefix:
		o.Prefix = ""
	}
	if o.MaxBody <= 0 {
		o.MaxBody = 1 << 20
	}
	if o.Hash == nil {
		o.Hash = sha256.New
	}
	// One byte past the limit is how "too long" is told apart from "exactly
	// the limit" without reading the whole thing.
	body, err := io.ReadAll(io.LimitReader(r.Body, o.MaxBody+1))
	if err != nil {
		return nil, fmt.Errorf("webhook: reading the body: %w", err)
	}
	if int64(len(body)) > o.MaxBody {
		return nil, ErrSignature
	}
	if o.Secret == "" {
		return nil, ErrSignature
	}
	sig := r.Header.Get(o.Header)
	if len(sig) <= len(o.Prefix) || sig[:len(o.Prefix)] != o.Prefix {
		return nil, ErrSignature
	}
	got, err := hex.DecodeString(sig[len(o.Prefix):])
	if err != nil {
		return nil, ErrSignature
	}
	m := hmac.New(o.Hash, []byte(o.Secret))
	m.Write(body)
	if !hmac.Equal(got, m.Sum(nil)) {
		return nil, ErrSignature
	}
	return body, nil
}
