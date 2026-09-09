package task

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

// Store is where the state lives. Five methods, and four of them are obvious;
// the fifth is Interrupt, which is the one an application would not have
// thought to write.
//
// There is no SQL implementation here on purpose, and it is the same choice
// every other store in this framework makes: a shipped SQL store has to pick a
// placeholder dialect and own a DDL, which is exactly what the framework does
// not do. The recipe carries the whole implementation, and it is forty lines.
type Store interface {
	// Save writes the task, creating or replacing it whole.
	Save(ctx context.Context, t Task) error
	// Get answers ErrNotFound for an id it does not have.
	Get(ctx context.Context, id string) (Task, error)
	// List answers the tasks a screen shows, newest first.
	List(ctx context.Context, p ListParams) ([]Task, error)
	// Active answers the id of a queued or running task with this name and
	// key, or "" when there is none. It is the deduplication.
	Active(ctx context.Context, name, key string) (string, error)
	// Interrupt marks everything still queued or running as interrupted, and
	// answers how many. It runs once at boot: those tasks belong to a process
	// that no longer exists, and leaving them "running" is how a screen ends
	// up waiting forever for something nobody is doing.
	Interrupt(ctx context.Context, at time.Time) (int, error)
}

// ListParams filters the administration screen. Everything is optional, and
// the zero value is "the last fifty of everything".
type ListParams struct {
	Name   string
	Key    string
	State  State
	Limit  int
	Offset int
}

// Memory is the store for a single process that does not need the history to
// survive a restart — which is most of them, since the tasks themselves do not
// survive one either.
func Memory() Store { return &memory{rows: map[string]Task{}} }

type memory struct {
	mu   sync.RWMutex
	rows map[string]Task
}

func (m *memory) Save(ctx context.Context, t Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[t.ID] = t
	return nil
}

func (m *memory) Get(ctx context.Context, id string) (Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.rows[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	return t, nil
}

func (m *memory) List(ctx context.Context, p ListParams) ([]Task, error) {
	m.mu.RLock()
	var out []Task
	for _, t := range m.rows {
		switch {
		case p.Name != "" && t.Name != p.Name:
		case p.Key != "" && t.Key != p.Key:
		case p.State != "" && t.State != p.State:
		default:
			out = append(out, t)
		}
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Queued.Equal(out[j].Queued) {
			return out[i].ID > out[j].ID
		}
		return out[i].Queued.After(out[j].Queued)
	})
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

func (m *memory) Active(ctx context.Context, name, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.rows {
		if t.Name == name && t.Key == key && t.State.Live() {
			return t.ID, nil
		}
	}
	return "", nil
}

func (m *memory) Interrupt(ctx context.Context, at time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, t := range m.rows {
		if !t.State.Live() {
			continue
		}
		t.State, t.Err, t.Ended = Interrupted, "the process stopped before this finished", at
		m.rows[id] = t
		n++
	}
	return n, nil
}

// idSeq breaks the tie between two tasks queued in the same millisecond. It
// is not decoration: without it the administration screen shows two
// simultaneous tasks in whatever order the random bytes fell in, which reads
// as a bug every time somebody presses two buttons quickly.
var idSeq atomic.Uint32

// newID is the timestamp, then the sequence, then randomness — sortable, so
// newest-first is a string comparison, and unguessable, so an id says nothing
// about how many there have been.
func newID() string {
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
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:]))
}
