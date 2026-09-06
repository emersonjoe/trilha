---
title: Scheduled tasks
description: A ticker that starts with the app and stops with it, one tick at a time, a panic that does not take the process down — and the line where cron becomes the right answer.
---

Some work has no request behind it: expiring sessions, sending a digest, retrying what failed.
While there is one instance, a goroutine with a ticker is the whole answer and it lives inside
the app, with the same pool and the same logger.

## The shape

```go
// Job is something that runs on a schedule inside the process. It is the
// right shape while one instance runs it; the moment there are two, the
// answer is a queue or a lock in the database, not a second ticker.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(context.Context) error
}
```

```go
// Start runs the job until the context is cancelled. A tick that arrives
// while the previous run is still going is dropped, not queued: a job that
// takes longer than its interval must not pile up copies of itself.
func Start(ctx context.Context, log *slog.Logger, j Job) {
	t := time.NewTicker(j.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runJob(ctx, log, j)
		}
	}
}
```

A `time.Ticker` drops a tick when nobody is receiving, and that is the behaviour you want: a
job that takes longer than its interval must not accumulate copies of itself. The loop calls
the job and only then waits again.

```go
// runJob keeps one tick from taking the process down. A panic in a
// background task is a bug, and a bug in one task should not stop the ones
// that work.
func runJob(ctx context.Context, log *slog.Logger, j Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("job panicked", "job", j.Name, "panic", r)
		}
	}()
	start := time.Now()
	if err := j.Run(ctx); err != nil {
		log.Error("job failed", "job", j.Name, "error", err, "took", time.Since(start))
		return
	}
	log.Info("job done", "job", j.Name, "took", time.Since(start))
}
```

The `recover` is not optimism about your code. A panic in a goroutine takes the whole process
down — the HTTP server included — so an unhandled bug in the nightly digest would stop the
site. Logged and skipped, it costs one run.

## Starting and stopping

```go
// SetupJobs starts the tasks and makes the shutdown wait for them. A job
// killed halfway is a row written and an e-mail not sent, and it is the
// hardest kind of bug to reproduce.
func SetupJobs(a *trilha.App) error {
	ctx, stop := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, j := range []Job{
		{Name: "expire-sessions", Every: time.Hour, Run: expireSessions},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Start(ctx, a.Logger(), j)
		}()
	}
	a.OnShutdown(func(*trilha.App) error {
		stop()
		wg.Wait()
		return nil
	})
	return nil
}
```

The context cancels on shutdown and the `WaitGroup` makes the process wait for the tick in
flight. Without it, a deploy kills a job halfway: the row was written, the e-mail was not, and
it is the hardest kind of bug to reproduce.

The job itself is an ordinary function taking a context, so a test calls it directly, with no
timer involved:

```go
// expireSessions is an ordinary function taking a context: nothing about it
// knows it runs on a timer, so a test calls it directly.
func expireSessions(ctx context.Context) error {
	_, err := DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	return err
}
```

## When one instance stops being true

The moment there are two instances, both tick. Every job runs twice, and "send the digest"
becomes "send the digest twice".

| Situation | What to do |
|---|---|
| one instance | this recipe |
| more than one, idempotent job | keep it; running twice changes nothing |
| more than one, job that must run once | a lock in the database |
| work that must survive a restart | a queue, not a ticker |

The lock is smaller than it sounds — one row per job name, a conditional update that only
succeeds for the instance that gets there first:

```sql
UPDATE job_locks SET locked_until = now() + interval '5 minutes', owner = $1
WHERE name = $2 AND locked_until < now();
```

If the update touched no rows, another instance is running it, and this one skips the tick.

:::note
And there is nothing wrong with `cron` calling a route with a token, or a systemd timer
running your binary with a subcommand. The advantage is separation: a task that hangs does not
hold a slot in the server. The cost is one more thing to deploy.
:::
