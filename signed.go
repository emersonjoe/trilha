package trilha

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// MinSecretLen is the minimum accepted secret size in bytes.
const MinSecretLen = 32

// Signer signs and verifies values with HMAC-SHA256. The first key signs;
// every key verifies, so a previous key can be kept during rotation.
type Signer struct {
	keys [][]byte
}

// NewSigner creates a signer; the first key signs, the rest only verify.
func NewSigner(keys ...[]byte) *Signer {
	s := &Signer{}
	for _, k := range keys {
		if len(k) > 0 {
			s.keys = append(s.keys, k)
		}
	}
	return s
}

// ErrNoSecret is returned by SetSigned when no secret is configured.
var ErrNoSecret = errors.New("trilha: TRILHA_SECRET não definido; cookies assinados indisponíveis")

// decodeSecret accepts base64 (std or URL) or raw text, at least 32 bytes.
func decodeSecret(s string) []byte {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) >= MinSecretLen {
			return b
		}
	}
	if len(s) >= MinSecretLen {
		return []byte(s)
	}
	return nil
}

func secretsFromEnv() (cur, prev []byte, short bool) {
	raw := os.Getenv("TRILHA_SECRET")
	cur = decodeSecret(raw)
	short = raw != "" && cur == nil
	prev = decodeSecret(os.Getenv("TRILHA_SECRET_PREVIOUS"))
	return
}

func randomSecret() []byte {
	b := make([]byte, MinSecretLen)
	_, _ = rand.Read(b)
	return b
}

// Sign returns value|expiry|mac, safe for a cookie.
func (s *Signer) Sign(value string, exp time.Time) (string, error) {
	if s == nil || len(s.keys) == 0 {
		return "", ErrNoSecret
	}
	v := base64.RawURLEncoding.EncodeToString([]byte(value))
	e := strconv.FormatInt(exp.Unix(), 10)
	return v + "|" + e + "|" + s.mac(s.keys[0], v, e), nil
}

func (s *Signer) mac(key []byte, v, e string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v + "|" + e))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// Verify checks the signature and expiry with any key, returning the value.
func (s *Signer) Verify(token string, now time.Time) (string, bool) {
	if s == nil {
		return "", false
	}
	parts := strings.Split(token, "|")
	if len(parts) != 3 {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > exp {
		return "", false
	}
	for _, k := range s.keys {
		if subtle.ConstantTimeCompare([]byte(s.mac(k, parts[0], parts[1])), []byte(parts[2])) == 1 {
			b, err := base64.RawURLEncoding.DecodeString(parts[0])
			if err != nil {
				return "", false
			}
			return string(b), true
		}
	}
	return "", false
}

// SetSigned stores a tamper-proof cookie (HttpOnly, SameSite=Lax, Secure on
// HTTPS) that expires after ttl. Returns ErrNoSecret without a secret.
func (c *Ctx) SetSigned(name, value string, ttl time.Duration) error {
	exp := time.Now().Add(ttl)
	tok, err := c.app.signer.Sign(value, exp)
	if err != nil {
		return err
	}
	http.SetCookie(c.w, &http.Cookie{
		Name: name, Value: tok, Path: "/", Expires: exp, MaxAge: int(ttl.Seconds()),
		HttpOnly: true, Secure: c.isSecure(), SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Signed reads a cookie written by SetSigned; ok is false when missing,
// tampered or expired.
func (c *Ctx) Signed(name string) (value string, ok bool) {
	ck, err := c.r.Cookie(name)
	if err != nil {
		return "", false
	}
	return c.app.signer.Verify(ck.Value, time.Now())
}

// ClearCookie expires a cookie.
func (c *Ctx) ClearCookie(name string) {
	http.SetCookie(c.w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
