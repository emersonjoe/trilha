package trilha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Versioned keeps a numbered history of one kind of thing, with one version
// marked as the published one.
//
// It is the pattern two different screens of the same application always end
// up writing twice: a table of versions, "the published one is frozen", and a
// button that goes back to version N. What changes between the two is the
// struct being kept, which is what the type parameter is for.
//
//	var Modelos = trilha.NewVersioned[Modelo]("modelos", trilha.VersionedOpts{})
//
//	n, _ := Modelos.Draft(c, id)                // n+1, a copy of the current one
//	n, _ = Modelos.Save(c, id, m, "adjusted")   // writes into the open draft
//	_ = Modelos.Publish(c, id, n)               // freezes n and makes it current
//
// The value is stored as JSON, which is what lets the history outlive a field
// being added to the struct: an old version reads back with the new field at
// its zero value instead of failing to load.
type Versioned[T any] struct {
	key   string
	store VersionStore
	now   func() time.Time
}

// Version is one entry of the history — everything about a version except the
// value itself.
type Version struct {
	N         int
	By        string
	At        time.Time
	Published bool
	Note      string
}

// VersionRecord is what a store keeps: the entry, plus the value as JSON.
//
// The store is not generic on purpose: a table of versions holds bytes, and an
// interface with a type parameter would need one implementation per struct in
// the application.
type VersionRecord struct {
	Version
	Data []byte
}

// VersionStore is where the history lives. Memory is the default; a table
// behind the same three methods is the next step, and no screen changes.
type VersionStore interface {
	Save(ctx context.Context, key, id string, r VersionRecord) error
	List(ctx context.Context, key, id string) ([]VersionRecord, error)
	Get(ctx context.Context, key, id string, n int) (VersionRecord, error)
}

// VersionedOpts configures the history.
type VersionedOpts struct {
	Store VersionStore
}

// ErrVersionFrozen is what writing to a published version answers. A published
// version is the one other people are looking at: editing it in place is how
// two readers of the same version number end up with different documents.
var ErrVersionFrozen = errors.New("trilha: this version is published, and a published version does not change")

// ErrNoVersion is an id, or a number, with no version behind it.
var ErrNoVersion = errors.New("trilha: there is no such version")

// NewVersioned builds the history. key names the kind of thing — it is the
// prefix of the rows in the store, and what an application reads in a query.
func NewVersioned[T any](key string, o VersionedOpts) *Versioned[T] {
	if key == "" {
		panic("trilha: NewVersioned needs a key")
	}
	v := &Versioned[T]{key: key, store: o.Store, now: time.Now}
	if v.store == nil {
		v.store = VersionMemory()
	}
	return v
}

// Draft opens the next version as a copy of the current one, and answers its
// number. It is what somebody presses before editing something that is
// published.
func (v *Versioned[T]) Draft(c *Ctx, id string) (int, error) {
	list, err := v.store.List(ctxOrBackground(c), v.key, id)
	if err != nil {
		return 0, err
	}
	n := len(list) + 1
	var data []byte
	if cur, ok := currentOf(list); ok {
		data = cur.Data
	} else {
		empty, err := json.Marshal(*new(T))
		if err != nil {
			return 0, err
		}
		data = empty
	}
	rec := VersionRecord{Version: Version{N: n, By: actorSubject(c), At: v.now()}, Data: data}
	if err := v.store.Save(ctxOrBackground(c), v.key, id, rec); err != nil {
		return 0, err
	}
	audit(c, v.key+".draft", id, Fields{"n": n})
	return n, nil
}

// Save writes the value into the open draft, and answers its number.
//
// With no version yet it writes version 1. With the last version published it
// refuses: the published one is what other people are reading, and a draft is
// one call away.
func (v *Versioned[T]) Save(c *Ctx, id string, value T, note string) (int, error) {
	list, err := v.store.List(ctxOrBackground(c), v.key, id)
	if err != nil {
		return 0, err
	}
	n := 1
	if len(list) > 0 {
		last := list[len(list)-1]
		if last.Published {
			return 0, NewHint(ErrFrozen, ErrVersionFrozen).
				Fix(fmt.Sprintf("open a draft first: Draft(c, %q) copies version %d and gives you the next one to edit", id, last.N)).
				Doc("/reference/app")
		}
		n = last.N
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	rec := VersionRecord{Version: Version{N: n, By: actorSubject(c), At: v.now(), Note: note}, Data: data}
	if err := v.store.Save(ctxOrBackground(c), v.key, id, rec); err != nil {
		return 0, err
	}
	audit(c, v.key+".save", id, Fields{"n": n})
	return n, nil
}

// Publish freezes a version and makes it the current one.
func (v *Versioned[T]) Publish(c *Ctx, id string, n int) error {
	rec, err := v.store.Get(ctxOrBackground(c), v.key, id, n)
	if err != nil {
		return err
	}
	if rec.Published {
		return nil // publishing what is published is not an error, it is a second click
	}
	list, err := v.store.List(ctxOrBackground(c), v.key, id)
	if err != nil {
		return err
	}
	// One published version at a time: "the current one" has to be a single
	// answer, or two screens read two different documents.
	for _, old := range list {
		if old.Published {
			old.Published = false
			if err := v.store.Save(ctxOrBackground(c), v.key, id, old); err != nil {
				return err
			}
		}
	}
	rec.Published = true
	if err := v.store.Save(ctxOrBackground(c), v.key, id, rec); err != nil {
		return err
	}
	audit(c, v.key+".publish", id, Fields{"n": n})
	return nil
}

// Current is the published version, or the last one when nothing was ever
// published — which is what a screen shows before the first publish.
func (v *Versioned[T]) Current(ctx context.Context, id string) (T, Version, error) {
	list, err := v.store.List(ctx, v.key, id)
	if err != nil {
		return *new(T), Version{}, err
	}
	rec, ok := currentOf(list)
	if !ok {
		return *new(T), Version{}, ErrNoVersion
	}
	return decode[T](rec)
}

// At is one version by number.
func (v *Versioned[T]) At(ctx context.Context, id string, n int) (T, Version, error) {
	rec, err := v.store.Get(ctx, v.key, id, n)
	if err != nil {
		return *new(T), Version{}, err
	}
	return decode[T](rec)
}

// History is every version, oldest first.
func (v *Versioned[T]) History(ctx context.Context, id string) ([]Version, error) {
	list, err := v.store.List(ctx, v.key, id)
	if err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(list))
	for _, r := range list {
		out = append(out, r.Version)
	}
	return out, nil
}

// Restore writes the content of version n as a new version, and answers its
// number.
//
// It does not delete anything and does not move the published mark: going back
// is a thing that happened, and a history that can lose an entry is a history
// nobody can answer questions with.
func (v *Versioned[T]) Restore(c *Ctx, id string, n int) (int, error) {
	old, err := v.store.Get(ctxOrBackground(c), v.key, id, n)
	if err != nil {
		return 0, err
	}
	list, err := v.store.List(ctxOrBackground(c), v.key, id)
	if err != nil {
		return 0, err
	}
	next := len(list) + 1
	if len(list) > 0 && !list[len(list)-1].Published {
		next = list[len(list)-1].N // an open draft is where a restore lands
	}
	rec := VersionRecord{
		Version: Version{N: next, By: actorSubject(c), At: v.now(),
			Note: fmt.Sprintf("restored from %d", n)},
		Data: old.Data,
	}
	if err := v.store.Save(ctxOrBackground(c), v.key, id, rec); err != nil {
		return 0, err
	}
	audit(c, v.key+".restore", id, Fields{"n": next, "from": n})
	return next, nil
}

func decode[T any](rec VersionRecord) (T, Version, error) {
	var out T
	if len(rec.Data) > 0 {
		if err := json.Unmarshal(rec.Data, &out); err != nil {
			return out, rec.Version, err
		}
	}
	return out, rec.Version, nil
}

// currentOf is the published version, or the last one.
func currentOf(list []VersionRecord) (VersionRecord, bool) {
	if len(list) == 0 {
		return VersionRecord{}, false
	}
	for _, r := range list {
		if r.Published {
			return r, true
		}
	}
	return list[len(list)-1], true
}

func ctxOrBackground(c *Ctx) context.Context {
	if c == nil {
		return context.Background()
	}
	return c.Context()
}

func actorSubject(c *Ctx) string {
	if c == nil {
		return ""
	}
	return c.Actor().Subject
}

func audit(c *Ctx, action, target string, f Fields) {
	if c != nil {
		c.Audit(action, target, f)
	}
}

// VersionMemory keeps the history in this process.
func VersionMemory() VersionStore { return &versionMemory{rows: map[string][]VersionRecord{}} }

type versionMemory struct {
	mu   sync.RWMutex
	rows map[string][]VersionRecord
}

func (m *versionMemory) Save(_ context.Context, key, id string, r VersionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key + "/" + id
	list := m.rows[k]
	for i, old := range list {
		if old.N == r.N {
			list[i] = r
			m.rows[k] = list
			return nil
		}
	}
	list = append(list, r)
	sort.Slice(list, func(i, j int) bool { return list[i].N < list[j].N })
	m.rows[k] = list
	return nil
}

func (m *versionMemory) List(_ context.Context, key, id string) ([]VersionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]VersionRecord{}, m.rows[key+"/"+id]...), nil
}

func (m *versionMemory) Get(_ context.Context, key, id string, n int) (VersionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.rows[key+"/"+id] {
		if r.N == n {
			return r, nil
		}
	}
	return VersionRecord{}, ErrNoVersion
}
