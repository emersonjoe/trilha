package webhook

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Store is where subscriptions and deliveries live. Six methods, and the one
// worth reading twice is Due: it is what turns "try again in twelve hours"
// into a row with a time on it instead of a sleeping goroutine a deploy would
// forget.
//
// There is no SQL implementation here, the same choice every store in this
// framework makes — it would have to pick a placeholder dialect and own a
// DDL. The recipe carries the whole file.
type Store interface {
	SaveSubscription(ctx context.Context, s Subscription) error
	Subscription(ctx context.Context, id string) (Subscription, error)
	// Subscriptions answers a tenant's subscriptions, revoked ones included:
	// the screen has to show them, because the deliveries already made point
	// at them and "which endpoint was that?" is the question somebody asks
	// afterwards. Emit filters with Subscription.Wants, which is where being
	// revoked stops a delivery.
	//
	// An empty tenant is an application without them, and answers all of them.
	Subscriptions(ctx context.Context, tenant string) ([]Subscription, error)

	SaveDelivery(ctx context.Context, d Delivery) error
	Delivery(ctx context.Context, id string) (Delivery, error)
	// Deliveries answers what a screen shows, newest first.
	Deliveries(ctx context.Context, p ListParams) ([]Delivery, error)
	// Due answers the ids of pending deliveries whose next attempt has
	// arrived, oldest first, at most limit of them.
	Due(ctx context.Context, at time.Time, limit int) ([]string, error)
}

// ListParams filters the deliveries screen. The zero value is "the last fifty
// of everything".
type ListParams struct {
	Tenant         string
	SubscriptionID string
	Event          string
	State          State
	Limit          int
	Offset         int
}

// Memory keeps everything in memory: right for a single process, and for the
// tests of an application that emits events.
func Memory() Store {
	return &memory{subs: map[string]Subscription{}, dels: map[string]Delivery{}}
}

type memory struct {
	mu   sync.RWMutex
	subs map[string]Subscription
	dels map[string]Delivery
}

func (m *memory) SaveSubscription(ctx context.Context, s Subscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[s.ID] = s
	return nil
}

func (m *memory) Subscription(ctx context.Context, id string) (Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.subs[id]
	if !ok {
		return Subscription{}, ErrNotFound
	}
	return s, nil
}

func (m *memory) Subscriptions(ctx context.Context, tenant string) ([]Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Subscription
	for _, s := range m.subs {
		if tenant != "" && s.Tenant != tenant {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memory) SaveDelivery(ctx context.Context, d Delivery) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dels[d.ID] = d
	return nil
}

func (m *memory) Delivery(ctx context.Context, id string) (Delivery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.dels[id]
	if !ok {
		return Delivery{}, ErrNotFound
	}
	return d, nil
}

func (m *memory) Deliveries(ctx context.Context, p ListParams) ([]Delivery, error) {
	m.mu.RLock()
	var out []Delivery
	for _, d := range m.dels {
		switch {
		case p.Tenant != "" && d.Tenant != p.Tenant:
		case p.SubscriptionID != "" && d.SubscriptionID != p.SubscriptionID:
		case p.Event != "" && d.Event != p.Event:
		case p.State != "" && d.State != p.State:
		default:
			out = append(out, d)
		}
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if p.Offset > 0 {
		if p.Offset >= len(out) {
			return nil, nil
		}
		out = out[p.Offset:]
	}
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memory) Due(ctx context.Context, at time.Time, limit int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var due []Delivery
	for _, d := range m.dels {
		if d.State == Pending && !d.NextTry.After(at) {
			due = append(due, d)
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].NextTry.Before(due[j].NextTry) })
	if limit > 0 && len(due) > limit {
		due = due[:limit]
	}
	out := make([]string, 0, len(due))
	for _, d := range due {
		out = append(out, d.ID)
	}
	return out, nil
}

// idSeq breaks the tie between two ids made in the same millisecond, so the
// deliveries screen has a stable order for two events emitted at once.
var idSeq atomic.Uint32

// newID is the timestamp, then the sequence, then randomness: sortable, so
// newest-first is a string comparison, and unguessable, because the id travels
// to the partner as the idempotency key.
func newID(prefix string) string {
	ms := time.Now().UnixMilli()
	var b [16]byte
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	n := idSeq.Add(1)
	for i := 9; i >= 6; i-- {
		b[i] = byte(n)
		n >>= 8
	}
	rand.Read(b[10:])
	return prefix + "_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
}

// newSecret is what signs the deliveries. Base32 without padding, lowercase:
// it goes into somebody else's configuration file by copy and paste, and a
// character that needs escaping there is a support message.
func newSecret() string {
	var b [32]byte
	rand.Read(b[:])
	return "whsec_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
}
