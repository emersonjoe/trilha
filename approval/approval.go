// Package approval is the other half of the work an application does: the part
// that waits for a person.
//
// The task package runs what a machine can finish on its own. This one is the
// queue every business application grows anyway — things somebody has to
// approve, reject or let expire — with the four things that queue always needs
// and that nobody writes the first time: an owner, a deadline, the reason
// written down, and who decided.
//
// What it is not is a workflow engine. One step at a time; chaining is an On
// handler opening the next request, which is a line the application writes and
// can read.
package approval

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// The states of a request. They are a registered trilha.Enum, so ui.Status
// colours them and the enum= tag validates them without the application
// declaring the list a second time.
const (
	Pending   = "pending"
	Approved  = "approved"
	Rejected  = "rejected"
	Withdrawn = "withdrawn"
	Expired   = "expired"
)

// States is the list, with the tone each one wears.
var States = trilha.Enum{
	{Value: Pending, Label: "Pending", Tone: "info"},
	{Value: Approved, Label: "Approved", Tone: "success"},
	{Value: Rejected, Label: "Rejected", Tone: "danger"},
	{Value: Withdrawn, Label: "Withdrawn", Tone: "muted"},
	{Value: Expired, Label: "Expired", Tone: "warning"},
}

func init() { trilha.RegisterEnum("approval.State", States) }

// ErrUnknown is an id nothing answers to.
var ErrUnknown = errors.New("approval: no request with that id")

// ErrNotYours is what somebody who may not decide a request gets. It is one
// error for "not assigned to you" and for "already decided": which of the two
// it is is not the visitor's business.
var ErrNotYours = errors.New("approval: this request is not yours to decide")

// Assignee is who may decide: a role, or one person. It is a struct and not
// two fields on Request so the choice is visible at the call site —
// approval.Role("cpad") reads as what it is.
type Assignee struct {
	Role string
	User string
}

// Role assigns a request to whoever holds a role.
func Role(name string) Assignee { return Assignee{Role: name} }

// User assigns a request to one person, by the subject of their session.
func User(subject string) Assignee { return Assignee{User: subject} }

// Request is what an application opens.
type Request struct {
	// Kind groups the queue: "eliminacao", "reembolso". It is what the inbox
	// filters by and what On is registered for.
	Kind string
	// Subject is the line somebody reads.
	Subject string
	// Target is where the thing being decided lives, so the inbox can link to
	// the context instead of describing it.
	Target string
	// Assign is who may decide.
	Assign Assignee
	// Due is when it stops being worth deciding. Zero is no deadline, and a
	// deadline is what makes a queue a queue instead of a list that grows.
	Due time.Time
	// Data is what the handler needs and the screen may show. Strings, because
	// it crosses a store: an id belongs here, an object does not.
	Data map[string]string
}

// Record is one request, as the store keeps it.
type Record struct {
	ID      string
	Kind    string
	Subject string
	Target  string
	State   string
	Assign  Assignee
	Data    map[string]string
	Opened  time.Time
	Due     time.Time
	// Decided, By and Reason are the decision: when, who, and why. The reason
	// is the field an application always adds later, after the first argument
	// about a decision nobody can explain.
	Decided time.Time
	By      string
	Reason  string
	// OpenedBy is who asked, which is not always who decides.
	OpenedBy string
}

// Late reports whether a pending request is past its deadline.
func (r Record) Late(now time.Time) bool {
	return r.State == Pending && !r.Due.IsZero() && now.After(r.Due)
}

// Store is where the queue lives. Memory is the default, and a table behind
// the same three methods is the next step — no screen changes.
type Store interface {
	Save(ctx context.Context, r Record) error
	Get(ctx context.Context, id string) (Record, error)
	List(ctx context.Context, p ListParams) ([]Record, error)
}

// ListParams is what a listing asks for.
type ListParams struct {
	Kind  string
	State string
	// Subject and Roles narrow the list to what one person may decide. Empty
	// ones list everything, which is what an administration screen shows.
	Subject string
	Roles   []string
	Limit   int
}

// Options configures the queue.
type Options struct {
	Store Store
	// Tick is how often deadlines are checked (default 30 s). The deadline
	// expires in this process, for the same reason the task package sweeps its
	// own: an application that needs a cron to be correct is an application
	// that is wrong on the day the cron does not run.
	Tick time.Duration
	// Roles answers which roles the current session holds, and it is how a
	// request assigned to a role finds its people.
	//
	// It is a function the application supplies — one line, sessao.Atual(c)
	// .Roles or the OIDC claim — because this package does not know how you
	// authenticate, and a package that guessed would be a check that looks
	// like a guarantee and is not one. Without it, only requests assigned to a
	// person can be decided.
	Roles  func(c *trilha.Ctx) []string
	Logger *slog.Logger
}

// Approvals is the queue.
type Approvals struct {
	store Store
	tick  time.Duration
	roles func(*trilha.Ctx) []string
	log   *slog.Logger
	now   func() time.Time

	mu     sync.RWMutex
	on     map[string]func(*trilha.Ctx, Record) error
	stop   chan struct{}
	closed bool
}

// New builds the queue. It starts nothing: Setup does.
func New(o Options) *Approvals {
	a := &Approvals{store: o.Store, tick: o.Tick, roles: o.Roles, log: o.Logger, now: time.Now,
		on: map[string]func(*trilha.Ctx, Record) error{}, stop: make(chan struct{})}
	if a.store == nil {
		a.store = Memory()
	}
	if a.tick <= 0 {
		a.tick = 30 * time.Second
	}
	if a.log == nil {
		a.log = slog.Default()
	}
	return a
}

// On registers what happens when a request of that kind is decided.
//
// It runs after the decision is written, and its error does not undo it: the
// decision is the record of what a person chose, and a mail server being down
// is not a reason to pretend they did not choose. The error is logged, and
// making the work retryable is the handler's job — a task, a webhook.
func (a *Approvals) On(kind string, fn func(c *trilha.Ctx, r Record) error) {
	if kind == "" || fn == nil {
		panic("approval: On needs a kind and a function")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, taken := a.on[kind]; taken {
		panic("approval: two handlers registered for " + kind)
	}
	a.on[kind] = fn
}

// Setup starts the deadline clock and hangs the shutdown on the app.
func (a *Approvals) Setup(app *trilha.App) error {
	go a.clock()
	if app != nil {
		app.OnShutdown(func(*trilha.App) error { a.Shutdown(); return nil })
	}
	return nil
}

// Shutdown stops the clock.
func (a *Approvals) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.closed {
		a.closed = true
		close(a.stop)
	}
}

// Open records a request and answers its id.
func (a *Approvals) Open(c *trilha.Ctx, r Request) (string, error) {
	if r.Kind == "" || r.Subject == "" {
		return "", errors.New("approval: a request needs a Kind and a Subject")
	}
	if r.Assign.Role == "" && r.Assign.User == "" {
		return "", errors.New("approval: a request needs somebody to decide it — approval.Role or approval.User")
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	rec := Record{
		ID: id, Kind: r.Kind, Subject: r.Subject, Target: r.Target, State: Pending,
		Assign: r.Assign, Data: r.Data, Opened: a.now(), Due: r.Due, OpenedBy: actorOf(c),
	}
	if err := a.store.Save(ctxOf(c), rec); err != nil {
		return "", err
	}
	if c != nil {
		c.Audit("approval.open", id, trilha.Fields{"kind": r.Kind, "subject": r.Subject})
	}
	return id, nil
}

// Decide writes the decision, and then runs what the application registered
// for that kind.
//
// Who may decide is checked here and not on the screen: a screen that hides a
// button is a screen, and the address behind it is still an address.
func (a *Approvals) Decide(c *trilha.Ctx, id, state, reason string) error {
	switch state {
	case Approved, Rejected, Withdrawn:
	default:
		return fmt.Errorf("approval: %q is not a decision", state)
	}
	rec, err := a.store.Get(ctxOf(c), id)
	if err != nil {
		return err
	}
	if rec.State != Pending || !a.MayDecide(c, rec) {
		return ErrNotYours
	}
	rec.State, rec.Reason, rec.Decided, rec.By = state, reason, a.now(), actorOf(c)
	if err := a.store.Save(ctxOf(c), rec); err != nil {
		return err
	}
	if c != nil {
		c.Audit("approval.decide", id, trilha.Fields{"kind": rec.Kind, "state": state, "reason": reason})
	}
	a.mu.RLock()
	fn := a.on[rec.Kind]
	a.mu.RUnlock()
	if fn == nil {
		return nil
	}
	if err := fn(c, rec); err != nil {
		// The decision stands: a person chose, and it is written down. What
		// failed is what the application does about it, and that is the
		// application's to retry.
		a.log.Error("approval: the handler of a decision failed",
			"kind", rec.Kind, "id", id, "err", err)
	}
	return nil
}

// MayDecide answers whether the current session may decide that request: the
// person it was assigned to, or somebody holding the role it was assigned to.
func (a *Approvals) MayDecide(c *trilha.Ctx, r Record) bool {
	if c == nil {
		return false
	}
	if r.Assign.User != "" {
		return r.Assign.User == c.Actor().Subject
	}
	for _, role := range a.rolesOf(c) {
		if role == r.Assign.Role {
			return true
		}
	}
	return false
}

// rolesOf is what the application said the session holds.
func (a *Approvals) rolesOf(c *trilha.Ctx) []string {
	if a.roles == nil || c == nil {
		return nil
	}
	return a.roles(c)
}

// Inbox is what the current session may decide.
func (a *Approvals) Inbox(c *trilha.Ctx, p ListParams) ([]Record, error) {
	if c == nil {
		return nil, nil
	}
	p.Subject, p.Roles = c.Actor().Subject, a.rolesOf(c)
	if p.State == "" {
		p.State = Pending
	}
	return a.store.List(ctxOf(c), p)
}

// List is the whole queue, for an administration screen.
func (a *Approvals) List(ctx context.Context, p ListParams) ([]Record, error) {
	return a.store.List(ctx, p)
}

// Get is one request.
func (a *Approvals) Get(ctx context.Context, id string) (Record, error) { return a.store.Get(ctx, id) }

// clock expires what is past its deadline.
func (a *Approvals) clock() {
	t := time.NewTicker(a.tick)
	defer t.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-t.C:
			a.Expire(context.Background())
		}
	}
}

// Expire moves what is past its deadline, and answers how many. It is
// exported because a test — and an application that would rather run it from
// its own scheduler — needs to ask for it without waiting for a tick.
func (a *Approvals) Expire(ctx context.Context) int {
	now := a.now()
	list, err := a.store.List(ctx, ListParams{State: Pending})
	if err != nil {
		a.log.Warn("approval: reading the queue to expire", "err", err)
		return 0
	}
	n := 0
	for _, r := range list {
		if !r.Late(now) {
			continue
		}
		r.State, r.Decided = Expired, now
		if err := a.store.Save(ctx, r); err != nil {
			a.log.Warn("approval: expiring", "id", r.ID, "err", err)
			continue
		}
		n++
	}
	return n
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "apr_" + hex.EncodeToString(b), nil
}

func ctxOf(c *trilha.Ctx) context.Context {
	if c == nil {
		return context.Background()
	}
	return c.Context()
}

func actorOf(c *trilha.Ctx) string {
	if c == nil {
		return ""
	}
	return c.Actor().Subject
}
