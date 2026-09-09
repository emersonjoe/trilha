package cookbook

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/emersonjoe/trilha/task"
)

// TaskSQL keeps the task records in a table, so what ran survives a restart
// and the administration screen has a history. The tasks themselves do not
// survive — they never do — but "interrupted, at 14:32, by the deploy" is the
// sentence somebody needs.
//
// It is here and not in the module for the reason every store in this
// framework is: a shipped SQL store has to pick a placeholder dialect and own
// a DDL, and the framework does not own your schema. This is Postgres; SQLite
// and MySQL are the same file with ? instead of $1.
type TaskSQL struct{ DB *sql.DB }

// The compiler is the check that this file is still a Store: a method whose
// signature drifts is a runtime surprise otherwise, three deploys later.
var _ task.Store = TaskSQL{}

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

// Save writes the record whole. An upsert and not an update: the first write
// of a task is a create, and every one after it is a replace.
func (s TaskSQL) Save(ctx context.Context, t task.Task) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO trilha_tasks (id, name, key, state, step, n, of_n, err, by_whom, queued, started, ended)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (id) DO UPDATE SET
			state = EXCLUDED.state, step = EXCLUDED.step, n = EXCLUDED.n,
			of_n = EXCLUDED.of_n, err = EXCLUDED.err,
			started = EXCLUDED.started, ended = EXCLUDED.ended`,
		t.ID, t.Name, t.Key, string(t.State), t.Step, t.N, t.Of, t.Err, t.By,
		t.Queued, nullTime(t.Started), nullTime(t.Ended))
	return err
}

// Get answers task.ErrNotFound, and not sql.ErrNoRows: the caller checks the
// module's error, and leaking the driver's would make every caller import
// database/sql to handle a missing id.
func (s TaskSQL) Get(ctx context.Context, id string) (task.Task, error) {
	row := s.DB.QueryRowContext(ctx, taskColumns+` WHERE id = $1`, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, task.ErrNotFound
	}
	return t, err
}

// List is the administration screen: newest first, filtered by whatever the
// person chose.
func (s TaskSQL) List(ctx context.Context, p task.ListParams) ([]task.Task, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, taskColumns+`
		WHERE ($1 = '' OR name = $1) AND ($2 = '' OR key = $2) AND ($3 = '' OR state = $3)
		ORDER BY queued DESC, id DESC LIMIT $4 OFFSET $5`,
		p.Name, p.Key, string(p.State), limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []task.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

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

const taskColumns = `SELECT id, name, key, state, step, n, of_n, err, by_whom, queued, started, ended FROM trilha_tasks`

// scanner is what QueryRow and Rows have in common, so one function reads a
// row for both.
type scanner interface{ Scan(dest ...any) error }

func scanTask(r scanner) (task.Task, error) {
	var t task.Task
	var estado string
	var comecou, terminou sql.NullTime
	err := r.Scan(&t.ID, &t.Name, &t.Key, &estado, &t.Step, &t.N, &t.Of, &t.Err, &t.By,
		&t.Queued, &comecou, &terminou)
	t.State = task.State(estado)
	t.Started, t.Ended = comecou.Time, terminou.Time
	return t, err
}

// nullTime keeps a zero time out of the column: a task that never started has
// no start, and writing year 1 there makes every report subtract from it.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
