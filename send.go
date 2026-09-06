package trilha

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// inlineTypes is what a browser may open in place. The list is short on
// purpose: it holds what a viewer renders as content and leaves out
// everything a browser would treat as a document of its own — HTML, SVG and
// XML run script, and running script from the app's own origin is the whole
// of stored XSS.
var inlineTypes = []string{
	"application/pdf",
	"image/",
	"audio/",
	"video/",
	"text/plain",
	"text/csv",
}

// Attachment sends the body as a download: Content-Disposition: attachment
// with a safe name, the type detected from the content when ctype is empty,
// and Range when body is an io.ReadSeeker.
//
//	return c.Attachment("report-2026.csv", bytes.NewReader(csv), "")
//
// The name is sanitised like an upload's: a path never becomes a filename,
// and no part of it can add a second header line.
func (c *Ctx) Attachment(name string, body io.Reader, ctype string) error {
	return c.send("attachment", name, body, ctype, time.Time{})
}

// Inline sends the body for the browser to open in place — a PDF inside an
// <iframe>, an image, a plain-text file. Anything the browser would run
// instead of render (HTML, SVG, XML) is refused as a programming error.
//
// The iframe still needs the app to say so: the default policy is
// frame-ancestors 'none', so add "frame-src": {"'self'"} to Security.CSPExtra
// on the page that frames it. Inline does not loosen the policy from below.
func (c *Ctx) Inline(name string, body io.Reader, ctype string) error {
	return c.send("inline", name, body, ctype, time.Time{})
}

// AttachmentFile opens the file at path and sends it as a download. A file
// that is not there is a 404; a directory is an error. The file is closed
// before the call returns.
func (c *Ctx) AttachmentFile(path string, ctype string) error {
	return c.sendFile("attachment", path, ctype)
}

// InlineFile is AttachmentFile for a file the browser opens in place.
func (c *Ctx) InlineFile(path string, ctype string) error {
	return c.sendFile("inline", path, ctype)
}

func (c *Ctx) sendFile(kind, path, ctype string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &HTTPError{Code: http.StatusNotFound}
		}
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("trilha: %s is a directory, not a file", path)
	}
	return c.send(kind, st.Name(), f, ctype, st.ModTime())
}

// send is the one place that writes a file response.
func (c *Ctx) send(kind, name string, body io.Reader, ctype string, mod time.Time) error {
	name = safeName(name)
	if ctype == "" {
		var err error
		if ctype, body, err = sniffReader(body, name); err != nil {
			return err
		}
	}
	if kind == "inline" && !inlineOK(ctype) {
		return fmt.Errorf("trilha: %s may not be sent inline; use Attachment", ctype)
	}
	h := c.w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Disposition", disposition(kind, name))
	// A download that the browser is free to re-interpret is a download that
	// can become a page of this origin.
	h.Set("X-Content-Type-Options", "nosniff")
	// A big file on a bad link is not a slow handler; it is a handler doing
	// what it was told.
	_ = c.NoWriteDeadline()
	if rs, ok := body.(io.ReadSeeker); ok {
		// ServeContent owns Range, If-Range, 304 and HEAD. The name is only
		// used to guess a type, and the type is already set.
		http.ServeContent(c.w, c.r, "", mod, rs)
		return nil
	}
	if c.r.Method == http.MethodHead {
		c.w.WriteHeader(http.StatusOK)
		return nil
	}
	_, err := io.Copy(c.w, body)
	return err
}

// sniffReader reads the first bytes to find the type and gives back a reader
// that still starts at the beginning. An io.ReadSeeker rewinds; anything else
// is stitched back together with io.MultiReader.
func sniffReader(body io.Reader, name string) (string, io.Reader, error) {
	if body == nil {
		return "application/octet-stream", strings.NewReader(""), nil
	}
	buf := make([]byte, 512)
	n, err := io.ReadFull(body, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", nil, err
	}
	buf = buf[:n]
	ctype := http.DetectContentType(buf)
	// The content is the truth about what runs; the extension is the truth
	// about what the person meant to send. DetectContentType answers
	// text/plain for a CSV and octet-stream for a DOCX, and neither helps.
	if ext := extOf(name); ext != "" {
		if byExt := mime.TypeByExtension(ext); byExt != "" && refines(ctype, byExt) {
			ctype = byExt
		}
	}
	if rs, ok := body.(io.ReadSeeker); ok {
		if _, err := rs.Seek(0, io.SeekStart); err == nil {
			return ctype, rs, nil
		}
	}
	return ctype, io.MultiReader(strings.NewReader(string(buf)), body), nil
}

// refines reports whether the type the extension claims is a sharper version
// of the one the bytes gave. text/plain to text/csv is sharper; anything to a
// type of another family is a name arguing with the content, and the content
// wins.
func refines(sniffed, byExt string) bool {
	sniffed, _, _ = strings.Cut(sniffed, ";")
	byExt, _, _ = strings.Cut(byExt, ";")
	if sniffed == "application/octet-stream" {
		return true
	}
	s, _, _ := strings.Cut(sniffed, "/")
	e, _, _ := strings.Cut(byExt, "/")
	return s == e
}

func extOf(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return strings.ToLower(name[i:])
	}
	return ""
}

// inlineNever is the exception inside the families above: an SVG is an image
// to the eye and a document with script to the browser, and a family prefix
// would let it through.
var inlineNever = map[string]bool{
	"image/svg+xml":         true,
	"image/svg":             true,
	"text/html":             true,
	"application/xhtml+xml": true,
}

func inlineOK(ctype string) bool {
	kind, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		kind, _, _ = strings.Cut(ctype, ";")
		kind = strings.ToLower(strings.TrimSpace(kind))
	}
	if inlineNever[kind] {
		return false
	}
	for _, ok := range inlineTypes {
		if strings.HasSuffix(ok, "/") && strings.HasPrefix(kind, ok) {
			return true
		}
		if kind == ok {
			return true
		}
	}
	return false
}

// disposition writes the header both halves of the world understand: the
// UTF-8 name for a browser that reads RFC 5987, and an ASCII one for a
// browser that does not. Percent-encoding the first and quoting the second is
// also what keeps a name from adding a parameter of its own.
func disposition(kind, name string) string {
	ascii := strings.Map(func(r rune) rune {
		switch {
		case r < 0x20 || r == 0x7f:
			return '_'
		case r > 0x7e:
			return '_'
		case r == '"' || r == '\\':
			return '_'
		}
		return r
	}, name)
	if ascii == "" {
		ascii = "file"
	}
	d := kind + `; filename="` + ascii + `"`
	if ascii != name {
		d += "; filename*=UTF-8''" + url.PathEscape(name)
	}
	return d
}

// pipeHeaders is what may cross from another service's answer into ours. It
// is a list, not a filter: a header nobody thought about does not travel, and
// Set-Cookie in particular never does — the body of another service does not
// get to sit on this one's session.
var pipeHeaders = []string{
	"Content-Type",
	"Content-Disposition",
	"Content-Length",
	"Content-Encoding",
	"Content-Range",
	"Accept-Ranges",
	"Cache-Control",
	"ETag",
	"Last-Modified",
	"Expires",
	"Vary",
	"Age",
}

// Pipe hands the answer of another service to the browser: the status, the
// headers on the closed list above, and the body copied as a stream. The
// response body is closed.
//
//	res, err := http.DefaultClient.Do(req)
//	if err != nil { return err }
//	return c.Pipe(res)
func (c *Ctx) Pipe(res *http.Response) error {
	if res == nil {
		return errors.New("trilha: Pipe of a nil response")
	}
	defer res.Body.Close()
	dst := c.w.Header()
	for _, k := range pipeHeaders {
		for _, v := range res.Header.Values(k) {
			dst.Add(k, v)
		}
	}
	if dst.Get("X-Content-Type-Options") == "" {
		dst.Set("X-Content-Type-Options", "nosniff")
	}
	_ = c.NoWriteDeadline()
	c.w.WriteHeader(res.StatusCode)
	if c.r.Method == http.MethodHead {
		return nil
	}
	_, err := io.Copy(c.w, res.Body)
	return err
}
