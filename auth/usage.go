package auth

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// The first question anybody asks after handing a key to a partner is not about
// security: it is "are they using it? where? when did they stop?". The key
// already knows who called (Keys.User) and when it was last seen
// (Key.LastUsed). What was missing is the shape of the use — which routes, how
// often, how many of them failed.

// UsageRow is one bucket of use: one key, one method, one route, one day.
//
// The route is the pattern and not the path — /documents/{id}, never
// /documents/8f2c… — because a counter per concrete path is a counter with one
// row per request.
type UsageRow struct {
	KeyID  string
	Method string
	Route  string
	// Day is the day the calls happened on, at midnight UTC.
	Day time.Time
	// Count is how many calls, Errors how many of them ended in one.
	Count  int
	Errors int
	// Last and LastStatus are the most recent call in this bucket.
	Last       time.Time
	LastStatus int
}

// UsageQuery narrows a report. A zero query is everything the store holds.
type UsageQuery struct {
	// Key limits the report to one key. Empty is every key.
	Key string
	// Since and Until bound the days, inclusive. Zero means unbounded.
	Since time.Time
	Until time.Time
}

// UsageRoute is what one route received.
type UsageRoute struct {
	Method string
	Route  string
	Count  int
	Errors int
	Last   time.Time
}

// UsageDay is what one day received.
type UsageDay struct {
	Day    time.Time
	Count  int
	Errors int
}

// UsageKeyTotal is what one key did, for a report across keys.
type UsageKeyTotal struct {
	KeyID  string
	Count  int
	Errors int
	Last   time.Time
}

// UsageReport is the answer: the total, the shape by route, the shape by day,
// and — when the query was not about one key — the total per key.
type UsageReport struct {
	Total   int
	Errors  int
	Last    time.Time
	ByRoute []UsageRoute
	ByDay   []UsageDay
	ByKey   []UsageKeyTotal
}

// UsageStore is where the counters live. Memory is the default; a table behind
// the same three methods is the next step, and no screen changes.
//
// Add is given buckets that are already aggregated, so a store writes one row
// per (key, method, route, day) and not one per request — which is the whole
// reason counting is affordable.
type UsageStore interface {
	Add(ctx context.Context, rows []UsageRow) error
	Query(ctx context.Context, q UsageQuery) (UsageReport, error)
	// Prune deletes buckets whose day is before that date. Retention is a
	// decision, and a counter table nobody ever deletes from is a table that
	// outgrows what it is worth.
	Prune(ctx context.Context, before time.Time) error
}

// ErrNoUsage is a report asked of an application that does not count.
var ErrNoUsage = errors.New("auth: this application has no KeyOptions.Usage")

// defaultUsageFlush is how often the buffer goes to the store.
const defaultUsageFlush = 30 * time.Second

// usageBucket is the key of the in-memory aggregate.
type usageBucket struct {
	key    string
	method string
	route  string
	day    int64
}

// usageCell is what a bucket accumulates between two flushes.
type usageCell struct {
	count      int
	errors     int
	last       time.Time
	lastStatus int
}

// record adds one call to the buffer. It is the only thing a request pays for:
// a map write under a mutex, and nothing that can block on a network.
//
// The status is 200 for a request that returned no error and the code the error
// carried otherwise. A handler that writes 201 itself counts as a success,
// which is what the question this answers — is this key working? — needs.
func (ks *Keys) record(c *trilha.Ctx, k *Key, err error) {
	if ks.opts.Usage == nil {
		return
	}
	route := c.Pattern()
	if route == "" {
		// No pattern means the fallback answered, and the concrete path is
		// user input: counting it would be a row per request, made by whoever
		// asks for enough addresses that do not exist.
		return
	}
	status := http.StatusOK
	if err != nil {
		status = trilha.StatusOf(err)
	}
	now := ks.now()
	b := usageBucket{key: k.ID, method: c.Request().Method, route: route,
		day: now.UTC().Truncate(24 * time.Hour).Unix()}

	ks.usageMu.Lock()
	defer ks.usageMu.Unlock()
	if ks.usage == nil {
		ks.usage = map[usageBucket]*usageCell{}
	}
	cell := ks.usage[b]
	if cell == nil {
		cell = &usageCell{}
		ks.usage[b] = cell
	}
	cell.count++
	if status >= 400 {
		cell.errors++
	}
	cell.last, cell.lastStatus = now, status
}

// Flush writes what is buffered into the store.
//
// It is called on a timer and on shutdown by Setup, and it is exported because
// a test that asserts a count should not have to wait for a clock.
func (ks *Keys) Flush(ctx context.Context) error {
	if ks.opts.Usage == nil {
		return nil
	}
	ks.usageMu.Lock()
	buf := ks.usage
	ks.usage = nil
	ks.usageMu.Unlock()
	if len(buf) == 0 {
		return nil
	}
	rows := make([]UsageRow, 0, len(buf))
	for b, cell := range buf {
		rows = append(rows, UsageRow{
			KeyID: b.key, Method: b.method, Route: b.route,
			Day:   time.Unix(b.day, 0).UTC(),
			Count: cell.count, Errors: cell.errors,
			Last: cell.last, LastStatus: cell.lastStatus,
		})
	}
	// Sorted so a store that writes in order writes the same order twice.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].KeyID != rows[j].KeyID {
			return rows[i].KeyID < rows[j].KeyID
		}
		if rows[i].Route != rows[j].Route {
			return rows[i].Route < rows[j].Route
		}
		return rows[i].Method < rows[j].Method
	})
	if err := ks.opts.Usage.Add(ctx, rows); err != nil {
		// The counts go back into the buffer: losing them because the database
		// was restarting is the failure mode this whole design is avoiding.
		ks.usageMu.Lock()
		if ks.usage == nil {
			ks.usage = buf
		} else {
			for b, cell := range buf {
				if old := ks.usage[b]; old != nil {
					old.count += cell.count
					old.errors += cell.errors
					continue
				}
				ks.usage[b] = cell
			}
		}
		ks.usageMu.Unlock()
		return err
	}
	return nil
}

// Usage answers the report.
//
// It reads the store and does not flush first, on purpose: a screen shows what
// was written, and a read that wrote would make two people opening the same
// page race each other.
func (ks *Keys) Usage(ctx context.Context, q UsageQuery) (UsageReport, error) {
	if ks.opts.Usage == nil {
		return UsageReport{}, ErrNoUsage
	}
	return ks.opts.Usage.Query(ctx, q)
}

// Idle is the keys with no recorded call since that moment.
//
// It is the question the issue asked of `trilha audit` — "keys unused for 90
// days" — answered where the data is. The command reads code on somebody's
// laptop; the counters live in production, and a check that cannot see them
// would be a check that always says everything is fine.
func (ks *Keys) Idle(ctx context.Context, since time.Time) ([]*Key, error) {
	if ks.opts.Usage == nil {
		return nil, ErrNoUsage
	}
	rel, err := ks.opts.Usage.Query(ctx, UsageQuery{Since: since})
	if err != nil {
		return nil, err
	}
	used := make(map[string]bool, len(rel.ByKey))
	for _, k := range rel.ByKey {
		used[k.KeyID] = true
	}
	all, err := ks.store.All()
	if err != nil {
		return nil, err
	}
	var out []*Key
	for _, k := range all {
		if !used[k.ID] {
			out = append(out, k)
		}
	}
	return out, nil
}

// Setup wires the counters to the application: a flush on a timer, a flush when
// the server stops, and the retention sweep.
//
//	func main() { … if err := Chaves.Setup(a); err != nil { … } … }
//
// Without it the counting still happens and Flush is yours to call. With it,
// what was counted survives a deploy — a counter that only exists in the
// process is a counter that disappears every release.
func (ks *Keys) Setup(a *trilha.App) error {
	if ks.opts.Usage == nil {
		return nil
	}
	every := ks.opts.UsageFlush
	if every <= 0 {
		every = defaultUsageFlush
	}
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if err := ks.Flush(context.Background()); err != nil {
					a.Logger().Warn("auth: api key usage not written", "error", err)
				}
				if keep := ks.opts.UsageKeep; keep > 0 {
					if err := ks.opts.Usage.Prune(context.Background(), ks.now().Add(-keep)); err != nil {
						a.Logger().Warn("auth: api key usage not pruned", "error", err)
					}
				}
			}
		}
	}()
	a.OnShutdown(func(*trilha.App) error {
		close(stop)
		return ks.Flush(context.Background())
	})
	return nil
}

// UsageMemory keeps the counters in this process.
//
// It is the default so an application counts before it has a table, and it is
// honest about what it is: the numbers last as long as the process, which is
// enough to see the shape and not enough to answer for last quarter.
func UsageMemory() UsageStore { return &usageMemory{rows: map[usageBucket]*UsageRow{}} }

type usageMemory struct {
	mu   sync.RWMutex
	rows map[usageBucket]*UsageRow
}

func (m *usageMemory) Add(_ context.Context, rows []UsageRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range rows {
		b := usageBucket{key: r.KeyID, method: r.Method, route: r.Route, day: r.Day.UTC().Unix()}
		old := m.rows[b]
		if old == nil {
			row := r
			m.rows[b] = &row
			continue
		}
		old.Count += r.Count
		old.Errors += r.Errors
		if r.Last.After(old.Last) {
			old.Last, old.LastStatus = r.Last, r.LastStatus
		}
	}
	return nil
}

func (m *usageMemory) Prune(_ context.Context, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for b, r := range m.rows {
		if r.Day.Before(before) {
			delete(m.rows, b)
		}
	}
	return nil
}

func (m *usageMemory) Query(_ context.Context, q UsageQuery) (UsageReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var rel UsageReport
	byRoute := map[[2]string]*UsageRoute{}
	byDay := map[int64]*UsageDay{}
	byKey := map[string]*UsageKeyTotal{}
	for _, r := range m.rows {
		if q.Key != "" && r.KeyID != q.Key {
			continue
		}
		if !q.Since.IsZero() && r.Day.Before(q.Since.UTC().Truncate(24*time.Hour)) {
			continue
		}
		if !q.Until.IsZero() && r.Day.After(q.Until) {
			continue
		}
		rel.Total += r.Count
		rel.Errors += r.Errors
		if r.Last.After(rel.Last) {
			rel.Last = r.Last
		}

		rk := [2]string{r.Method, r.Route}
		if byRoute[rk] == nil {
			byRoute[rk] = &UsageRoute{Method: r.Method, Route: r.Route}
		}
		acc := byRoute[rk]
		acc.Count += r.Count
		acc.Errors += r.Errors
		if r.Last.After(acc.Last) {
			acc.Last = r.Last
		}

		d := r.Day.Unix()
		if byDay[d] == nil {
			byDay[d] = &UsageDay{Day: r.Day}
		}
		byDay[d].Count += r.Count
		byDay[d].Errors += r.Errors

		if byKey[r.KeyID] == nil {
			byKey[r.KeyID] = &UsageKeyTotal{KeyID: r.KeyID}
		}
		kt := byKey[r.KeyID]
		kt.Count += r.Count
		kt.Errors += r.Errors
		if r.Last.After(kt.Last) {
			kt.Last = r.Last
		}
	}
	for _, v := range byRoute {
		rel.ByRoute = append(rel.ByRoute, *v)
	}
	for _, v := range byDay {
		rel.ByDay = append(rel.ByDay, *v)
	}
	for _, v := range byKey {
		rel.ByKey = append(rel.ByKey, *v)
	}
	// Busiest route first — it is the one somebody is looking for — and days
	// in the order a chart draws them.
	sort.Slice(rel.ByRoute, func(i, j int) bool {
		if rel.ByRoute[i].Count != rel.ByRoute[j].Count {
			return rel.ByRoute[i].Count > rel.ByRoute[j].Count
		}
		return rel.ByRoute[i].Route < rel.ByRoute[j].Route
	})
	sort.Slice(rel.ByDay, func(i, j int) bool { return rel.ByDay[i].Day.Before(rel.ByDay[j].Day) })
	sort.Slice(rel.ByKey, func(i, j int) bool { return rel.ByKey[i].Count > rel.ByKey[j].Count })
	return rel, nil
}
