// Package auth adds OpenID Connect login, signed sessions and role checks to a
// Trilha app, using only the standard library. The package registers no route:
// it exposes handlers that the app exposes under app/, as every other route.
package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"strings"
	"time"
)

// clockSkew is how far the token clock may drift from ours. A server whose
// clock is a few seconds off is the most common cause of "the login only
// fails in production".
const clockSkew = 60 * time.Second

// allowedAlgs are the signature algorithms we accept. The list is fixed on
// purpose: reading the algorithm from the token and trusting it is how JWT
// libraries get broken ("none", or an RSA key verified as an HMAC secret).
var allowedAlgs = map[string]crypto.Hash{
	"RS256": crypto.SHA256, "RS384": crypto.SHA384, "RS512": crypto.SHA512,
	"ES256": crypto.SHA256, "ES384": crypto.SHA384,
}

// Claims is the decoded payload of an ID token. Everything the standard
// defines is typed; the rest stays in All.
type Claims struct {
	Issuer    string
	Subject   string
	Audience  []string
	Email     string
	Name      string
	Nonce     string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
	All       map[string]any
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// parseJWS splits a compact JWS, checks the header and returns the pieces.
func parseJWS(token string) (jwtHeader, []byte, []byte, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtHeader{}, nil, nil, nil, errors.New("token is not a compact JWS")
	}
	headerRaw, err := b64(parts[0])
	if err != nil {
		return jwtHeader{}, nil, nil, nil, fmt.Errorf("unreadable header: %w", err)
	}
	var hdr jwtHeader
	if err := json.Unmarshal(headerRaw, &hdr); err != nil {
		return jwtHeader{}, nil, nil, nil, fmt.Errorf("unreadable header: %w", err)
	}
	if _, ok := allowedAlgs[hdr.Alg]; !ok {
		return jwtHeader{}, nil, nil, nil, fmt.Errorf("algorithm not accepted: %q", hdr.Alg)
	}
	if hdr.Kid == "" {
		return jwtHeader{}, nil, nil, nil, errors.New("token without kid: cannot pick a key")
	}
	payload, err := b64(parts[1])
	if err != nil {
		return jwtHeader{}, nil, nil, nil, fmt.Errorf("unreadable payload: %w", err)
	}
	sig, err := b64(parts[2])
	if err != nil {
		return jwtHeader{}, nil, nil, nil, fmt.Errorf("unreadable signature: %w", err)
	}
	signed := []byte(parts[0] + "." + parts[1])
	return hdr, payload, sig, signed, nil
}

func b64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

// verifySignature checks the signature of signed with key.
func verifySignature(alg string, key any, signed, sig []byte) error {
	h := allowedAlgs[alg]
	var d hash.Hash
	switch h {
	case crypto.SHA256:
		d = sha256.New()
	case crypto.SHA384:
		d = sha512.New384()
	default:
		d = sha512.New()
	}
	d.Write(signed)
	sum := d.Sum(nil)
	switch k := key.(type) {
	case *rsa.PublicKey:
		if !strings.HasPrefix(alg, "RS") {
			return errors.New("RSA key with a non-RSA algorithm")
		}
		return rsa.VerifyPKCS1v15(k, h, sum, sig)
	case *ecdsa.PublicKey:
		if !strings.HasPrefix(alg, "ES") {
			return errors.New("EC key with a non-EC algorithm")
		}
		n := (k.Curve.Params().BitSize + 7) / 8
		if len(sig) != 2*n {
			return errors.New("EC signature with wrong size")
		}
		r := new(big.Int).SetBytes(sig[:n])
		s := new(big.Int).SetBytes(sig[n:])
		if !ecdsa.Verify(k, sum, r, s) {
			return errors.New("invalid EC signature")
		}
		return nil
	}
	return errors.New("unsupported key type")
}

// decodeClaims reads the payload into Claims.
func decodeClaims(payload []byte) (*Claims, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("unreadable payload: %w", err)
	}
	c := &Claims{All: raw}
	c.Issuer, _ = raw["iss"].(string)
	c.Subject, _ = raw["sub"].(string)
	c.Email, _ = raw["email"].(string)
	c.Name, _ = raw["name"].(string)
	c.Nonce, _ = raw["nonce"].(string)
	switch aud := raw["aud"].(type) {
	case string:
		c.Audience = []string{aud}
	case []any:
		for _, v := range aud {
			if s, ok := v.(string); ok {
				c.Audience = append(c.Audience, s)
			}
		}
	}
	c.ExpiresAt = unixClaim(raw["exp"])
	c.IssuedAt = unixClaim(raw["iat"])
	c.NotBefore = unixClaim(raw["nbf"])
	return c, nil
}

func unixClaim(v any) time.Time {
	f, ok := v.(float64)
	if !ok {
		return time.Time{}
	}
	return time.Unix(int64(f), 0)
}

// validate checks the registered claims. The signature is checked by the
// caller, which owns the key set.
func (c *Claims) validate(issuer, clientID, nonce string, now time.Time) error {
	if c.Issuer != issuer {
		return fmt.Errorf("unexpected issuer: %q", c.Issuer)
	}
	if c.Subject == "" {
		return errors.New("token without sub")
	}
	found := false
	for _, a := range c.Audience {
		if a == clientID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("token issued for another client")
	}
	if azp, ok := c.All["azp"].(string); ok && azp != "" && azp != clientID {
		return errors.New("azp of another client")
	}
	if c.ExpiresAt.IsZero() || now.After(c.ExpiresAt.Add(clockSkew)) {
		return errors.New("token expired")
	}
	if !c.NotBefore.IsZero() && now.Add(clockSkew).Before(c.NotBefore) {
		return errors.New("token not yet valid")
	}
	if nonce != "" && c.Nonce != nonce {
		return errors.New("nonce differs from the one sent")
	}
	return nil
}
