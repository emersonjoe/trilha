// Package mail sends the handful of messages an internal application actually
// sends: the invitation, the link, the notice that a flow finished. It is not
// a mail platform — it is the part every application writes badly by hand.
//
// What gets written by hand is net/smtp inside the handler, and the questions
// that stops on are never about the product: 587 or 465, STARTTLS or implicit
// TLS, PLAIN or LOGIN, how to write a body without going back to 2003 tables,
// how to test without mailing a real person. The worst of them is invisible:
// smtp.SendMail has no deadline, so a slow server pins the handler until TCP
// gives up, which the visitor sees as a page that spins.
//
// The body is an h.Node, the same language the pages are written in, and every
// message goes out as multipart/alternative — the plain text is generated from
// the same node, so a client without HTML reads it whole and a link becomes
// "text <https://…>" instead of disappearing.
package mail

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/task"
)

// ErrNotConfigured is what Send answers in production when no server was
// configured. Writing the file to ./mail would be worse than the error: the
// application would report success, and nobody would find out until someone
// asked why the invitation never arrived.
var ErrNotConfigured = errors.New("mail: no server configured (TRILHA_MAIL_URL)")

// Transport is where a message goes. One method, so an application that sends
// through an HTTP provider writes it instead of this package growing a driver
// for every one of them.
//
// raw is the complete message, headers included, ready for the wire. from and
// to are the envelope — which is not always what the headers say, and that is
// on purpose: a Bcc is a recipient of the envelope and of no header.
type Transport interface {
	Deliver(ctx context.Context, from string, to []string, raw []byte) error
}

// Options configures a Mailer. FromEnv fills it from the environment, which is
// what most applications want.
type Options struct {
	// From is the sender, in either form: "no-reply@org.br" or
	// "Acervo <no-reply@org.br>". A Message may override it.
	From string
	// Transport is where messages go. nil means the dev directory when
	// TRILHA_ENV is dev, and nothing at all otherwise — see ErrNotConfigured.
	Transport Transport
	// Timeout bounds one delivery attempt (default 10 s). It is a deadline on
	// the context, so a handler that gives up hangs up. A server that checks
	// for spam while it receives — an authenticated submission on 465 is often
	// one — can take longer than that: with Tasks set nobody waits for it, so
	// raise it freely.
	Timeout time.Duration
	// Tasks makes Send asynchronous: the message is built at once — a bad
	// address is still an error at Send — then queued on this runner, and
	// Send returns. A temporary failure (a 4xx answer, a timeout, a server that
	// did not answer) is tried again after each wait in Backoff; a permanent
	// one (5xx) is not. The task shows on the runner's screens as "mail.send".
	//
	// The message waits in this process's memory, because a task carries no
	// payload: a restart before delivery loses it, and the runner records the
	// task as interrupted. That is the task package's shape, said out loud.
	Tasks *task.Tasks
	// Backoff is the wait before each new attempt when Tasks is set (default
	// 5 s, 30 s, 2 min: four attempts in all).
	Backoff []time.Duration
	// Logger receives one line per message (default slog.Default()).
	Logger *slog.Logger
}

// Mailer sends messages. It is safe for concurrent use, and it opens no
// connection until the first Send.
type Mailer struct {
	from      string
	transport Transport
	timeout   time.Duration
	log       *slog.Logger
	tasks     *task.Tasks
	backoff   []time.Duration
}

// New builds a Mailer. It touches no network and reads no file, so it belongs
// in a var at package level, next to the rest of the app's setup.
func New(o Options) *Mailer {
	m := &Mailer{from: strings.TrimSpace(o.From), transport: o.Transport,
		timeout: o.Timeout, log: o.Logger}
	if m.timeout <= 0 {
		m.timeout = 10 * time.Second
	}
	if m.log == nil {
		m.log = slog.Default()
	}
	if o.Tasks != nil {
		m.tasks, m.backoff = o.Tasks, o.Backoff
		if m.backoff == nil {
			m.backoff = []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute}
		}
		handle(o.Tasks)
	}
	return m
}

// Message is one message. Body is an h.Node — the same nodes a page is written
// with — and everything else is a header.
type Message struct {
	To  []string
	Cc  []string
	Bcc []string
	// From overrides the Mailer's sender, for the application that sends as
	// more than one address.
	From string
	// ReplyTo is where an answer should go, when it is not the sender. A
	// no-reply address that bounces the reply is a support ticket nobody sees.
	ReplyTo string
	Subject string
	// Body is the HTML. Layout wraps it in something that renders in the
	// clients people actually use.
	Body h.Node
	// Text is the plain alternative. Left empty — which is the normal case —
	// it is generated from Body.
	Text string
	// Headers are extra headers, for the ones this struct has no field for
	// (List-Unsubscribe, an X- of your own). The ones this package writes
	// cannot be overridden here.
	Headers map[string]string
}

// Send builds the message and hands it to the transport. Without
// Options.Tasks it blocks until the server accepted it or the deadline passed.
// With Tasks it returns once the message is queued, and delivery — with its
// new attempts — happens on the runner; what Send still answers is everything
// that can be known without the server: no transport, no sender, no
// recipients, a header that does not parse, a full queue.
func (m *Mailer) Send(ctx context.Context, msg Message) error {
	if m.transport == nil {
		return ErrNotConfigured
	}
	from := msg.From
	if from == "" {
		from = m.from
	}
	if from == "" {
		return errors.New("mail: no sender (Options.From or Message.From)")
	}
	raw, envelope, err := build(from, msg)
	if err != nil {
		return err
	}
	if len(envelope) == 0 {
		return errors.New("mail: no recipients")
	}
	j := &job{m: m, subject: msg.Subject, from: address(from), to: envelope, raw: raw}
	if m.tasks != nil {
		return m.enqueue(j)
	}
	// A nil context is a mistake, and panicking inside package context for it
	// hands back a stack trace that points at context.go. It happens from a
	// background job, where there was never a request to take one from.
	if ctx == nil {
		ctx = context.Background()
	}
	return m.deliver(ctx, j)
}

// deliver is one attempt, bounded by the timeout, with its line in the log.
func (m *Mailer) deliver(ctx context.Context, j *job) error {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	start := time.Now()
	if err := m.transport.Deliver(ctx, j.from, j.to, j.raw); err != nil {
		m.log.Error("mail", "subject", j.subject, "to", strings.Join(j.to, ","),
			"took", time.Since(start).Round(time.Millisecond), "err", err)
		return fmt.Errorf("mail: sending %q: %w", j.subject, err)
	}
	m.log.Info("mail", "subject", j.subject, "to", strings.Join(j.to, ","),
		"took", time.Since(start).Round(time.Millisecond))
	return nil
}
