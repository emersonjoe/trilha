package trilha

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// OfflineKeyField is the hidden field carrying the idempotency key of a form
// marked with OfflineForm. The browser sends it back with the queued form, and
// Idempotent reads it when the request has no Idempotency-Key header.
const OfflineKeyField = "_idempotency_key"

// OfflineQueuedAtField is the hidden field the outbox script fills with the
// moment the person pressed the button, in RFC 3339. It is the client's stamp
// and nothing more: the server never trusts it for ordering, only records it
// so a last-write-wins resolution can be explained afterwards.
const OfflineQueuedAtField = "_queued_at"

// defaultIdempotencyTTL is how long a key is remembered when the handler asks
// for no window of its own.
const defaultIdempotencyTTL = 24 * time.Hour

// maxIdempotencyKeys bounds the in-process store: an outbox that replays for a
// week must not become a leak. The oldest keys go first, and a key that was
// dropped answers "not seen", which is the safe direction — the handler runs
// again instead of confirming something that never happened.
const maxIdempotencyKeys = 10000

// OfflineForm marks a form as one the browser may queue when the network is
// gone, and gives that queued copy the two things the server needs on the
// other side: the key that makes the replay harmless and the slot for the
// moment it was filled in.
//
//	h.Form(h.Method("post"), h.Action("/coleta"),
//	    trilha.CSRFInput(c),
//	    trilha.OfflineForm(c),
//	    ui.Field("nota", "Nota", ui.Input(h.Name("nota"))),
//	    ui.Submit(h.Text("Send")),
//	)
//
// With ui.OfflineScript on the page, a submit with no network goes into the
// browser's outbox and is sent in order when the network comes back. Without
// JavaScript nothing changes: the form posts as usual, and the handler still
// gets the key.
//
// A form that carries a file is never queued — the bytes do not survive the
// wait — and the kit says so on the screen instead of pretending.
func OfflineForm(c *Ctx) h.Node {
	return h.Group(
		// The marker rides on the hidden field, and not on the <form>: this is
		// a fragment placed among the form's children, and an attribute inside
		// a fragment is content, not an attribute of the element around it. A
		// form may also carry h.Data("trilha-offline", "") itself, and the
		// script takes either.
		h.Input(h.Type("hidden"), h.Data("trilha-offline", ""),
			h.Name(OfflineKeyField), h.Value(NewIdempotencyKey())),
		h.Input(h.Type("hidden"), h.Name(OfflineQueuedAtField), h.Value("")),
	)
}

// NewIdempotencyKey mints the key of one submission: 16 random bytes in hex.
// It is minted on the server, so a form that was drawn once cannot be replayed
// as two different submissions by a client that renames its own key.
func NewIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("trilha: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// IdempotencyStore remembers the keys already handled, so the same submission
// arriving twice is done once. One method, and the hard part is that it has to
// be atomic: two replays racing each other is exactly the case this exists
// for.
//
// Nil keeps the keys in the process, which is honest about one replica and
// said once in the log. A SQL implementation is an INSERT with a unique
// constraint (a duplicate key error means seen); Redis is SET NX with the TTL.
type IdempotencyStore interface {
	// Seen records the key and reports whether it was already there. The
	// recording and the answer are one operation, and ttl is how long the key
	// stays remembered.
	Seen(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Idempotent reports whether this request is the replay of one already
// handled. It reads the key from the Idempotency-Key header or, failing that,
// from the hidden field OfflineForm wrote, records it, and answers.
//
//	replay, err := trilha.Idempotent(c, time.Hour)
//	if err != nil {
//	    return err
//	}
//	if replay {
//	    return c.Redirect("/coleta") // the same answer as the first time
//	}
//
// A request with no key at all is not a replay and nothing is recorded: a form
// that never asked for this cannot be made to fail by sending it twice.
//
// A replay is written to the trail with c.Audit, with the key masked and the
// client's stamp beside it, because "this arrived twice" is the fact somebody
// reading the trail needs when two devices filled in the same screen.
func Idempotent(c *Ctx, ttl time.Duration) (bool, error) {
	key := IdempotencyKey(c)
	if key == "" {
		return false, nil
	}
	if ttl <= 0 {
		ttl = defaultIdempotencyTTL
	}
	seen, err := c.app.idempotency().Seen(c.Context(), key, ttl)
	if err != nil || !seen {
		return false, err
	}
	fields := Fields{}
	if at, ok := QueuedAt(c); ok {
		fields["queued_at"] = at.UTC().Format(time.RFC3339)
	}
	c.Audit("offline.replay", maskSecret(key), fields)
	return true, nil
}

// IdempotencyKey is the key of this request: the Idempotency-Key header, or
// the hidden field of a form marked with OfflineForm. Empty when there is
// none.
func IdempotencyKey(c *Ctx) string {
	if v := c.Request().Header.Get("Idempotency-Key"); v != "" {
		return v
	}
	return c.Form(OfflineKeyField)
}

// QueuedAt is when the browser says the person pressed the button, from the
// field the outbox script fills before it stores the submission. It is what a
// handler resolving a conflict writes down beside the value it kept:
// last-write-wins with the stamp recorded is a decision somebody can read
// later, and a CRDT is not what a form in a field notebook needs.
//
// It is a client clock, so it is evidence and never authority: a handler that
// lets it decide ordering has handed the ordering to the device.
func QueuedAt(c *Ctx) (time.Time, bool) {
	v := c.Form(OfflineQueuedAtField)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// idempotency is the store the app answers with; nil in Config keeps the keys
// in the process, which is enough for one replica and said once.
func (a *App) idempotency() IdempotencyStore {
	a.idemOnce.Do(func() {
		if a.cfg.Idempotency == nil {
			a.idem = &memIdempotency{}
			a.infoOnce("idempotency:memory", "trilha: idempotency keys kept in memory; a second replica would replay a submission it never saw")
			return
		}
		a.idem = a.cfg.Idempotency
	})
	return a.idem
}

// memIdempotency remembers keys in the process, with an expiry and a ceiling.
type memIdempotency struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func (s *memIdempotency) Seen(_ context.Context, key string, ttl time.Duration) (bool, error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]time.Time{}
	}
	if until, ok := s.m[key]; ok && until.After(now) {
		return true, nil
	}
	s.evict(now)
	s.m[key] = now.Add(ttl)
	return false, nil
}

// evict drops what expired and, if the map is still at the ceiling, the keys
// that expire soonest. Forgetting a key costs a handler that runs twice;
// growing without a bound costs the process.
func (s *memIdempotency) evict(now time.Time) {
	for k, until := range s.m {
		if !until.After(now) {
			delete(s.m, k)
		}
	}
	if len(s.m) < maxIdempotencyKeys {
		return
	}
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return s.m[keys[i]].Before(s.m[keys[j]]) })
	for _, k := range keys[:len(keys)-maxIdempotencyKeys/2] {
		delete(s.m, k)
	}
}
