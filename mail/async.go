package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"sync"
	"time"

	"github.com/emersonjoe/trilha/task"
)

// TaskName is the name the queued deliveries carry on the task runner.
const TaskName = "mail.send"

// job is one built message waiting for its delivery.
type job struct {
	m       *Mailer
	subject string
	from    string
	to      []string
	raw     []byte
}

var (
	// pending holds the messages between Send and the worker, by the task's
	// key: a task carries a key and no payload.
	pending sync.Map // string → *job
	// handled is the runners that already know TaskName, so two Mailers on
	// the same runner register it once.
	handled sync.Map // *task.Tasks → struct{}
)

func handle(t *task.Tasks) {
	if _, done := handled.LoadOrStore(t, struct{}{}); !done {
		t.Handle(TaskName, run)
	}
}

func (m *Mailer) enqueue(j *job) error {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	key := hex.EncodeToString(b)
	pending.Store(key, j)
	if _, err := m.tasks.Run(nil, TaskName, key); err != nil {
		pending.Delete(key)
		return fmt.Errorf("mail: queueing %q: %w", j.subject, err)
	}
	return nil
}

// run is the task: attempts until one is accepted, the failure is permanent,
// the waits run out, or the runner shuts down.
func run(ctx context.Context, p *task.Progress) error {
	v, ok := pending.LoadAndDelete(p.Key)
	if !ok {
		// A retry from the runner's screen, after the message was delivered or
		// given up on: the payload lived in memory and is gone.
		return errors.New("mail: the message is no longer in memory; send it again")
	}
	j := v.(*job)
	for attempt := 0; ; attempt++ {
		err := j.m.deliver(ctx, j)
		if err == nil {
			return nil
		}
		if !temporary(err) || attempt == len(j.m.backoff) {
			return err
		}
		p.Step("waiting to try again", attempt+1, len(j.m.backoff)+1)
		wait := time.NewTimer(j.m.backoff[attempt])
		select {
		case <-ctx.Done():
			wait.Stop()
			return ctx.Err()
		case <-wait.C:
		}
	}
}

// temporary says whether trying again could work. A 4xx answer is the server
// saying "later" (greylisting, a full mailbox, too many connections); a 5xx is
// "no", and asking again is how a sender ends up on a block list. A timeout or
// a server that did not answer is the network. A Transport of the
// application's own marks its errors with a Temporary() bool method.
// Anything else — a certificate that does not verify, say — is permanent.
func temporary(err error) bool {
	var tp *textproto.Error
	if errors.As(err, &tp) {
		return tp.Code >= 400 && tp.Code < 500
	}
	var op *net.OpError
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &op) {
		return true
	}
	var t interface{ Temporary() bool }
	return errors.As(err, &t) && t.Temporary()
}
