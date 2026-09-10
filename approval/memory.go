package approval

import (
	"context"
	"sort"
	"sync"
)

// Memory keeps the queue in this process. It is the right store for one
// server and for every test, and it is what makes the screens work from the
// first request — a table behind the same three methods changes nothing above.
func Memory() Store { return &memory{rows: map[string]Record{}} }

type memory struct {
	mu   sync.RWMutex
	rows map[string]Record
}

func (m *memory) Save(_ context.Context, r Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[r.ID] = r
	return nil
}

func (m *memory) Get(_ context.Context, id string) (Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rows[id]
	if !ok {
		return Record{}, ErrUnknown
	}
	return r, nil
}

// List answers oldest first: a queue people work through is worked from the
// top, and the top is what has been waiting longest.
func (m *memory) List(_ context.Context, p ListParams) ([]Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Record, 0, len(m.rows))
	for _, r := range m.rows {
		if p.Kind != "" && r.Kind != p.Kind {
			continue
		}
		if p.State != "" && r.State != p.State {
			continue
		}
		if !matches(r, p) {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Opened.Equal(out[j].Opened) {
			return out[i].ID < out[j].ID
		}
		return out[i].Opened.Before(out[j].Opened)
	})
	if p.Limit > 0 && len(out) > p.Limit {
		out = out[:p.Limit]
	}
	return out, nil
}

// matches is the filter of an inbox: a request assigned to this person, or to
// one of their roles. A listing with no subject and no roles is the
// administration screen, and it sees everything.
func matches(r Record, p ListParams) bool {
	if p.Subject == "" && len(p.Roles) == 0 {
		return true
	}
	if r.Assign.User != "" {
		return r.Assign.User == p.Subject
	}
	for _, role := range p.Roles {
		if role == r.Assign.Role {
			return true
		}
	}
	return false
}
