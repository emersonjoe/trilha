package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksTTL is how long a key set is reused, and refetchMin is the shortest gap
// between two fetches: a token with an unknown kid must not turn into one HTTP
// request per request (an amplification vector).
const (
	jwksTTL    = time.Hour
	refetchMin = time.Minute
)

type jwks struct {
	url    string
	client *http.Client

	mu      sync.Mutex
	keys    map[string]any
	fetched time.Time
	last    time.Time
}

func newJWKS(url string, client *http.Client) *jwks {
	return &jwks{url: url, client: client, keys: map[string]any{}}
}

// key returns the public key for kid, fetching the set when it is unknown or
// stale (the provider rotates keys without warning).
func (j *jwks) key(ctx context.Context, kid string) (any, error) {
	j.mu.Lock()
	k, ok := j.keys[kid]
	fresh := time.Since(j.fetched) < jwksTTL
	canFetch := time.Since(j.last) > refetchMin || j.fetched.IsZero()
	j.mu.Unlock()
	if ok && fresh {
		return k, nil
	}
	if !ok && !canFetch {
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	if err := j.fetch(ctx); err != nil {
		if ok {
			return k, nil // stale but usable: better than refusing every login
		}
		return nil, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key %q", kid)
}

type jwkDoc struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
		Crv string `json:"crv"`
		X   string `json:"x"`
		Y   string `json:"y"`
	} `json:"keys"`
}

func (j *jwks) fetch(ctx context.Context) error {
	j.mu.Lock()
	j.last = time.Now()
	j.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("JWKS unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS answered %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var doc jwkDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("JWKS unreadable: %w", err)
	}
	keys := map[string]any{}
	for _, k := range doc.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		pub, err := parseJWK(k.Kty, k.Crv, k.N, k.E, k.X, k.Y)
		if err != nil || k.Kid == "" {
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("JWKS has no usable signing key")
	}
	j.mu.Lock()
	j.keys, j.fetched = keys, time.Now()
	j.mu.Unlock()
	return nil
}

func parseJWK(kty, crv, n, e, x, y string) (any, error) {
	switch kty {
	case "RSA":
		nb, err := base64.RawURLEncoding.DecodeString(n)
		if err != nil {
			return nil, err
		}
		eb, err := base64.RawURLEncoding.DecodeString(e)
		if err != nil {
			return nil, err
		}
		if len(nb) < 256 {
			return nil, errors.New("RSA modulus smaller than 2048 bits")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(new(big.Int).SetBytes(eb).Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		default:
			return nil, fmt.Errorf("unsupported curve: %q", crv)
		}
		xb, err := base64.RawURLEncoding.DecodeString(x)
		if err != nil {
			return nil, err
		}
		yb, err := base64.RawURLEncoding.DecodeString(y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(xb), Y: new(big.Int).SetBytes(yb)}, nil
	}
	return nil, fmt.Errorf("unsupported key type: %q", kty)
}
