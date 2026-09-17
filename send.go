package trilha

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
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
	return c.send(name, body, ctype, SendOpts{}, time.Time{})
}

// Inline sends the body for the browser to open in place — a PDF inside an
// <iframe>, an image, a plain-text file. Anything the browser would run
// instead of render (HTML, SVG, XML) is refused.
//
// This response says it may be framed by a page of this same origin: the
// default hardening sends X-Frame-Options: DENY and frame-ancestors 'none' on
// everything, and a document that carries those cannot be shown in place —
// which is the one thing Inline exists to do. The relaxation is on this
// response and on this origin only; every other response keeps DENY, and an
// app that set Security.CSP or Security.Delegated itself is left alone.
//
// It is the framed answer that decides this, not the page around it: a page
// adding frame-src to its own policy changes nothing while the document it
// frames still refuses to be framed.
//
// The refusal comes back wrapped in ErrCannotInline, so a screen can tell "this
// content does not show" from "this code is wrong" and draw its own fallback;
// Send with SendOpts.NeutralizeScript is the way to send it anyway.
func (c *Ctx) Inline(name string, body io.Reader, ctype string) error {
	return c.send(name, body, ctype, SendOpts{Inline: true}, time.Time{})
}

// ErrCannotInline is what Inline answers for a media type a browser would run
// or would not show — an SVG, an HTML page, a zip. It is a value and not a
// plain error so that the screen can tell a content it cannot show from a bug
// in its own code:
//
//	if err := c.Inline(up.Name, body, up.MIME); errors.Is(err, trilha.ErrCannotInline) {
//		return c.Render(http.StatusOK, noPreview(up))
//	}
//
// trilha.CanInline answers the same question before the call.
var ErrCannotInline = errors.New("trilha: media type may not be sent inline")

// SendOpts is what the three-argument Inline and Attachment cannot say: how
// long a body that cannot seek is, that the body already is the slice the
// browser asked for, and that the caller means to send a document with script
// with the script turned off.
type SendOpts struct {
	// Inline opens the body in place instead of downloading it — the
	// difference between Inline and Attachment, and the same two answers.
	Inline bool

	// Size is the whole length of a body that cannot seek: the Content-Length
	// another service answered. With it the response promises Accept-Ranges
	// and a Range is answered with 206 and Content-Range, by dropping the
	// bytes before the first one and stopping at the last — the prefix is
	// discarded as it arrives, never held in memory.
	//
	// Without it a body that cannot seek is sent whole, with 200: the kit
	// cannot invent the total of a Content-Range. It is ignored for a body
	// that can seek, which http.ServeContent already answers, and it has to be
	// the real length — it goes out as Content-Length.
	Size int64

	// ContentRange says the body already is the part the browser asked for:
	// the answer of another service that was handed the Range header. The
	// value is that answer's Content-Range ("bytes 10-19/4096"); the status
	// becomes 206, Content-Length is computed from the range and nothing is
	// skipped. It is the cheap half of Range — the prefix never travels.
	//
	// A value that is not "bytes first-last/total" is a programming error, not
	// a crooked response. Setting it together with Size is an error too: they
	// are two different claims about the same body.
	ContentRange string

	// NeutralizeScript sends inline a type Inline refuses — image/svg+xml,
	// text/html, application/xhtml+xml — under a policy that turns script off:
	//
	//	Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'self'
	//
	// No script, no fetch of anything, inline style (without it an SVG stops
	// being a drawing) and framed by this same origin. The policy is the kit's
	// and goes out for every type sent with the option, over an app's own
	// Security.CSP: here it is not hardening, it is the condition for the
	// document to go out at all.
	//
	// It opens the script door and no other: what a viewer shows is a
	// different list, so an application/zip inline stays refused.
	NeutralizeScript bool
}

// neutralizedCSP is the policy a document with script goes out under. It is
// one string in the kit and not one per app: whoever is porting a screen
// should not have to work out which policy is the right one, and three apps
// writing their own would write three, one of them wrong.
const neutralizedCSP = "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'self'"

// Send is Inline and Attachment with the answers the three-argument forms
// cannot give: a Range over a body that cannot seek, and a document with
// script served with the script turned off. The envelope is the same one —
// sanitised name, nosniff, framing relaxed on this response only.
//
//	// the audio of another service, seekable on an iPhone
//	res, err := http.DefaultClient.Do(req)
//	if err != nil {
//		return err
//	}
//	defer res.Body.Close()
//	return c.Send(name, res.Body, res.Header.Get("Content-Type"), trilha.SendOpts{
//		Inline: true,
//		Size:   res.ContentLength,
//	})
//
// The body is not closed here: the caller opened it and knows when it is over.
func (c *Ctx) Send(name string, body io.Reader, ctype string, o SendOpts) error {
	return c.send(name, body, ctype, o, time.Time{})
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
	return c.send(st.Name(), f, ctype, SendOpts{Inline: kind == "inline"}, st.ModTime())
}

// send is the one place that writes a file response.
func (c *Ctx) send(name string, body io.Reader, ctype string, o SendOpts, mod time.Time) error {
	kind := "attachment"
	if o.Inline {
		kind = "inline"
	}
	if o.Size < 0 {
		return fmt.Errorf("trilha: SendOpts.Size is %d; a body has no negative length", o.Size)
	}
	if o.Size > 0 && o.ContentRange != "" {
		return errors.New("trilha: SendOpts.Size and SendOpts.ContentRange say two different things about the same body; set one")
	}
	part, err := partOf(o.ContentRange)
	if err != nil {
		return err
	}
	name = safeName(name)
	if ctype == "" {
		if ctype, body, err = sniffReader(body, name); err != nil {
			return err
		}
	}
	if o.Inline && !inlineOK(ctype) && !(o.NeutralizeScript && scriptRuns(ctype)) {
		return fmt.Errorf("%w: %s; send it as an attachment, or with SendOpts{NeutralizeScript: true} to serve it with script turned off", ErrCannotInline, ctype)
	}
	h := c.w.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Disposition", disposition(kind, name))
	if o.Inline {
		// Written before the relaxation below, which then only has the
		// X-Frame-Options of this response left to adjust: the policy already
		// says who may frame it.
		if o.NeutralizeScript {
			h.Set("Content-Security-Policy", neutralizedCSP)
		}
		c.allowSameOriginFrame()
	}
	// A download that the browser is free to re-interpret is a download that
	// can become a page of this origin.
	h.Set("X-Content-Type-Options", "nosniff")
	// A big file on a bad link is not a slow handler; it is a handler doing
	// what it was told.
	_ = c.NoWriteDeadline()
	if part != nil {
		// The body already is the slice; nothing to cut, nothing to skip.
		h.Set("Accept-Ranges", "bytes")
		h.Set("Content-Range", o.ContentRange)
		h.Set("Content-Length", strconv.FormatInt(part.last-part.first+1, 10))
		c.w.WriteHeader(http.StatusPartialContent)
		if c.r.Method == http.MethodHead {
			return nil
		}
		_, err := io.Copy(c.w, body)
		return err
	}
	if rs, ok := body.(io.ReadSeeker); ok {
		// ServeContent owns Range, If-Range, 304 and HEAD. The name is only
		// used to guess a type, and the type is already set.
		http.ServeContent(c.w, c.r, "", mod, rs)
		return nil
	}
	if o.Size > 0 {
		return c.sendSized(body, o.Size)
	}
	if c.r.Method == http.MethodHead {
		c.w.WriteHeader(http.StatusOK)
		return nil
	}
	_, err = io.Copy(c.w, body)
	return err
}

// sendSized writes a body that cannot seek but whose length is known: the
// whole of it, or the one slice the browser asked for, cut out of the stream
// as it arrives.
func (c *Ctx) sendSized(body io.Reader, size int64) error {
	h := c.w.Header()
	h.Set("Accept-Ranges", "bytes")
	first, last, state := sliceOf(c.r.Header.Get("Range"), size)
	switch state {
	case rangeNone:
		h.Set("Content-Length", strconv.FormatInt(size, 10))
		if c.r.Method == http.MethodHead {
			c.w.WriteHeader(http.StatusOK)
			return nil
		}
		_, err := io.Copy(c.w, body)
		return err
	case rangeUnsatisfiable:
		// No file is being delivered, so the envelope of one does not go out
		// with it.
		h.Del("Content-Disposition")
		h.Set("Content-Type", "text/plain; charset=utf-8")
		h.Set("Content-Range", "bytes */"+strconv.FormatInt(size, 10))
		c.w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		if c.r.Method == http.MethodHead {
			return nil
		}
		_, err := io.WriteString(c.w, "requested range not satisfiable\n")
		return err
	}
	n := last - first + 1
	h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", first, last, size))
	h.Set("Content-Length", strconv.FormatInt(n, 10))
	c.w.WriteHeader(http.StatusPartialContent)
	if c.r.Method == http.MethodHead {
		return nil
	}
	// The prefix is thrown away as it arrives. Reading it into memory to get
	// an io.ReadSeeker would cost the whole file on every request for 64 KB of
	// the middle, which is the trade this exists to avoid.
	if _, err := io.CopyN(io.Discard, body, first); err != nil {
		return err
	}
	_, err := io.Copy(c.w, io.LimitReader(body, n))
	return err
}

type rangeState int

const (
	rangeNone rangeState = iota
	rangeOne
	rangeUnsatisfiable
)

// sliceOf reads the Range header of a request against a known size. A header
// that does not parse is unsatisfiable and not ignored, because
// http.ServeContent — the other half of this same method, the one that answers
// when the body can seek — answers 416 to it: one method giving two different
// answers depending on the type of the body would be worse than following the
// neighbour inside the house. More than one range is answered whole.
func sliceOf(hdr string, size int64) (first, last int64, state rangeState) {
	if hdr == "" {
		return 0, 0, rangeNone
	}
	spec, ok := strings.CutPrefix(textproto.TrimString(hdr), "bytes=")
	if !ok {
		return 0, 0, rangeUnsatisfiable
	}
	if strings.Contains(spec, ",") {
		return 0, 0, rangeNone
	}
	start, end, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, rangeUnsatisfiable
	}
	start, end = textproto.TrimString(start), textproto.TrimString(end)
	switch {
	case start == "":
		// bytes=-500: the last 500 bytes.
		n, err := strconv.ParseInt(end, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, rangeUnsatisfiable
		}
		if n > size {
			n = size
		}
		first, last = size-n, size-1
	default:
		f, err := strconv.ParseInt(start, 10, 64)
		if err != nil || f < 0 || f >= size {
			return 0, 0, rangeUnsatisfiable
		}
		first, last = f, size-1
		if end != "" {
			l, err := strconv.ParseInt(end, 10, 64)
			if err != nil || l < f {
				return 0, 0, rangeUnsatisfiable
			}
			if l < last {
				last = l
			}
		}
	}
	return first, last, rangeOne
}

// part is a Content-Range the caller handed in, already read.
type part struct{ first, last int64 }

// partOf reads the Content-Range of another service's 206. It is checked
// before it becomes a header of ours: a value assembled wrong is a corrupt
// file in the browser, and a header built out of a third party's answer is how
// a second header line gets in.
func partOf(cr string) (*part, error) {
	if cr == "" {
		return nil, nil
	}
	bad := fmt.Errorf("trilha: SendOpts.ContentRange %q is not \"bytes first-last/total\"", cr)
	spec, ok := strings.CutPrefix(cr, "bytes ")
	if !ok {
		return nil, bad
	}
	rng, total, ok := strings.Cut(spec, "/")
	if !ok || total == "" {
		return nil, bad
	}
	if total != "*" {
		if n, err := strconv.ParseInt(total, 10, 64); err != nil || n <= 0 {
			return nil, bad
		}
	}
	start, end, ok := strings.Cut(rng, "-")
	if !ok {
		return nil, bad
	}
	first, err := strconv.ParseInt(start, 10, 64)
	if err != nil || first < 0 {
		return nil, bad
	}
	last, err := strconv.ParseInt(end, 10, 64)
	if err != nil || last < first {
		return nil, bad
	}
	return &part{first: first, last: last}, nil
}

// scriptRuns reports whether this is one of the types Inline refuses because
// the browser runs it — the ones NeutralizeScript is about.
func scriptRuns(ctype string) bool { return inlineNever[mediaKind(ctype)] }

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

// CanInline reports whether Inline would accept this media type — whether a
// browser shows it in place instead of running it. It is exported so a screen
// can offer the "view" link only where there is something to view, without
// keeping a second copy of the list that would drift from this one.
//
//	if trilha.CanInline(up.MIME) { … }
func CanInline(ctype string) bool { return inlineOK(ctype) }

func inlineOK(ctype string) bool {
	kind := mediaKind(ctype)
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

// mediaKind is the media type without its parameters, lowercased: the name
// the two lists above are written in.
func mediaKind(ctype string) string {
	kind, _, err := mime.ParseMediaType(ctype)
	if err != nil {
		kind, _, _ = strings.Cut(ctype, ";")
		kind = strings.ToLower(strings.TrimSpace(kind))
	}
	return kind
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
// get to sit on this one's session. Content-Security-Policy is left out on
// purpose: a policy of another service ruling over this one's page is a
// decision applySecurity makes, not one an upstream gets to make (#251).
// Config.PipeHeaders adds names; pipeNever is what no list lets through.
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
	"X-Robots-Tag",
}

// pipeNever are the headers no Config.PipeHeaders lets through: they carry
// state or a policy of the other service, and listing them is a mistake this
// list keeps quiet rather than honours.
var pipeNever = map[string]bool{"Set-Cookie": true, "Set-Cookie2": true, "Content-Security-Policy": true, "Content-Security-Policy-Report-Only": true, "Strict-Transport-Security": true}

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
	copyHeader := func(k string) {
		if pipeNever[http.CanonicalHeaderKey(k)] {
			return
		}
		for _, v := range res.Header.Values(k) {
			dst.Add(k, v)
		}
	}
	for _, k := range pipeHeaders {
		copyHeader(k)
	}
	if c.app != nil {
		for _, k := range c.app.cfg.PipeHeaders {
			copyHeader(k)
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
