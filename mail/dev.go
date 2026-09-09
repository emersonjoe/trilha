package mail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Dir writes messages to a directory instead of sending them, as .eml files —
// the format every mail client opens by double-clicking, so what you check is
// the real message and not a rendering of it made for the check.
//
// This is the default in development, and it is on purpose that it is a file
// and not a line in the log: an email is a layout, and a layout is something
// you look at.
type Dir string

// Deliver implements Transport.
func (d Dir) Deliver(ctx context.Context, from string, to []string, raw []byte) error {
	dir := string(d)
	if dir == "" {
		dir = "mail"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%s.eml", now().Format("2006-01-02T15-04-05"), slug(subjectOf(raw)))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	// The path is on the message the Mailer logs, so the terminal says where
	// to look instead of saying that something was written somewhere.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	fmt.Fprintf(os.Stderr, "mail: wrote %s\n", path)
	return nil
}

// Outbox keeps messages in memory, which is how an application tests that it
// sent the right one. It is here and not in a _test.go file because the test
// that needs it is the application's, not this package's.
//
// It is what mail.Sent(t) would have been. A package that is not a test
// package cannot import testing: everything that imports it registers the test
// flags in every binary of the project, and -test.v on a web server is a
// surprising thing to ship.
type Outbox struct {
	mu   sync.Mutex
	sent []Sent
}

// Sent is one message the Outbox kept, already taken apart: the test asserts
// on the link inside the body, not on quoted-printable.
type Sent struct {
	From    string
	To      []string
	Subject string
	Text    string
	HTML    string
	Raw     []byte
	At      time.Time
}

// Deliver implements Transport.
func (o *Outbox) Deliver(ctx context.Context, from string, to []string, raw []byte) error {
	text, html := partsOf(raw)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent = append(o.sent, Sent{From: from, To: append([]string{}, to...),
		Subject: subjectOf(raw), Text: text, HTML: html, Raw: raw, At: now()})
	return nil
}

// Messages is everything sent so far, oldest first.
func (o *Outbox) Messages() []Sent {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]Sent{}, o.sent...)
}

// Last is the most recent message, which is what a test almost always wants.
// It is a zero Sent when nothing was sent, so a test that forgets to check the
// count fails on the empty subject rather than panicking.
func (o *Outbox) Last() Sent {
	all := o.Messages()
	if len(all) == 0 {
		return Sent{}
	}
	return all[len(all)-1]
}

// Reset empties the outbox, for the test that runs the same flow twice.
func (o *Outbox) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent = nil
}

var slugBad = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugBad.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "message"
	}
	return s
}
