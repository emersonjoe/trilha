// Package webhook tells another application that something happened, and does
// the four things a handler with an http.Post in it does not.
//
// It does not make the visitor wait for somebody else's server — which is on
// another network, at another company, and sometimes from another decade. It
// signs, so the receiver can tell your call from whoever found the URL. It
// tries again, because the partner restarted at three in the morning and that
// event would otherwise simply never have existed. And it writes down every
// attempt, so "did you send it?" is a screen and not a grep.
//
// There is a fifth thing, and it is the ugliest. The URL belongs to the
// partner, but the person typing it is one of yours: a webhook pointed at
// http://169.254.169.254/ is your own server fetching the machine's cloud
// credentials and posting them to whoever registered the address. Every URL
// here is checked when it is registered and again when it is delivered to.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

var (
	// ErrUnknownEvent is a name outside Options.Events. The list is closed on
	// purpose: a typo in an Emit would otherwise be an event nobody is
	// subscribed to, which looks exactly like a partner who is not listening.
	ErrUnknownEvent = errors.New("webhook: unknown event")
	// ErrNotFound is an id the store does not have.
	ErrNotFound = errors.New("webhook: not found")
)

// State is where a delivery is.
type State string

const (
	// Pending is waiting for its turn, or for the next attempt.
	Pending State = "pending"
	// Delivered got a 2xx.
	Delivered State = "delivered"
	// Failed ran out of attempts. The status and the beginning of the body
	// are on the record, because those are what say why.
	Failed State = "failed"
)

// Subscription is one partner listening to some events.
type Subscription struct {
	ID string
	// Tenant is the organisation this belongs to, from the session. An empty
	// one is an application with no tenants, which is most of them.
	Tenant string
	// URL is where the POST goes. It is checked before it is stored.
	URL string
	// Events is what this subscription wants. Empty means every event of
	// Options.Events, which is a choice somebody makes on purpose.
	Events []string
	// Secret signs the deliveries. It is a trilha.Secret, so the column is
	// encrypted and the log is masked; the plain value is shown once, by
	// Subscribe, and never again.
	Secret trilha.Secret
	// Label is what a person calls it on the screen.
	Label   string
	Created time.Time
	Revoked bool
}

// Wants answers whether this subscription is listening to an event.
func (s Subscription) Wants(event string) bool {
	if s.Revoked {
		return false
	}
	if len(s.Events) == 0 {
		return true
	}
	for _, e := range s.Events {
		if e == event {
			return true
		}
	}
	return false
}

// Delivery is one attempt to tell one partner about one event — or rather all
// the attempts, because they share a record: what matters afterwards is
// whether it arrived, and what the other side said when it did not.
type Delivery struct {
	ID             string
	SubscriptionID string
	Tenant         string
	Event          string
	// Payload is the body as it was signed. It is kept so a retry sends the
	// same bytes: re-encoding the object would produce a different signature
	// for the same event, and a partner checking for duplicates would see two.
	Payload []byte
	State   State
	// Attempt counts what has been tried. NextTry is when the next one is due.
	Attempt int
	NextTry time.Time
	// Status is the last HTTP status, 0 when the request never got an answer.
	// Err is the transport error; Response the first kilobyte of the body.
	Status   int
	Err      string
	Response string
	// RetryOf is the delivery this one was created from, by the button.
	RetryOf string
	Created time.Time
	Ended   time.Time
}

// Options configures the sender.
type Options struct {
	// Store is where subscriptions and deliveries live. nil is Memory().
	Store Store
	// Events is the closed list of what this application emits. It is what
	// the screen offers and what Emit refuses to go outside of.
	Events []string
	// Env decides whether http:// is allowed: only in Dev. Pass a.Env().
	// The zero value is Prod, which is the safe way round for a field
	// somebody forgets.
	Env trilha.Env
	// Workers is how many deliveries go at once (default 2).
	Workers int
	// Timeout bounds one attempt (default 10 s).
	Timeout time.Duration
	// Backoff is the wait before each attempt after the first. Its length is
	// how many attempts there are; the default is 1 min, 5, 30, 2 h, 12 h —
	// about fifteen hours of patience, which covers a partner's night.
	Backoff []time.Duration
	// Tick is how often the clock looks for deliveries that are due
	// (default 15 s).
	Tick time.Duration
	// HTTP is the client. The default has no redirect following: a webhook
	// that follows a 302 is a webhook whose destination was never the one
	// that got checked.
	HTTP   *http.Client
	Logger *slog.Logger
	// AllowPrivateURL turns off the address check. It exists for the test
	// suite of this package and for an application whose partners really are
	// on the same private network — and it is spelled out because turning it
	// on is turning off the defence against a webhook pointed at your own
	// metadata service.
	AllowPrivateURL bool
}

// Hooks is the sender. One per application, built where the app's other
// values are: New starts nothing, Setup does.
type Hooks struct {
	store   Store
	events  []string
	env     trilha.Env
	workers int
	timeout time.Duration
	backoff []time.Duration
	tick    time.Duration
	client  *http.Client
	log     *slog.Logger
	allow   bool

	queue chan string
	mu    sync.Mutex
	// inflight is the deliveries a worker is holding. Emit enqueues, and so
	// does the clock when the row comes due — the same id can be in the queue
	// twice, and without this two workers would POST it at the same moment
	// and the partner would get one event twice.
	inflight map[string]bool

	started bool
	stop    chan struct{}
	done    sync.WaitGroup

	now func() time.Time
	// resolve replaces the DNS lookup, for the tests of the address check.
	resolve func(string) ([]net.IP, error)
}

// New builds the sender. It opens no connection and reads no store.
func New(o Options) *Hooks {
	h := &Hooks{
		store: o.Store, events: append([]string{}, o.Events...), env: o.Env,
		workers: o.Workers, timeout: o.Timeout, backoff: o.Backoff, tick: o.Tick,
		client: o.HTTP, log: o.Logger, allow: o.AllowPrivateURL,
		queue: make(chan string, 256), stop: make(chan struct{}), now: time.Now,
		inflight: map[string]bool{},
	}
	if h.store == nil {
		h.store = Memory()
	}
	if h.workers <= 0 {
		h.workers = 2
	}
	if h.timeout <= 0 {
		h.timeout = 10 * time.Second
	}
	if len(h.backoff) == 0 {
		h.backoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute,
			2 * time.Hour, 12 * time.Hour}
	}
	if h.tick <= 0 {
		h.tick = 15 * time.Second
	}
	if h.client == nil {
		h.client = &http.Client{
			// No redirects. A 302 from the partner's host to somewhere else is
			// a destination nobody checked, which is the whole SSRF story
			// wearing a different hat.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if h.log == nil {
		h.log = slog.Default()
	}
	return h
}

// Events is the closed list this application emits, for a screen to offer.
func (h *Hooks) Events() []string { return append([]string{}, h.events...) }

// Setup starts the workers and the clock, and hangs Shutdown on the app.
//
//	func Setup(a *trilha.App) error {
//		Hooks = webhook.New(webhook.Options{Events: eventos, Env: a.Env()})
//		trilha.Provide(a, Hooks)
//		return Hooks.Setup(a)
//	}
func (h *Hooks) Setup(a *trilha.App) error {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return nil
	}
	h.started = true
	h.stop = make(chan struct{})
	stop := h.stop
	h.mu.Unlock()

	for i := 0; i < h.workers; i++ {
		h.done.Add(1)
		go h.work(stop)
	}
	h.done.Add(1)
	go h.clock(stop)

	if a != nil {
		a.OnShutdown(func(*trilha.App) error { return h.Shutdown(context.Background()) })
	}
	return nil
}

// Shutdown stops the clock and waits for the attempts in flight. What is
// pending stays pending: it is a row with a due time, and the next process
// picks it up — which is the difference between a delivery and a goroutine.
func (h *Hooks) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	if !h.started {
		h.mu.Unlock()
		return nil
	}
	h.started = false
	close(h.stop)
	h.mu.Unlock()

	esperou := make(chan struct{})
	go func() { h.done.Wait(); close(esperou) }()
	select {
	case <-esperou:
	case <-ctx.Done():
	case <-time.After(h.timeout + 5*time.Second):
	}
	return nil
}

// Emit records one delivery per subscription listening to the event, and
// returns. It does not wait for the network: that is the point of it.
//
//	if err := app.Hooks.Emit(c, "documento.processado", doc); err != nil {
//		c.Log().Error("webhook", "err", err)
//	}
//
// The error is about this application — an unknown event, a store that would
// not write — and never about the partner. Whether the partner answered is a
// delivery record, and a handler has no business waiting to find out.
func (h *Hooks) Emit(c *trilha.Ctx, event string, payload any) error {
	if !h.known(event) {
		return fmt.Errorf("%w: %q is not in Options.Events", ErrUnknownEvent, event)
	}
	ctx := context.Background()
	tenant := ""
	if c != nil {
		ctx, tenant = c.Context(), c.Actor().Tenant
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook: encoding %s: %w", event, err)
	}
	subs, err := h.store.Subscriptions(ctx, tenant)
	if err != nil {
		return err
	}
	n := 0
	for _, s := range subs {
		if !s.Wants(event) {
			continue
		}
		d := Delivery{
			ID: newID("dlv"), SubscriptionID: s.ID, Tenant: s.Tenant, Event: event,
			Payload: body, State: Pending, Created: h.now(), NextTry: h.now(),
		}
		if err := h.store.SaveDelivery(ctx, d); err != nil {
			return err
		}
		h.enqueue(d.ID)
		n++
	}
	if c != nil && n > 0 {
		c.Audit("webhook.emitiu", event, trilha.Fields{"entregas": n})
	}
	return nil
}

// enqueue hands a delivery to a worker, and shrugs when the queue is full:
// the row carries its own due time, so the clock finds it on the next pass.
// Blocking here would be Emit waiting on the network by another route.
func (h *Hooks) enqueue(id string) {
	select {
	case h.queue <- id:
	default:
	}
}

func (h *Hooks) known(event string) bool {
	if len(h.events) == 0 {
		return true
	}
	for _, e := range h.events {
		if e == event {
			return true
		}
	}
	return false
}
