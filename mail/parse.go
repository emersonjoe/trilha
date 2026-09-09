package mail

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

// subjectOf and partsOf read a message back apart. They exist for the two
// places that need to look at what was built — the file name in Dir, and the
// Outbox a test asserts on — and the standard library does all of the work:
// decoding this by hand is how a test ends up passing on a message no client
// can read.

func subjectOf(raw []byte) string {
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	s, err := (&mime.WordDecoder{}).DecodeHeader(m.Header.Get("Subject"))
	if err != nil {
		return m.Header.Get("Subject")
	}
	return s
}

// partsOf returns the plain and the HTML alternatives, decoded.
func partsOf(raw []byte) (text, html string) {
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return "", ""
	}
	ctype, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(ctype, "multipart/") {
		body, _ := io.ReadAll(m.Body)
		return string(body), ""
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		body, err := io.ReadAll(decoded(p, p.Header.Get("Content-Transfer-Encoding")))
		if err != nil {
			continue
		}
		switch {
		case strings.HasPrefix(p.Header.Get("Content-Type"), "text/html"):
			html = string(body)
		case strings.HasPrefix(p.Header.Get("Content-Type"), "text/plain"):
			text = string(body)
		}
	}
	return text, html
}

func decoded(r io.Reader, encoding string) io.Reader {
	if strings.EqualFold(encoding, "quoted-printable") {
		return quotedprintable.NewReader(r)
	}
	return r
}
