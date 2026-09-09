package mail

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// now is a hook: a test that asserts on the Date header needs the date to hold
// still.
var now = time.Now

// build renders the message and returns it with the envelope recipients.
//
// The envelope is not the headers. Bcc is a recipient of the envelope and of
// no header — writing it into one is the mistake that shows every hidden
// address to everybody, and it is one line away from happening.
func build(from string, msg Message) (raw []byte, envelope []string, err error) {
	to, err := clean(msg.To)
	if err != nil {
		return nil, nil, err
	}
	cc, err := clean(msg.Cc)
	if err != nil {
		return nil, nil, err
	}
	bcc, err := clean(msg.Bcc)
	if err != nil {
		return nil, nil, err
	}
	envelope = append(append(append([]string{}, addresses(to)...), addresses(cc)...), addresses(bcc)...)

	html := ""
	if msg.Body != nil {
		if html, err = h.Render(msg.Body); err != nil {
			return nil, nil, fmt.Errorf("mail: rendering the body: %w", err)
		}
	}
	text := msg.Text
	if text == "" && msg.Body != nil {
		text = PlainText(msg.Body)
	}
	if text == "" && html == "" {
		return nil, nil, errors.New("mail: empty message (no Body, no Text)")
	}

	var b bytes.Buffer
	header := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%s: %s\r\n", name, value)
		}
	}
	header("From", from)
	header("To", strings.Join(to, ", "))
	header("Cc", strings.Join(cc, ", "))
	header("Reply-To", msg.ReplyTo)
	header("Subject", encodeHeader(msg.Subject))
	header("Date", now().Format(time.RFC1123Z))
	header("Message-ID", messageID(from))
	// Extra headers come after ours and cannot replace them: a Bcc smuggled in
	// here, or a second From, is a message some servers reject and others
	// deliver to the wrong person.
	for _, name := range sortedKeys(msg.Headers) {
		if reserved(name) {
			continue
		}
		header(name, encodeHeader(msg.Headers[name]))
	}
	header("MIME-Version", "1.0")

	// Always multipart/alternative, even when the caller wrote the text: a
	// message with only HTML is a message a text client shows empty, and one
	// more point of spam score for nothing.
	mp := multipart.NewWriter(&b)
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", mp.Boundary())
	if err := part(mp, "text/plain; charset=utf-8", text); err != nil {
		return nil, nil, err
	}
	if html != "" {
		if err := part(mp, "text/html; charset=utf-8", html); err != nil {
			return nil, nil, err
		}
	}
	if err := mp.Close(); err != nil {
		return nil, nil, err
	}
	return b.Bytes(), envelope, nil
}

// part writes one alternative, quoted-printable. The encoding is not a nicety:
// a line of generated HTML goes past the 998 octets SMTP accepts, and a server
// that wraps it for you wraps it in the middle of a URL.
func part(mp *multipart.Writer, ctype, body string) error {
	w, err := mp.CreatePart(map[string][]string{
		"Content-Type":              {ctype},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(body)); err != nil {
		return err
	}
	return qp.Close()
}

// clean parses every address and hands back the header forms. An address the
// standard library refuses is an error here and not at the server: the error
// with the bad address in it is worth more than a 501 three hops away.
func clean(list []string) ([]string, error) {
	var out []string
	for _, raw := range list {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		a, err := mail.ParseAddress(raw)
		if err != nil {
			return nil, fmt.Errorf("mail: address %q: %w", raw, err)
		}
		out = append(out, a.String())
	}
	return out, nil
}

// address is the bare address of a header form, which is what the envelope
// wants: "Acervo <no-reply@org.br>" travels as no-reply@org.br.
func address(s string) string {
	if a, err := mail.ParseAddress(strings.TrimSpace(s)); err == nil {
		return a.Address
	}
	return strings.TrimSpace(s)
}

func addresses(list []string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, address(s))
	}
	return out
}

// encodeHeader writes a header value that survives the wire. A subject with an
// accent is not ASCII, and a raw one arrives as "VocÃª foi convidado".
func encodeHeader(s string) string {
	return mime.QEncoding.Encode("utf-8", s)
}

func messageID(from string) string {
	var b [16]byte
	rand.Read(b[:])
	domain := "localhost"
	if _, d, ok := strings.Cut(address(from), "@"); ok && d != "" {
		domain = d
	}
	return "<" + hex.EncodeToString(b[:]) + "@" + domain + ">"
}

// reserved lists the headers this package writes. Letting Headers replace them
// is how a message ends up with two From lines.
func reserved(name string) bool {
	switch strings.ToLower(name) {
	case "from", "to", "cc", "bcc", "subject", "date", "message-id",
		"mime-version", "content-type", "content-transfer-encoding":
		return true
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// A deterministic message is a message a test can compare.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
