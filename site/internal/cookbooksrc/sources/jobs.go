package cookbook

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
)

// Job is something that runs on a schedule inside the process. It is the
// right shape while one instance runs it; the moment there are two, the
// answer is a queue or a lock in the database, not a second ticker.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(context.Context) error
}

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

// expireSessions is an ordinary function taking a context: nothing about it
// knows it runs on a timer, so a test calls it directly.
func expireSessions(ctx context.Context) error {
	_, err := DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	return err
}
