package auth

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// User is the authenticated person. It is what the app reads; the ID token
// itself never leaves this package, and the refresh token is never stored.
type User struct {
	Subject   string    `json:"sub"`
	Email     string    `json:"email,omitempty"`
	Name      string    `json:"name,omitempty"`
	Roles     []string  `json:"roles,omitempty"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
	// Seen is the last activity, used for the idle timeout.
	Seen time.Time `json:"seen"`
	// SessionID changes on every login (session fixation).
	SessionID string `json:"sid"`
}

// HasRole reports whether the user carries the role, case-insensitively.
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// Store persists sessions when the app needs immediate revocation or a
// session too large for a cookie. Nil means the stateless default: the
// session travels in a signed cookie.
type Store interface {
	Save(id string, u *User, ttl time.Duration) error
	Load(id string) (*User, bool)
	Delete(id string) error
}

// ErrNoSession is returned by Session when nobody is logged in.
var ErrNoSession = errors.New("auth: no session")

// Session reads and validates the session cookie. It renews the idle window
// when more than a minute has passed, so a busy session does not rewrite the
// cookie on every request.
func (a *Auth) Session(c *trilha.Ctx) (*User, error) {
	raw, ok := c.Signed(a.opts.CookieName)
	if !ok {
		return nil, ErrNoSession
	}
	var u User
	if a.opts.Store != nil {
		stored, ok := a.opts.Store.Load(raw)
		if !ok {
			return nil, ErrNoSession
		}
		u = *stored
	} else if err := json.Unmarshal([]byte(raw), &u); err != nil {
		return nil, ErrNoSession
	}
	now := time.Now()
	if now.After(u.ExpiresAt) {
		return nil, ErrNoSession
	}
	if a.opts.Idle > 0 && now.Sub(u.Seen) > a.opts.Idle {
		return nil, ErrNoSession
	}
	if now.Sub(u.Seen) > time.Minute {
		u.Seen = now
		_ = a.write(c, &u)
	}
	return &u, nil
}

// login creates a fresh session. The identifier is new on every login, which
// is what makes a fixated cookie useless to an attacker.
func (a *Auth) login(c *trilha.Ctx, u *User) error {
	now := time.Now()
	u.IssuedAt, u.Seen = now, now
	u.ExpiresAt = now.Add(a.opts.Absolute)
	u.SessionID = randomID()
	return a.write(c, u)
}

func (a *Auth) write(c *trilha.Ctx, u *User) error {
	ttl := time.Until(u.ExpiresAt)
	if ttl <= 0 {
		return errors.New("auth: session already expired")
	}
	if a.opts.Store != nil {
		if err := a.opts.Store.Save(u.SessionID, u, ttl); err != nil {
			return err
		}
		return c.SetSigned(a.opts.CookieName, u.SessionID, ttl)
	}
	b, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return c.SetSigned(a.opts.CookieName, string(b), ttl)
}

// clear removes the session, from the store too when there is one.
func (a *Auth) clear(c *trilha.Ctx) {
	if a.opts.Store != nil {
		if id, ok := c.Signed(a.opts.CookieName); ok {
			_ = a.opts.Store.Delete(id)
		}
	}
	c.ClearCookie(a.opts.CookieName)
}

// MemoryStore is a Store for a single process: it gives immediate logout and
// revocation, and loses every session on restart. Replicas do not share it.
type MemoryStore struct {
	mu   sync.Mutex
	data map[string]memEntry
}

type memEntry struct {
	user *User
	exp  time.Time
}

// NewMemoryStore returns an empty in-process store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{data: map[string]memEntry{}} }

// Save stores the session.
func (m *MemoryStore) Save(id string, u *User, ttl time.Duration) error {
	cp := *u
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.data)%128 == 0 {
		for k, v := range m.data {
			if time.Now().After(v.exp) {
				delete(m.data, k)
			}
		}
	}
	m.data[id] = memEntry{user: &cp, exp: time.Now().Add(ttl)}
	return nil
}

// Load returns the session when it exists and has not expired.
func (m *MemoryStore) Load(id string) (*User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[id]
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	cp := *e.user
	return &cp, true
}

// Delete forgets the session.
func (m *MemoryStore) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
}
