// Package task runs the work that does not fit inside a request: the four
// stages of processing a document, the batch somebody started, the report that
// takes a minute. It keeps state, so a screen has something to show.
//
// What it replaces is one line:
//
//	go func() { processa(docID) }()
//
// which has four defects that only show up in production. The error goes
// nowhere. The request's context dies when the browser closes, so half the
// work stops halfway. A panic in there takes down the process and not just the
// task. And the screen has nothing to show, because there is no state — there
// is a goroutine.
//
// # This is not a queue
//
// Tasks live in one process. Two replicas are two of these, and the same task
// runs twice; nothing survives a crash except the record saying it was
// interrupted. That is the honest shape of it, and it covers what an internal
// application actually does. Anything more wants a real queue — Store and the
// handler registry are the seam to swap it behind.
package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// State is where a task is. Five values, and the fifth is the one people
// forget: a task whose process is gone.
type State string

const (
	Queued      State = "queued"
	Running     State = "running"
	Done        State = "done"
	Failed      State = "error"
	Interrupted State = "interrupted"
)

// Live answers whether the task is still expected to finish. It is what the
// deduplication asks, and what the screen asks before polling again.
func (s State) Live() bool { return s == Queued || s == Running }

var (
	// ErrUnknownTask is a name nobody registered with Handle. It is an error
	// at Run rather than a task that sits queued forever waiting for a
	// function that does not exist.
	ErrUnknownTask = errors.New("task: no handler registered for this name")
	// ErrNotFound is an id the store does not have.
	ErrNotFound = errors.New("task: not found")
)

// Task is the record a screen reads. It is a value, not a handle: what comes
// out of the store is a copy of how things were, which is what a page renders.
type Task struct {
	ID   string
	Name string
	// Key is what the task is about — the document id, the batch id. It is
	// what makes two runs the same run, and it reaches the function as
	// Progress.Key.
	Key   string
	State State
	// Step, N and Of are the progress: "Classificar", 2, 4.
	Step string
	N    int
	Of   int
	// Err is the message of the failure, already a string: the screen shows
	// it and the store keeps it.
	Err string
	// By is who started it, from the audit identity of the request.
	By string

	Queued  time.Time
	Started time.Time
	Ended   time.Time
}

// Func is the work. It gets a context that is not the request's — closing the
// browser does not stop it — and cancels when the app shuts down.
type Func func(ctx context.Context, p *Progress) error

// Options configures the runner.
type Options struct {
	// Store is where the state lives. nil is Memory(), which is right for a
	// single-process app that does not need the history to survive a restart.
	Store Store
	// Workers is how many tasks run at once (default 1). One is the honest
	// default: work that goes here is usually work that fights for the same
	// CPU or the same external service.
	Workers int
	// Queue is how many tasks may wait (default 128). A full queue is an
	// error at Run and not an unbounded slice: the screen can say "try again
	// in a minute", and a memory leak cannot say anything.
	Queue int
	// Shutdown is how long to wait for running tasks (default 15 s). After
	// it, their context is cancelled and the wait ends.
	Shutdown time.Duration
	Logger   *slog.Logger
}

// Tasks is the runner. One per application, in a var next to the rest of the
// setup: New starts nothing, Setup does.
type Tasks struct {
	store    Store
	workers  int
	queue    chan string
	shutdown time.Duration
	log      *slog.Logger

	mu       sync.Mutex
	handlers map[string]Func
	running  map[string]context.CancelFunc

	started bool
	stop    chan struct{}
	done    sync.WaitGroup

	// now is a hook for the tests, which need the clock to hold still.
	now func() time.Time
}

// New builds the runner. It starts no goroutine and touches no store, so it
// belongs at package level.
func New(o Options) *Tasks {
	t := &Tasks{
		store: o.Store, workers: o.Workers, shutdown: o.Shutdown, log: o.Logger,
		handlers: map[string]Func{}, running: map[string]context.CancelFunc{},
		stop: make(chan struct{}), now: time.Now,
	}
	if t.store == nil {
		t.store = Memory()
	}
	if t.workers <= 0 {
		t.workers = 1
	}
	if t.shutdown <= 0 {
		t.shutdown = 15 * time.Second
	}
	if t.log == nil {
		t.log = slog.Default()
	}
	size := o.Queue
	if size <= 0 {
		size = 128
	}
	t.queue = make(chan string, size)
	return t
}

// Handle registers the function that runs a name. Call it in Setup, before
// Setup starts the workers.
//
// Registering by name instead of passing a closure to Run is what makes Retry
// work: a closure lives as long as the process, and the task somebody wants to
// retry is usually the one that died in a deploy.
func (t *Tasks) Handle(name string, fn Func) {
	if name == "" || fn == nil {
		panic("task: Handle needs a name and a function")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, taken := t.handlers[name]; taken {
		panic("task: two handlers registered for " + name)
	}
	t.handlers[name] = fn
}

// Setup starts the workers and hangs the shutdown on the app. It also marks
// everything the store still calls queued or running as interrupted: those
// belong to a process that no longer exists, and a task stuck on "running"
// forever is the classic bug this avoids.
//
//	func Setup(a *trilha.App) error {
//		Tarefas.Handle("processar", processar)
//		return Tarefas.Setup(a)
//	}
func (t *Tasks) Setup(a *trilha.App) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := t.store.Interrupt(ctx, t.now())
	if err != nil {
		return fmt.Errorf("task: sweeping what the last process left: %w", err)
	}
	if n > 0 {
		t.log.Warn("task", "interrupted", n,
			"msg", "tasks from a previous process were marked interrupted")
	}
	t.start()
	if a != nil {
		a.OnShutdown(func(*trilha.App) error { return t.Shutdown(context.Background()) })
	}
	return nil
}

func (t *Tasks) start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return
	}
	t.started = true
	// A fresh channel, because Shutdown closed the last one: without this a
	// runner that was stopped and started again gets workers that return
	// immediately, and tasks that queue forever.
	t.stop = make(chan struct{})
	for i := 0; i < t.workers; i++ {
		t.done.Add(1)
		go t.work(t.stop)
	}
}

// Run starts a task and answers its id.
//
// Two runs with the same name and key, while one is still alive, are one run:
// the id that comes back is the one already going. That is the double click on
// the button, and it is why the key exists.
func (t *Tasks) Run(c *trilha.Ctx, name, key string) (string, error) {
	t.mu.Lock()
	_, known := t.handlers[name]
	t.mu.Unlock()
	if !known {
		return "", fmt.Errorf("%w: %s", ErrUnknownTask, name)
	}
	return t.enqueue(c, name, key)
}

// Retry starts the task again, with the same name and key. It answers a new
// id: the failed run stays in the store, because a screen that loses the
// failure loses the reason somebody pressed the button.
func (t *Tasks) Retry(c *trilha.Ctx, id string) (string, error) {
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
	}
	old, err := t.store.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if old.State.Live() {
		return old.ID, nil // ainda está indo; tentar de novo é esperar
	}
	return t.Run(c, old.Name, old.Key)
}

func (t *Tasks) enqueue(c *trilha.Ctx, name, key string) (string, error) {
	ctx := context.Background()
	if c != nil {
		ctx = c.Context()
	}
	// The lock spans the check and the write, which is what makes the
	// deduplication true rather than likely — and it is honest only because
	// this runs in one process, which is the first thing the package doc says.
	t.mu.Lock()
	defer t.mu.Unlock()

	if id, err := t.store.Active(ctx, name, key); err != nil {
		return "", err
	} else if id != "" {
		return id, nil
	}
	task := Task{ID: newID(), Name: name, Key: key, State: Queued, Queued: t.now(), By: who(c)}
	if err := t.store.Save(ctx, task); err != nil {
		return "", err
	}
	select {
	case t.queue <- task.ID:
	default:
		// A full queue is an answer and not a slice that grows: the screen can
		// say "in a minute", and a leak cannot say anything.
		task.State, task.Err, task.Ended = Failed, "the queue is full", t.now()
		_ = t.store.Save(ctx, task)
		return "", fmt.Errorf("task: the queue is full (%d waiting); raise Options.Queue or add workers", cap(t.queue))
	}
	if c != nil {
		c.Audit("task.disparou", task.ID, trilha.Fields{"task": name, "chave": key})
	}
	return task.ID, nil
}

// Get answers one task.
func (t *Tasks) Get(ctx context.Context, id string) (Task, error) { return t.store.Get(ctx, id) }

// List answers the tasks a screen shows, newest first.
func (t *Tasks) List(ctx context.Context, p ListParams) ([]Task, error) {
	return t.store.List(ctx, p)
}

// Cancel stops a running task by cancelling its context. It is not a kill: a
// function that ignores its context runs to the end, and one that respects it
// stops where it looked.
func (t *Tasks) Cancel(ctx context.Context, id string) error {
	t.mu.Lock()
	cancel := t.running[id]
	t.mu.Unlock()
	if cancel == nil {
		return ErrNotFound
	}
	cancel()
	return nil
}

// Shutdown waits for what is running, up to Options.Shutdown, and then
// cancels. It is hung on the app by Setup, so an ordinary stop lets a task
// finish and a stuck one does not hold the deploy.
func (t *Tasks) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return nil
	}
	t.started = false
	close(t.stop)
	t.mu.Unlock()

	esperou := make(chan struct{})
	go func() { t.done.Wait(); close(esperou) }()

	prazo := time.NewTimer(t.shutdown)
	defer prazo.Stop()
	select {
	case <-esperou:
		return nil
	case <-ctx.Done():
	case <-prazo.C:
	}
	// Time is up: cancel every context and wait for the functions to notice.
	t.mu.Lock()
	for _, cancel := range t.running {
		cancel()
	}
	t.mu.Unlock()
	<-esperou
	return nil
}

// work takes the stop channel by value rather than reading the field: the
// field is written under the lock when the workers start, and a goroutine
// reading it without one is a race the detector would find on the next
// restart.
func (t *Tasks) work(stop chan struct{}) {
	defer t.done.Done()
	for {
		select {
		case <-stop:
			return
		case id := <-t.queue:
			t.run(id)
		}
	}
}

// run is one task, from the store back to the store. Everything that can go
// wrong inside the function ends as a record, which is the whole point.
func (t *Tasks) run(id string) {
	ctx, cancel := context.WithCancel(context.Background())
	t.mu.Lock()
	t.running[id] = cancel
	t.mu.Unlock()
	defer func() {
		cancel()
		t.mu.Lock()
		delete(t.running, id)
		t.mu.Unlock()
	}()

	task, err := t.store.Get(ctx, id)
	if err != nil {
		t.log.Error("task", "id", id, "err", err)
		return
	}
	t.mu.Lock()
	fn := t.handlers[task.Name]
	t.mu.Unlock()
	if fn == nil {
		t.finish(ctx, task, fmt.Errorf("%w: %s", ErrUnknownTask, task.Name))
		return
	}

	task.State, task.Started = Running, t.now()
	if err := t.store.Save(ctx, task); err != nil {
		t.log.Error("task", "id", id, "err", err)
	}
	p := &Progress{ID: task.ID, Name: task.Name, Key: task.Key, tasks: t}
	t.finish(ctx, task, t.call(ctx, fn, p))
}

// call runs the function with the panic turned into an error. A panic in a
// background goroutine takes down the whole process, and losing the web server
// because one document had a bad page is not a trade anybody would make on
// purpose.
func (t *Tasks) call(ctx context.Context, fn Func, p *Progress) (err error) {
	defer func() {
		if r := recover(); r != nil {
			t.log.Error("task panic", "id", p.ID, "task", p.Name, "key", p.Key,
				"panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("task: panic: %v", r)
		}
	}()
	return fn(ctx, p)
}

func (t *Tasks) finish(ctx context.Context, task Task, err error) {
	// The task's own context is cancelled by now in the shutdown path, so the
	// last write gets one of its own — the record of what happened must not be
	// the thing that gets lost.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	fresco, erro := t.store.Get(ctx, task.ID)
	if erro == nil {
		task = fresco
	}
	task.Ended = t.now()
	switch {
	case err == nil:
		task.State = Done
	case errors.Is(err, context.Canceled):
		task.State, task.Err = Interrupted, "cancelled"
	default:
		task.State, task.Err = Failed, err.Error()
	}
	if erro := t.store.Save(ctx, task); erro != nil {
		t.log.Error("task", "id", task.ID, "err", erro)
	}
	if err != nil {
		t.log.Error("task", "id", task.ID, "task", task.Name, "key", task.Key,
			"took", task.Ended.Sub(task.Started).Round(time.Millisecond), "err", err)
		return
	}
	t.log.Info("task", "id", task.ID, "task", task.Name, "key", task.Key,
		"took", task.Ended.Sub(task.Started).Round(time.Millisecond))
}

// who is the identity the audit already knows, so the record says who pressed
// the button without this package inventing a second notion of user.
func who(c *trilha.Ctx) string {
	if c == nil {
		return ""
	}
	a := c.Actor()
	for _, v := range []string{a.Email, a.Name, a.Subject} {
		if v != "" {
			return v
		}
	}
	return a.Via
}
