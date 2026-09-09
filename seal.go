package trilha

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// The framework already had a secret, a signer and signed cookies. What it did
// not have was "encrypt this to store it" — and what gets written instead is a
// token in the clear, then AES copied off the internet with a fixed IV, then
// the whole key coming back in a GET and showing up in the DevTools.

// ErrSealed is what Open answers when the bytes are not something Seal wrote,
// or were written with a key this process does not have. It is deliberately
// one error for both: telling them apart tells whoever is guessing which of
// the two they got right.
var ErrSealed = errors.New("trilha: could not open the sealed value")

// sealVersion is the first byte of everything Seal writes. It costs one byte
// and it is what lets the format change one day without a database full of
// values nobody can tell apart.
const sealVersion = 1

var (
	sealMu   sync.RWMutex
	sealKeys [][]byte
)

// setSealKeys installs the keys derived from the app's secret. Two apps in one
// process with different secrets is a real hazard — the second would seal what
// the first cannot open — so it is said out loud, once.
func setSealKeys(a *App, secrets ...[]byte) {
	var keys [][]byte
	for _, s := range secrets {
		if len(s) > 0 {
			keys = append(keys, sealKey(s))
		}
	}
	sealMu.Lock()
	defer sealMu.Unlock()
	if len(sealKeys) > 0 && len(keys) > 0 && !hmac.Equal(sealKeys[0], keys[0]) && a != nil {
		a.warnOnce("seal:keys", "trilha: a second secret was installed for Seal; values sealed with the first cannot be opened")
	}
	sealKeys = keys
}

// sealKey derives the encryption key from a signing secret. It is HKDF over
// SHA-256 with a fixed info string, so the key that encrypts is never the key
// that signs — one leaking does not hand over the other.
func sealKey(secret []byte) []byte {
	// Extract: no salt, which is what HKDF says to do when there is none.
	mac := hmac.New(sha256.New, nil)
	mac.Write(secret)
	prk := mac.Sum(nil)
	// Expand, one block: AES-256 wants exactly the 32 bytes SHA-256 gives.
	mac = hmac.New(sha256.New, prk)
	mac.Write([]byte("trilha:seal:v1"))
	mac.Write([]byte{1})
	return mac.Sum(nil)
}

// Seal encrypts a value to be stored: AES-256-GCM under a key derived from
// TRILHA_SECRET, with a random nonce every time.
//
//	sealed, err := trilha.Seal([]byte(apiKey))
//
// What comes back is version, nonce and ciphertext, and it is safe to put in a
// column. **The key is the app's secret**, so this protects a database dump
// and a backup — not the operator, and not anybody holding the environment.
// Encrypting against your own infrastructure needs a key manager, and that is
// a decision this does not pretend to make.
//
// Without a secret it is ErrNoSecret and never a value in the clear.
func Seal(plain []byte) ([]byte, error) {
	sealMu.RLock()
	keys := sealKeys
	sealMu.RUnlock()
	if len(keys) == 0 {
		return nil, ErrNoSecret
	}
	block, err := aes.NewCipher(keys[0])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 1+gcm.NonceSize(), 1+gcm.NonceSize()+len(plain)+gcm.Overhead())
	out[0] = sealVersion
	nonce := out[1:]
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// The version byte travels as additional data: changing it invalidates the
	// tag, so nobody can replay a value as if it were another format.
	return gcm.Seal(out, nonce, plain, out[:1]), nil
}

// Open decrypts what Seal wrote. It tries the current secret and then
// Config.PreviousSecret, which is what makes a rotation possible: everything
// stored under the old key still opens, and everything written from now on
// uses the new one.
//
// A value it cannot open is ErrSealed and never a guess.
func Open(sealed []byte) ([]byte, error) {
	sealMu.RLock()
	keys := sealKeys
	sealMu.RUnlock()
	if len(keys) == 0 {
		return nil, ErrNoSecret
	}
	if len(sealed) < 2 || sealed[0] != sealVersion {
		return nil, ErrSealed
	}
	for _, k := range keys {
		block, err := aes.NewCipher(k)
		if err != nil {
			continue
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil || len(sealed) < 1+gcm.NonceSize() {
			continue
		}
		nonce := sealed[1 : 1+gcm.NonceSize()]
		if plain, err := gcm.Open(nil, nonce, sealed[1+gcm.NonceSize():], sealed[:1]); err == nil {
			return plain, nil
		}
	}
	return nil, ErrSealed
}

// Secret is a value that must not be stored in the clear and must not come
// back whole on a screen: an API key, a webhook secret, an SMTP password.
//
//	type Integration struct {
//		Name  string        `json:"name"  form:"name"`
//		Token trilha.Secret `json:"token" form:"token"`
//	}
//
// It is a string underneath, so Bind fills it and the validate tags work. What
// it changes is everything that could leak it by accident:
//
//   - JSON, String and the logger all answer the mask ("sk-…4f2a"), so a
//     c.JSON of the struct, a %v in a log line and an audit record never carry
//     the value;
//
//   - Reveal() is the only way to read it, and it is a call somebody writes on
//     purpose;
//
//   - Sealed() is what goes in the database, and database/sql does it by
//     itself — the type is a driver.Valuer and an sql.Scanner;
//
//   - a form field that came back **empty leaves the stored value alone**,
//     which is the "leave blank to keep" every settings screen needs and
//     every settings screen writes by hand.
//
//     see: trilha.Seal, ui.SecretField
type Secret string

// Reveal is the secret itself. It is a method and not a conversion because
// reading a secret should be a line somebody wrote deliberately.
//
// The issue that asked for this type called it Value. It cannot be: a Secret
// is a string underneath, and database/sql converts a string-kinded value all
// by itself — so unless Value() is the driver.Valuer, passing a Secret to a
// query stores the plaintext, silently, which is the exact accident this type
// exists to prevent. The driver method wins the name; the reader gets one that
// says what it does.
func (s Secret) Reveal() string { return string(s) }

// Empty reports whether there is nothing stored.
func (s Secret) Empty() bool { return s == "" }

// String is the mask, which is what makes %v and %s safe.
func (s Secret) String() string { return maskSecret(string(s)) }

// MarshalJSON writes the mask: a struct answered as JSON never carries the
// secret, and that is the accident this type exists to prevent.
func (s Secret) MarshalJSON() ([]byte, error) {
	return []byte(`"` + maskSecret(string(s)) + `"`), nil
}

// UnmarshalJSON accepts the value as it comes — a client sending a new secret
// sends the secret, not a mask. A mask arriving back is treated as "unchanged"
// and never stored as if it were the value.
func (s *Secret) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' {
		return fmt.Errorf("trilha: Secret expects a string")
	}
	v := string(b[1 : len(b)-1])
	if isMask(v) {
		return nil
	}
	*s = Secret(v)
	return nil
}

// LogValue is what log/slog prints: the mask, always.
func (s Secret) LogValue() slog.Value { return slog.StringValue(maskSecret(string(s))) }

// Sealed is the encrypted form, for storing.
func (s Secret) Sealed() ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	return Seal([]byte(s))
}

// SecretFrom rebuilds a Secret from what Sealed wrote.
func SecretFrom(sealed []byte) (Secret, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	plain, err := Open(sealed)
	if err != nil {
		return "", err
	}
	return Secret(plain), nil
}

// Value makes the type a driver.Valuer: what reaches the column is the sealed
// bytes. Without this method database/sql would convert the string underneath
// and store the secret in the clear — the accident is one Exec away, and it is
// silent.
func (s Secret) Value() (driver.Value, error) { return s.Sealed() }

var (
	_ driver.Valuer = Secret("")
	_ sql.Scanner   = (*Secret)(nil)
)

// Scan reads the sealed bytes from a column.
func (s *Secret) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*s = ""
		return nil
	case []byte:
		got, err := SecretFrom(t)
		if err != nil {
			return err
		}
		*s = got
		return nil
	case string:
		got, err := SecretFrom([]byte(t))
		if err != nil {
			return err
		}
		*s = got
		return nil
	}
	return fmt.Errorf("trilha: Secret cannot scan %T", v)
}

// maskEllipsis is the character that says "there is more here".
const maskEllipsis = "…"

// maskSecret shows enough to recognise a key and never enough to use it: the
// first three characters (which is the provider's prefix, and the thing
// somebody is actually checking) and the last four.
func maskSecret(v string) string {
	switch {
	case v == "":
		return ""
	case len(v) <= 8:
		// Too short to show anything without showing most of it.
		return maskEllipsis
	}
	return v[:3] + maskEllipsis + v[len(v)-4:]
}

// isMask recognises what maskSecret writes, so a value that came back from a
// screen is not stored as if somebody had typed it. A real secret with an
// ellipsis in it would be misread as a mask — and a secret containing "…" is
// not a thing any provider issues, while a mask travelling back into the
// database is a thing that happens on the first screen somebody builds.
func isMask(v string) bool { return strings.Contains(v, maskEllipsis) }

// Pepper is a keyed hash of data under a key derived from the app's secret —
// the same secret Seal and the signer use, with an info string of its own.
//
// It is what turns "store the hash" into "store a hash nobody can attack
// offline with a rainbow table": the digest cannot be computed without the
// application's secret, so a stolen database of hashes is not a list of
// guessable inputs. Use it for an API key, never for a password — a password
// needs a slow hash, and auth.HashPassword is that.
//
// The answer is deterministic, which is what makes it a lookup key; without a
// secret it is ErrNoSecret, never an unkeyed digest that would look like it
// worked.
func Pepper(data []byte) ([]byte, error) {
	sealMu.RLock()
	keys := sealKeys
	sealMu.RUnlock()
	if len(keys) == 0 {
		return nil, ErrNoSecret
	}
	mac := hmac.New(sha256.New, keys[0])
	mac.Write([]byte("trilha:pepper:v1"))
	mac.Write(data)
	return mac.Sum(nil), nil
}
