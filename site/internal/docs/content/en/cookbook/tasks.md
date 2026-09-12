---
title: Work that outlives the request
description: trilha/task — the four stages of processing something, with a screen that shows the progress, a panic that does not take the process down, and a store the state survives in.
---

Every management app has the screen that starts something long and then asks "is it done yet?".
What gets written for it is one line:

```go
go func() { processa(docID) }()
```

That line has four defects, and all four show up in production rather than on your machine.

The error goes nowhere — nobody learns it failed. The request's `context` dies when the browser
closes, so half the work stops halfway. A `panic` in there takes down the whole process, web
server included, because a background goroutine has nobody to recover it. And the screen has
nothing to show, because there is no state — there is a goroutine.

## Or: `trilha add tasks`

```bash
trilha add tasks --dry-run
```

```text
  + internal/trabalho/trabalho.go
  + internal/trabalho/trabalho_test.go
  + app/tarefas/page.go
  + tarefas_test.go
  ~ app/setup.go (one line added)

--dry-run: nothing was written
```

That is the engine registered, a progress screen at `/tarefas` and the wiring, with a
placeholder job to replace with your own. What this page adds is the part the recipe cannot
write for you — the four defects above, why `Handle` and `Run` stay apart because of `Retry`,
and the SQL store for when memory is not enough for the history you want to keep. Run the
recipe for the engine and the screen; keep this page for the job itself and for the trade the
whole module makes on purpose — one process, no cross-machine queue.

## This is not a queue

Before anything else, because it decides whether the module is right for you: tasks live in
**one process**. Two replicas are two of these, and the same task runs twice. Nothing survives a
crash except the record saying it was interrupted.

That is the honest shape of it, and it covers what an internal application actually does. If you
need work spread across machines, or retried by somebody else's worker, you want a real queue —
and `Store` is the seam to put it behind.

## Registering and starting

```go
// Novo builds the engine with the handler already registered. The pace of the
// stages is read once, here, so each engine carries its own — a test asks for
// a fast one through the environment, before the app is built, and nothing is
// shared afterwards.
func Novo(hooks *webhook.Hooks) *task.Tasks {
	m := &motor{passo: 250 * time.Millisecond, hooks: hooks}
	if v := os.Getenv("BLOG_TASK_STEP"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			m.passo = d
		}
	}
	t := task.New(task.Options{Workers: 2})
	t.Handle(Nome, m.processar)
	return t
}
```

`Handle` and `Run` are separate on purpose, and the reason is `Retry`. A closure passed to `Run`
lives as long as the process; after a restart, the interrupted task would have no function to
try again with — and the task somebody wants to retry is usually exactly the one that died in a
deploy. Registered by name, the button works.

```go
	motor := tarefas.Novo(hooks)
	trilha.Provide(a, motor)
	if err := motor.Setup(a); err != nil {
		return err
	}
```

`Setup` does two things worth knowing about. It hangs `Shutdown` on the app, so a deploy in the
middle of a run waits instead of cutting. And it marks everything the store still calls queued
or running as **interrupted**: those belong to a process that no longer exists, and a task stuck
on "running" forever is the classic bug.

The engine is a value the app provides, not a package variable — the same reason the stores are.
A suite that stands up one server per test gives each one its own, and no test watches another's
tasks finish.

```go
// POST starts the processing of one document and goes to its screen.
func POST(c *trilha.Ctx) error {
	id := c.Form("documento")
	if _, ok := documentos.Um(id); !ok {
		return trilha.Errorf(http.StatusNotFound, "documento não encontrado")
	}
	motor := trilha.Use[*task.Tasks](c)
	// Duplo clique no botão devolve o id da que já está indo, e não uma
	// segunda: é o dedupe por chave, e é por isso que a chave é o documento.
	tarefa, err := motor.Run(c, tarefas.Nome, id)
	if err != nil {
		return err
	}
	return c.Redirect("/tarefas/" + tarefa)
}
```

The key is what makes two runs the same run. With one alive, a second `Run` answers the id of
the first — which is the double click on the button, and the reason the key exists.

## The work

```go
// processar is the work. It reports each stage with Progress.Step and looks at
// the context between them: a task that never looks is a task the shutdown has
// to cancel by force.
func (m *motor) processar(ctx context.Context, p *task.Progress) error {
	doc, ok := documentos.Um(p.Key)
	if !ok {
		return errors.New("documento não existe mais")
	}
	documentos.Marcar(p.Key, "processando")

	estagios := []string{"Extrair texto", "Classificar", "Indexar", "Concluir"}
	for i, estagio := range estagios {
		p.Step(estagio, i+1, len(estagios))
		select {
		case <-ctx.Done():
			// Interrupted is not failed, and the difference is what the screen
			// tells somebody before they press "try again".
			documentos.Marcar(p.Key, "fila")
			return ctx.Err()
		case <-time.After(m.passo):
		}
		// Um documento com "erro" no nome falha de propósito: é o caminho que
		// o exemplo precisa mostrar tanto quanto o que dá certo.
		if i == 1 && strings.Contains(strings.ToLower(doc.Nome), "erro") {
			documentos.Marcar(p.Key, "fila")
			m.avisa("documento.falhou", doc)
			return errors.New("não consegui classificar: o arquivo não tem texto")
		}
	}
	documentos.Marcar(p.Key, "pronto")
	m.avisa("documento.processado", doc)
	return nil
}
```

There is no `Args`: the key is the id, and everything else the function looks up in your own
table. That is the same trade `c.Link` makes, and for the same reason — what travels stays small
and stays true.

`Step` writes to the store, so it costs a write. Call it once per stage, not once per row.

The `hooks` in `Novo(hooks)` and the `m.avisa` at the end are the other module: when the work
finishes, somebody outside is told. That pairing is the shape this has in a real application —
long work happens, and a partner needs to know — and the failure to tell does not fail the task:
the document was processed, and a notice that did not go out is a delivery the
[webhooks screen](/cookbook/webhooks) shows. Doing it the other way round would reprocess a
document because somebody else's server is down.

A `panic` inside becomes a recorded error with the stack in the log. Losing the web server
because one document had a bad page is not a trade anybody would make on purpose.

## The screen

```go
// Page draws the progress, and only the progress when asked for the fragment.
func Page(c *trilha.Ctx) (h.Node, error) {
	motor := trilha.Use[*task.Tasks](c)
	id := c.Param("id")
	if c.Fragment() == "trilha-task" {
		return ui.TaskProgress(c, motor, id), nil
	}
	t, err := motor.Get(c.Context(), id)
	if err != nil {
		return nil, trilha.Errorf(http.StatusNotFound, "tarefa não encontrada")
	}
	c.SetTitle("Processando " + t.Key)
	return h.Div(
		ui.PageHeader("Processando "+t.Key, ui.Back{Href: "/tarefas", Label: "Tarefas"}),
		ui.Muted(h.Text("Esta página se atualiza sozinha e para quando a tarefa termina.")),
		ui.Card(ui.CardContent(ui.TaskProgress(c, motor, id))),
		h.P(h.A(h.Href("/tarefas"), h.Text("Voltar para as tarefas"))),
		ui.LiveScript(c),
	), nil
}
```

`ui.TaskProgress` is a `ui.Poll` that stops on its own: while the task is alive the fragment
asks again every two seconds, and the moment it ends the route answers `Ctx.PollStop` and the
browser stops. Without JavaScript it is the state as of the load — old, never broken — and
reloading is the refresh.

With no step count it draws an **indeterminate** bar rather than 0%: a bar stuck at zero reads
as broken, and a moving one reads as going.

The administration screen is `ui.TaskTable`, and the retry button is a POST of its own:

```go
// POST runs the task again and goes to the new run's screen.
func POST(c *trilha.Ctx) error {
	motor := trilha.Use[*task.Tasks](c)
	novo, err := motor.Retry(c, c.Form("id"))
	if err != nil {
		return trilha.Errorf(http.StatusNotFound, "tarefa não encontrada")
	}
	c.Flash("info", "Rodando de novo.")
	return c.Redirect("/tarefas/" + novo)
}
```

`Retry` answers a **new** id, and the failed run stays in the store. A screen that swallows the
failure loses the reason somebody pressed the button.

## Keeping the history: a SQL store

`Memory()` is the default and is right for most apps — the tasks do not survive a restart
either. A table buys you the history, and "interrupted at 14:32 by the deploy" is a sentence
somebody eventually needs.

```go
// TaskSchema is the table. Two indexes and both are load-bearing: the first is
// the deduplication, which runs on every Run, and the second is the listing,
// which is the only order the screen ever asks for.
const TaskSchema = `
CREATE TABLE IF NOT EXISTS trilha_tasks (
	id       TEXT PRIMARY KEY,
	name     TEXT NOT NULL,
	key      TEXT NOT NULL,
	state    TEXT NOT NULL,
	step     TEXT NOT NULL DEFAULT '',
	n        INTEGER NOT NULL DEFAULT 0,
	of_n     INTEGER NOT NULL DEFAULT 0,
	err      TEXT NOT NULL DEFAULT '',
	by_whom  TEXT NOT NULL DEFAULT '',
	queued   TIMESTAMPTZ NOT NULL,
	started  TIMESTAMPTZ,
	ended    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS trilha_tasks_active ON trilha_tasks (name, key) WHERE state IN ('queued','running');
CREATE INDEX IF NOT EXISTS trilha_tasks_recent ON trilha_tasks (queued DESC);
`
```

```go
// Active is the deduplication, and it is the one query that runs on every
// Run: the partial index above exists for this line.
func (s TaskSQL) Active(ctx context.Context, name, key string) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx,
		`SELECT id FROM trilha_tasks WHERE name = $1 AND key = $2 AND state IN ('queued','running') LIMIT 1`,
		name, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}
```

```go
// Interrupt runs once, at boot, and is the reason a table beats memory here:
// it is the row that says the deploy caught this one halfway, instead of a
// screen waiting forever for a process that is gone.
func (s TaskSQL) Interrupt(ctx context.Context, at time.Time) (int, error) {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE trilha_tasks SET state = 'interrupted', ended = $1,
			err = 'the process stopped before this finished'
		WHERE state IN ('queued','running')`, at)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
```

The whole file is in `examples/cookbook/tasks.go`, Postgres flavoured; SQLite and MySQL are the
same file with `?` instead of `$1`.

There is no `task.SQL(db)` in the module for the reason no store in this framework ships one: it
would have to pick a placeholder dialect and own a DDL, and the framework does not own your
schema.

:::note
The deduplication holds a lock across the check and the write, which is what makes it true
rather than likely — and honest only because this runs in one process. With a SQL store and two
replicas, add a unique index on `(name, key) WHERE state IN ('queued','running')` and the
database enforces what the lock cannot.
:::

## Testing it

The example's test is the one worth copying, because it asserts the thing that matters — that
the POST came back before the work finished:

```go
	id, _ := primeiroDoc(t)
	inicio := time.Now()
	rec := c.PostForm("/tarefas", url.Values{"documento": {id}})
	rec.WantStatus(http.StatusSeeOther)
	if passou := time.Since(inicio); passou > time.Second {
		t.Fatalf("o POST esperou %s: a tarefa rodou dentro da requisição", passou)
	}
```

Wait on a condition rather than on the clock. A `time.Sleep` is slow when it passes and a liar
when it fails.

## Scheduling

Work with no request behind it — expiring sessions, a nightly digest — is a different shape:
see [scheduled tasks](/cookbook/scheduled-tasks). A ticker starts it; this module runs it.
