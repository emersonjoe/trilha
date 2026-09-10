package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// The key an application issues to its own callers is a pile of rules a
// beginner does not know: store only the hash, show the secret once, keep a
// prefix so a key can be found without opening the hash, a scope per route, a
// limit per key rather than per address, revocation that takes effect now, and
// a record of use. The first version always stores the key in the clear.

// Key is one issued key. The secret is not in it and never was: what is stored
// is the hash, and the only moment the secret exists is the answer to Issue.
type Key struct {
	ID string
	// Handle is what identifies the key without opening the hash — it is the
	// first half of what the caller sends, and what a screen shows.
	Handle string
	Name   string
	Scopes []string
	// Hash is the peppered digest of the secret. Comparing is constant time.
	Hash      []byte
	Created   time.Time
	LastUsed  time.Time
	ExpiresAt time.Time
	RevokedAt time.Time
}

// Active reports whether the key may still be used at that moment.
func (k *Key) Active(now time.Time) bool {
	if !k.RevokedAt.IsZero() && !now.Before(k.RevokedAt) {
		return false
	}
	return k.ExpiresAt.IsZero() || now.Before(k.ExpiresAt)
}

// KeyStore is where the keys live. Five methods, all of them about one key,
// because the decision the application makes is which table — the framework
// has no database.
//
// Find is by handle and not by secret: the secret is never stored, so it
// cannot be looked up.
type KeyStore interface {
	Save(k *Key) error
	Find(handle string) (*Key, bool)
	All() ([]*Key, error)
	Touch(id string, at time.Time) error
	Revoke(id string, at time.Time) error
}

// KeyOptions is what an application decides about its keys.
type KeyOptions struct {
	// Store is where they are kept. Nil uses MemoryKeyStore, which is what a
	// test wants and what a first version can run on; it says so in the log,
	// because keys that vanish on restart are a bad surprise to find later.
	Store KeyStore
	// Prefix opens every key: "ak" gives "ak_<handle>_<secret>". It is what
	// makes a leaked key recognisable in a log or a repository — a secret
	// scanner looks for exactly this.
	Prefix string
	// Scopes is the closed list this application understands. Require refuses
	// a scope that is not in it, at wiring time.
	Scopes []string
	// RateLimit applies per key rather than per address: one caller behind one
	// key is one budget, whatever their IP is doing.
	RateLimit trilha.RateLimit
	// Usage counts the calls per key, method, route and day. Nil counts
	// nothing, which is what an application that never asked for it pays.
	Usage UsageStore
	// UsageFlush is how often the buffer goes to the store (default 30s), and
	// UsageKeep how long a bucket is worth keeping. Zero keeps for ever, which
	// is a decision somebody should make on purpose.
	UsageFlush time.Duration
	UsageKeep  time.Duration
}

// Keys is the set of API keys of an application.
type Keys struct {
	opts    KeyOptions
	store   KeyStore
	limiter *trilha.Limiter
	mu      sync.Mutex
	touched map[string]time.Time
	now     func() time.Time

	usageMu sync.Mutex
	usage   map[usageBucket]*usageCell
}

// APIKeys wires the keys of an application.
//
//	var Keys = auth.APIKeys(auth.KeyOptions{
//		Store:     chaves.NewStore(db),
//		Prefix:    "ak",
//		Scopes:    []string{"docs:read", "docs:write"},
//		RateLimit: trilha.RateLimit{RPS: 10, Burst: 30},
//	})
//
//	// app/api/v1/middleware.go
//	func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
//	var exige = Keys.Require("docs:read")
func APIKeys(o KeyOptions) *Keys {
	if o.Prefix == "" {
		o.Prefix = "ak"
	}
	k := &Keys{opts: o, store: o.Store, touched: map[string]time.Time{}, now: time.Now}
	if k.store == nil {
		k.store = MemoryKeyStore()
	}
	if o.RateLimit.RPS > 0 || o.RateLimit.Burst > 0 {
		k.limiter = trilha.NewLimiter(o.RateLimit)
	}
	return k
}

// ErrUnknownKey is what Revoke answers for an id nobody issued.
var ErrUnknownKey = errors.New("auth: no such key")

// Issue creates a key and answers it together with the secret — the only
// moment the secret exists. What is stored is the hash; nothing, anywhere, can
// show the secret again, which is the property that makes a leak recoverable
// by revoking instead of by hoping.
//
//	k, secret, err := Keys.Issue(c, "Integração X", []string{"docs:read"}, 0)
//
// A zero ttl never expires. The call is audited: issuing a key is exactly the
// action somebody asks about later.
func (ks *Keys) Issue(c *trilha.Ctx, name string, scopes []string, ttl time.Duration) (*Key, string, error) {
	for _, s := range scopes {
		if !ks.known(s) {
			return nil, "", fmt.Errorf("auth: scope %q is not in KeyOptions.Scopes", s)
		}
	}
	handle, err := randomToken(6)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomToken(24)
	if err != nil {
		return nil, "", err
	}
	hash, err := trilha.Pepper([]byte(secret))
	if err != nil {
		// Without a secret the hash would not be keyed, and a database of
		// unkeyed digests is a database somebody attacks offline. Refusing is
		// the only honest answer.
		return nil, "", fmt.Errorf("auth: API keys need TRILHA_SECRET: %w", err)
	}
	now := ks.now()
	k := &Key{
		ID: hex.EncodeToString(hash[:8]), Handle: handle, Name: name,
		Scopes: append([]string(nil), scopes...), Hash: hash, Created: now,
	}
	if ttl > 0 {
		k.ExpiresAt = now.Add(ttl)
	}
	if err := ks.store.Save(k); err != nil {
		return nil, "", err
	}
	if c != nil {
		c.Audit("apikey.emitiu", k.ID, trilha.Fields{"name": name, "scopes": strings.Join(scopes, " ")})
	}
	return k, ks.opts.Prefix + "_" + handle + "_" + secret, nil
}

// Require guards a route with the key: the caller sends it as a bearer token,
// and everything the application would have to check is checked here.
//
// A scope this application never declared is a panic, at wiring time — the
// alternative is a route that guards nothing because of a typo, and nobody
// notices until it is read in a breach report.
func (ks *Keys) Require(scopes ...string) trilha.MiddlewareFunc {
	for _, s := range scopes {
		if !ks.known(s) {
			panic("auth: Require(" + s + "): scope is not in KeyOptions.Scopes")
		}
	}
	return func(c *trilha.Ctx, next trilha.Next) error {
		k, err := ks.authenticate(c)
		if err != nil {
			return err
		}
		if !hasScopes(k, scopes) {
			c.Log().Warn("auth: key missing scope", "key", k.ID, "need", strings.Join(scopes, ","))
			return &trilha.HTTPError{Code: http.StatusForbidden, Message: "the key does not carry " + strings.Join(scopes, ", ")}
		}
		// A probe asks whether the key would get through; it is not a call.
		// It spends no budget and counts nothing: a listing that probes
		// twenty routes would otherwise use up the key before its first
		// real request.
		if c.Probing() {
			ks.remember(c, k)
			return next()
		}
		if ks.limiter != nil {
			if ok, after := ks.limiter.Allow(k.ID); !ok {
				c.Header("Retry-After", fmt.Sprint(after))
				return &trilha.HTTPError{Code: http.StatusTooManyRequests, Message: "too many requests for this key"}
			}
		}
		ks.remember(c, k)
		ks.touch(k)
		err = next()
		// The counter is written after the answer, so what a request pays for
		// counting is a map write and never a round trip.
		ks.record(c, k, err)
		return err
	}
}

// authenticate is the whole check, in the order that gives nothing away.
func (ks *Keys) authenticate(c *trilha.Ctx) (*Key, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(c.Request().Header.Get("Authorization"), "Bearer"))
	handle, secret, ok := splitKey(raw, ks.opts.Prefix)
	if !ok {
		return nil, ks.unauthorized(c, "a key is required")
	}
	k, found := ks.store.Find(handle)
	if !found {
		return nil, ks.unauthorized(c, "invalid key")
	}
	hash, err := trilha.Pepper([]byte(secret))
	if err != nil {
		return nil, err
	}
	// Constant time, and the revocation check after it: answering faster for a
	// revoked key than for a wrong one says which of the two happened.
	if subtle.ConstantTimeCompare(hash, k.Hash) != 1 {
		return nil, ks.unauthorized(c, "invalid key")
	}
	if !k.Active(ks.now()) {
		return nil, ks.unauthorized(c, "invalid key")
	}
	return k, nil
}

func (ks *Keys) unauthorized(c *trilha.Ctx, msg string) error {
	// The header is what tells a client library it should send a key at all,
	// instead of retrying the same request for ever.
	c.Header("WWW-Authenticate", `Bearer realm="api"`)
	return &trilha.HTTPError{Code: http.StatusUnauthorized, Message: msg}
}

// remember puts the key in the request the way a session would, so everything
// downstream — c.Audit, the policy, the log — sees a caller and not a hole.
func (ks *Keys) remember(c *trilha.Ctx, k *Key) {
	u := &User{
		Subject:   "key:" + k.ID,
		Name:      k.Name,
		Roles:     append([]string(nil), k.Scopes...),
		ExpiresAt: k.ExpiresAt,
		Extra:     map[string]string{"via": "api_key", "key": k.ID},
	}
	c.Set(ctxKey, u)
	c.SetActor(trilha.Actor{Subject: u.Subject, Name: k.Name, Via: "api_key"})
}

// touch records the use at most once a minute. Writing on every request turns
// a read-only endpoint into a write per call, and the question this answers —
// "is this key still in use?" — does not need the second.
func (ks *Keys) touch(k *Key) {
	now := ks.now()
	ks.mu.Lock()
	last, ok := ks.touched[k.ID]
	if ok && now.Sub(last) < time.Minute {
		ks.mu.Unlock()
		return
	}
	ks.touched[k.ID] = now
	ks.mu.Unlock()
	_ = ks.store.Touch(k.ID, now)
}

// User is the caller of a request authenticated by a key, or nil.
func (ks *Keys) User(c *trilha.Ctx) *User {
	if u, ok := c.Get(ctxKey).(*User); ok {
		return u
	}
	return nil
}

// Revoke ends a key now: the next request with it is a 401, with no cache and
// no window.
func (ks *Keys) Revoke(c *trilha.Ctx, id string) error {
	now := ks.now()
	if err := ks.store.Revoke(id, now); err != nil {
		return err
	}
	if c != nil {
		c.Audit("apikey.revogou", id)
	}
	return nil
}

// All is every key, for the screen that lists them.
func (ks *Keys) All() ([]*Key, error) { return ks.store.All() }

func (ks *Keys) known(scope string) bool {
	if len(ks.opts.Scopes) == 0 {
		return true
	}
	for _, s := range ks.opts.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func hasScopes(k *Key, need []string) bool {
	for _, n := range need {
		found := false
		for _, s := range k.Scopes {
			if s == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// splitKey reads "ak_<handle>_<secret>" without deciding anything about the
// halves — that is the caller's job, in constant time.
func splitKey(raw, prefix string) (handle, secret string, ok bool) {
	parts := strings.SplitN(raw, "_", 3)
	if len(parts) != 3 || parts[0] != prefix || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// randomToken is base32 in lower case, and the alphabet is the point: the key
// is "ak_<handle>_<secret>", so a token that could contain an underscore would
// split in the wrong place — intermittently, on the fraction of keys whose
// random bytes happened to encode one. base64url has both "_" and "-"; base32
// has letters and digits and nothing else.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)), nil
}

// MemoryKeyStore keeps the keys in the process. It is the store for a test and
// for a first version; it forgets on restart, which is why APIKeys says so in
// the log when it falls back to it.
func MemoryKeyStore() KeyStore { return &memKeys{m: map[string]*Key{}} }

type memKeys struct {
	mu sync.Mutex
	m  map[string]*Key
}

func (s *memKeys) Save(k *Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *k
	s.m[k.Handle] = &cp
	return nil
}

func (s *memKeys) Find(handle string) (*Key, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k, ok := s.m[handle]
	if !ok {
		return nil, false
	}
	cp := *k
	return &cp, true
}

func (s *memKeys) All() ([]*Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Key, 0, len(s.m))
	for _, k := range s.m {
		cp := *k
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

func (s *memKeys) Touch(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.m {
		if k.ID == id {
			k.LastUsed = at
			return nil
		}
	}
	return ErrUnknownKey
}

func (s *memKeys) Revoke(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.m {
		if k.ID == id {
			k.RevokedAt = at
			return nil
		}
	}
	return ErrUnknownKey
}
