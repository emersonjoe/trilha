package auth

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// User is the authenticated person. It is what the app reads; the ID token
// itself never leaves this package, and the refresh token is never stored.
type User struct {
	Subject string `json:"sub"`
	Email   string `json:"email,omitempty"`
	// EmailVerified is the provider's word that the address is this person's,
	// and not merely what they typed into a form. Callback fills it from the
	// id_token; a session written before this field existed loads as false,
	// which is the safe value.
	//
	// With a provider it is read-only information. With auth.Sessions there is
	// no provider and no claim: it is the application's to set, and only the
	// application knows whether it confirmed the address.
	EmailVerified bool      `json:"email_verified,omitempty"`
	Name          string    `json:"name,omitempty"`
	Roles         []string  `json:"roles,omitempty"`
	IssuedAt      time.Time `json:"iat"`
	ExpiresAt     time.Time `json:"exp"`
	// Seen is the last activity, used for the idle timeout.
	Seen time.Time `json:"seen"`
	// SessionID changes on every login (session fixation).
	SessionID string `json:"sid"`
	// Tenant is the organisation this session is inside, for the most common
	// shape of multi-tenant: one column. It is a field of its own and not one
	// more entry in Extra because everything the framework does with it —
	// putting it in the audit trail, in the access log, refusing a session
	// without one — has to find it in the same place in every application.
	//
	// The framework carries it and points at the query that forgot it. The
	// query is yours: there is no ORM here, and a WHERE this package wrote
	// would be a WHERE nobody could read.
	Tenant string `json:"tenant,omitempty"`
	// Extra carries what this app's session needs and OIDC has no claim for:
	// the token the upstream wants, the tenant, the plan. It travels where the
	// rest of the session travels — the signed cookie, or the Store — so keep
	// it small and never put a password in it.
	Extra map[string]string `json:"extra,omitempty"`
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

// StoreContext is what a Store implements when it talks to something that can
// be slow, fail, or be worth cancelling — a table in Postgres, Redis, a
// service. The flow uses it when the Store has it and the three methods above
// when it does not, so no Store written against the older interface breaks.
//
// It exists because the three methods of Store have neither a context nor,
// for Load, an error. Without a context a remote store cannot honour the
// request's deadline, cancel the query when the browser goes away, or carry
// the trace: it is reduced to context.Background() and a made-up timeout, on
// every authenticated request. Without an error, a database that is down
// looks exactly like a session that does not exist, and everybody is sent to
// the login while no log says why.
//
//	func (s *PGStore) LoadContext(ctx context.Context, id string) (*auth.User, error) {
//		var blob []byte
//		err := s.db.QueryRowContext(ctx, "select u from sessoes where id = $1", id).Scan(&blob)
//		if errors.Is(err, sql.ErrNoRows) {
//			return nil, auth.ErrNoSession   // não existe
//		}
//		if err != nil {
//			return nil, err                 // o banco falhou: 503, não login
//		}
//		...
//	}
//
// LoadContext answers ErrNoSession when there is no such session. Any other
// error is the store having failed, and the guards turn it into 503 instead of
// a redirect to the login.
//
//	see: auth.Store, auth.SessionLister
type StoreContext interface {
	SaveContext(ctx context.Context, id string, u *User, ttl time.Duration) error
	LoadContext(ctx context.Context, id string) (*User, error)
	DeleteContext(ctx context.Context, id string) error
}

// ErrNoSession is returned by Session when nobody is logged in. It is also
// what a StoreContext answers for a session that is not there, which is what
// separates it from the store having failed.
var ErrNoSession = errors.New("auth: no session")

// storeLoad reads the session, through StoreContext when the Store has it.
// The error it gives back is ErrNoSession or the store's own.
func (a *Auth) storeLoad(ctx context.Context, id string) (*User, error) {
	if sc, ok := a.opts.Store.(StoreContext); ok {
		u, err := sc.LoadContext(ctx, id)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, ErrNoSession
		}
		return u, nil
	}
	u, ok := a.opts.Store.Load(id)
	if !ok {
		return nil, ErrNoSession
	}
	return u, nil
}

func (a *Auth) storeSave(ctx context.Context, id string, u *User, ttl time.Duration) error {
	if sc, ok := a.opts.Store.(StoreContext); ok {
		return sc.SaveContext(ctx, id, u, ttl)
	}
	return a.opts.Store.Save(id, u, ttl)
}

func (a *Auth) storeDelete(ctx context.Context, id string) error {
	if sc, ok := a.opts.Store.(StoreContext); ok {
		return sc.DeleteContext(ctx, id)
	}
	return a.opts.Store.Delete(id)
}

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
		stored, err := a.storeLoad(c.Context(), raw)
		if err != nil {
			return nil, err
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
		if err := a.storeSave(c.Context(), u.SessionID, u, ttl); err != nil {
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
			_ = a.storeDelete(c.Context(), id)
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

// ErrNoSessionList is what Sessions and LogoutOthers answer when the sessions
// cannot be listed: there is no Store, so each session lives in its own
// cookie and nobody holds the list; or the Store does not implement
// SessionLister. It is an error and not an empty list, because a screen that
// drew "no other sessions" over it would be lying.
var ErrNoSessionList = errors.New("auth: the session store cannot list sessions by subject")

// SessionLister is what a Store implements when it can answer "every session
// of this person". MemoryStore does; a table with a subject column does in
// one query. It is what the account screen's "open sessions" and "end the
// others" are built on, and what makes a password change able to end the
// sessions that were opened with the old one.
type SessionLister interface {
	Sessions(subject string) []*User
}

// Sessions lists the sessions of whoever is logged in, the current one first
// so a screen can mark it. Expired ones are not in it.
func (a *Auth) Sessions(c *trilha.Ctx) ([]User, error) {
	me, err := a.Session(c)
	if err != nil {
		return nil, err
	}
	lister, ok := a.opts.Store.(SessionLister)
	if !ok {
		return nil, ErrNoSessionList
	}
	all := lister.Sessions(me.Subject)
	out := make([]User, 0, len(all))
	for _, u := range all {
		if u.SessionID == me.SessionID {
			out = append([]User{*u}, out...)
			continue
		}
		out = append(out, *u)
	}
	return out, nil
}

// LoginPath is where Require sends an anonymous browser — Options.LoginPath,
// or its default. It is what a screen that lives elsewhere links to, and what
// a test posts to, instead of a path copied from where the login happened to
// be written.
func (a *Auth) LoginPath() string { return a.opts.LoginPath }

// LogoutOthers ends every other session of whoever is logged in and keeps
// this one. It is what a password change calls, and what the account screen
// offers as a button: the machine somebody forgot to lock stops being a
// session without the person having to find it.
func (a *Auth) LogoutOthers(c *trilha.Ctx) error {
	me, err := a.Session(c)
	if err != nil {
		return err
	}
	lister, ok := a.opts.Store.(SessionLister)
	if !ok {
		return ErrNoSessionList
	}
	n := 0
	for _, u := range lister.Sessions(me.Subject) {
		if u.SessionID == me.SessionID {
			continue
		}
		if err := a.storeDelete(c.Context(), u.SessionID); err != nil {
			return err
		}
		n++
	}
	c.Audit("auth.logout_others", me.Subject, trilha.Fields{"sessions": n})
	return nil
}

// Sessions is SessionLister for the in-process store.
func (m *MemoryStore) Sessions(subject string) []*User {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*User
	for _, e := range m.data {
		if e.user.Subject != subject || time.Now().After(e.exp) {
			continue
		}
		cp := *e.user
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IssuedAt.After(out[j].IssuedAt) })
	return out
}
