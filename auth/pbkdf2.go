package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// DefaultPBKDF2Iterations is the work factor of a new hash (OWASP's floor for
// PBKDF2-HMAC-SHA256). An old hash keeps the number written inside it, so
// raising this does not lock anybody out.
const DefaultPBKDF2Iterations = 600000

// PBKDF2 derives a key from a password (RFC 8018) with HMAC-SHA256. It is here
// because the alternative was a dependency: bcrypt and argon2 are better at
// this and neither is in the standard library.
func PBKDF2(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hLen := prf.Size()
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	buf := make([]byte, 4)
	u := make([]byte, hLen)
	t := make([]byte, hLen)
	for b := 1; b <= blocks; b++ {
		prf.Reset()
		prf.Write(salt)
		binary.BigEndian.PutUint32(buf, uint32(b))
		prf.Write(buf)
		u = prf.Sum(u[:0])
		copy(t, u)
		for i := 1; i < iter; i++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

// HashPBKDF2 hashes a password in the format Django and most Python libraries
// write, so an existing users table stays valid without a password migration:
//
//	pbkdf2_sha256$600000$<salt>$<base64 of the 32 raw bytes>
func HashPBKDF2(password string) (string, error) {
	salt := make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	s := base64.RawStdEncoding.EncodeToString(salt)
	return encodePBKDF2(password, s, DefaultPBKDF2Iterations), nil
}

func encodePBKDF2(password, salt string, iter int) string {
	sum := PBKDF2([]byte(password), []byte(salt), iter, sha256.Size)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iter, salt, base64.StdEncoding.EncodeToString(sum))
}

// CheckPBKDF2 reports whether the password matches the encoded hash. The
// comparison is constant-time; a hash it cannot read is a false, never a
// panic — a row written by another tool must not take the login down.
//
// It reads two spellings, told apart by the prefix, because the table it has to
// keep working is not always Django's:
//
//	pbkdf2_sha256$<iterations>$<salt>$<digest base64>   Django, passlib
//	pbkdf2$<iterations>$<salt hex>$<digest hex>         hashlib, written by hand
//
// The second is what a Python app writes when it uses neither Django nor
// passlib: hashlib.pbkdf2_hmac returns bytes and no format at all, so whoever
// stores it picks one, and hex is what they pick. There the salt is hex that
// was decoded to bytes before it went into the derivation, which is why reading
// it as text gives the wrong answer for the right password.
//
// SHA-256 only. HashPBKDF2 keeps writing the first spelling: one to write, two
// to read.
func CheckPBKDF2(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 || iter > 10_000_000 {
		return false
	}
	salt, want := []byte(parts[2]), []byte(nil)
	switch parts[0] {
	case "pbkdf2_sha256":
		if want, err = base64.StdEncoding.DecodeString(parts[3]); err != nil {
			return false
		}
	case "pbkdf2":
		if want, err = hex.DecodeString(parts[3]); err != nil {
			return false
		}
		// The salt is bytes if it reads as hex, and text if it does not:
		// both are out there, and only the hash itself can say which.
		if raw, err := hex.DecodeString(parts[2]); err == nil && len(raw) > 0 {
			salt = raw
		}
	default:
		return false
	}
	if len(want) == 0 {
		return false
	}
	got := PBKDF2([]byte(password), salt, iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1
}
