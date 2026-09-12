---
title: task
description: Tasks, Options, Handle, Run, Progress, Store and the two screens — the API of the task package, with the defaults and what each field changes.
---

`import "github.com/emersonjoe/trilha/task"` — the work that does not fit inside a request: the
four stages of processing a document, the batch somebody started, the report that takes a
minute. It keeps state, so a screen has something to show.

## This is not a queue

Tasks live in **one process**. Two replicas are two runners, and the same task runs twice;
nothing survives a crash except the record saying it was interrupted. That is the honest shape,
and it covers what an internal application does. Anything more wants a real queue — `Store` and
the handler registry are the seam to put one behind.

## The runner

```go
func New(o Options) *Tasks                       // starts nothing
func (t *Tasks) Handle(name string, fn Func)     // before Setup
func (t *Tasks) Setup(a *trilha.App) error       // sweeps, starts, hangs Shutdown on the app
func (t *Tasks) Shutdown(ctx context.Context) error
```

`New` opens nothing, so it belongs where the app's other values are built. `Setup` does two
things beyond starting the workers: it hangs `Shutdown` on the app, and it marks everything the
store still calls queued or running as **interrupted** — those belong to a process that no
longer exists, and a task stuck on "running" forever is the classic bug.

| `Options` | Default | What it does |
|---|---|---|
| `Store Store` | `Memory()` | where the state lives |
| `Workers int` | 1 | how many tasks run at once |
| `Queue int` | 128 | how many may wait; a full queue is an error at `Run`, not a slice that grows |
| `Shutdown time.Duration` | 15 s | how long to wait for running tasks before cancelling their context |
| `Logger *slog.Logger` | `slog.Default()` | one line per task, and the stack of a panic |

One worker is the honest default: work that goes here usually fights for the same CPU or the
same external service.

## Registering and starting

```go
type Func func(ctx context.Context, p *Progress) error

func (t *Tasks) Run(c *trilha.Ctx, name, key string) (id string, err error)
func (t *Tasks) Retry(c *trilha.Ctx, id string) (string, error)
func (t *Tasks) Cancel(ctx context.Context, id string) error
func (t *Tasks) Get(ctx context.Context, id string) (Task, error)
func (t *Tasks) List(ctx context.Context, p ListParams) ([]Task, error)
```

`Handle` and `Run` are separate because of `Retry`: a closure passed to `Run` lives as long as
the process, and the task somebody wants to retry is usually the one that died in a deploy.
Registered by name, the button works after a restart.

**Two runs with the same name and key, while one is alive, are one run** — the id that comes
back is the one already going. That is the double click on the button, and the reason the key
exists. Once it ends, the key is free again.

There is no `Args`. The key is the id — the document, the batch — and everything else the
function looks up in your own table, which is the same trade `Ctx.Link` makes.

`Retry` answers a **new** id and leaves the failed run in the store: a screen that swallows the
failure loses the reason somebody pressed the button. Retrying something still alive answers its
id rather than starting a second one.

`Cancel` cancels the task's context. It is not a kill: a function that ignores its context runs
to the end, and one that respects it stops where it looked.

The context the function gets is **not the request's** — closing the browser does not stop the
work — and it cancels when `Shutdown` runs out of patience. A `panic` becomes a recorded error
with the stack in the log, because a panic in a background goroutine otherwise takes the web
server down with it.

## Progress and Task

```go
func (p *Progress) Step(label string, n, of int)   // p.Step("Classificar", 2, 4)

type Progress struct{ ID, Name, Key string }

type Task struct {
	ID, Name, Key string
	State         State  // Queued, Running, Done, Failed, Interrupted
	Step          string
	N, Of         int
	Err           string // already a string: the screen shows it
	By            string // from Ctx.Actor, so it says who pressed the button
	Queued, Started, Ended time.Time
}

func (t Task) Percent() int          // -1 when there is nothing to compute from
func (t Task) Took() time.Duration
func (s State) Live() bool           // queued or running
```

`Step` writes to the store, so it costs a write: call it once per stage, not once per row. A
failure to write is not returned — losing a progress update is no reason to fail work that is
going fine, and the next `Step` corrects the picture.

`Percent` answers **-1** rather than 0 when there is no count, and the kit draws an indeterminate
bar for it: a bar stuck at zero reads as broken.

## Store

```go
type Store interface {
	Save(ctx context.Context, t Task) error
	Get(ctx context.Context, id string) (Task, error)
	List(ctx context.Context, p ListParams) ([]Task, error)
	Active(ctx context.Context, name, key string) (string, error)
	Interrupt(ctx context.Context, at time.Time) (int, error)
}

func Memory() Store
```

Four of the five are obvious; `Interrupt` is the one an application would not have thought to
write. It runs once, at boot.

There is **no SQL implementation here**, the same choice every store in this framework makes: a
shipped one has to pick a placeholder dialect and own a DDL, and the framework does not own your
schema. The [recipe](/cookbook/tasks) carries the whole file.

`ListParams` filters the administration screen — `Name`, `Key`, `State`, `Limit`, `Offset` — and
its zero value is "the last fifty of everything", newest first.

## The screens

```go
func ui.TaskProgress(c *trilha.Ctx, tasks *task.Tasks, id string) h.Node
func ui.TaskTable(c *trilha.Ctx, tasks []task.Task, o ui.TaskTableOpts) h.Node
```

`TaskProgress` is a `ui.Poll` that stops on its own: while the task is alive the fragment asks
again every two seconds, and the moment it ends the route answers `Ctx.PollStop`. Without
JavaScript it is the state as of the load — old, never broken. The id comes from the
application, which knows which task belongs to the screen; passing one the visitor sent would be
letting them watch somebody else's work.

`TaskTableOpts` takes `Retry` (where the button posts; no button without it), `CSRF`
(`trilha.CSRFInput(c)`, required with `Retry`) and `Empty`. The button is drawn only for tasks
that have ended — "try again" on a running one is a second run.

[`trilha add tasks`](/reference/cli#trilha-add) writes the runner, both screens and the test.

@demo ui-tarefas

## Errors

`ErrUnknownTask` is a name nobody registered: an error at `Run` rather than a task sitting queued
forever waiting for a function that does not exist. `ErrNotFound` is an id the store does not
have.

## Not here

Priority, scheduling and automatic retries. Scheduling is a ticker — see
[scheduled tasks](/cookbook/scheduled-tasks) — and retrying by itself hides the failure instead
of showing it.
