package task

import (
	"context"
	"time"
)

// Progress is what the function reports through, and what the screen reads.
// It carries the identity of the task so the function does not have to close
// over anything: Key is the document id, the batch id, whatever the run is
// about.
type Progress struct {
	ID   string
	Name string
	Key  string

	tasks *Tasks
}

// Step records where the work is: the label somebody reads, and how far along
// it is.
//
//	p.Step("Classificar", 2, 4)
//
// It writes to the store, so it costs a write — call it once per stage and not
// once per row. A failure to write is not returned: losing a progress update
// is not a reason to fail work that is going fine, and the next Step corrects
// the picture anyway.
func (p *Progress) Step(label string, n, of int) {
	if p == nil || p.tasks == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t, err := p.tasks.store.Get(ctx, p.ID)
	if err != nil {
		return
	}
	t.Step, t.N, t.Of = label, n, of
	if err := p.tasks.store.Save(ctx, t); err != nil {
		p.tasks.log.Warn("task", "id", p.ID, "step", label, "err", err)
	}
}

// Percent is the progress as a whole number, and -1 when there is nothing to
// compute it from. It is here rather than in the template because "0 of 0" is
// a division somebody does eventually.
func (t Task) Percent() int {
	if t.State == Done {
		return 100
	}
	if t.Of <= 0 || t.N <= 0 {
		return -1
	}
	if t.N >= t.Of {
		return 100
	}
	return t.N * 100 / t.Of
}

// Took is how long the task ran, and how long it has been running when it has
// not finished.
func (t Task) Took() time.Duration {
	if t.Started.IsZero() {
		return 0
	}
	end := t.Ended
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(t.Started).Round(time.Second)
}
