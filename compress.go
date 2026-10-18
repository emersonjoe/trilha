package trilha

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// gzipMinSize is where gzip starts paying: below it the header and the extra
// round of CPU cost more than the bytes saved.
const gzipMinSize = 1024

// gzipAsset is the compressed copy of one file, for the version it was made
// from: a new version replaces it, so the map holds one copy per file.
type gzipAsset struct {
	v    string
	body []byte // nil when compressing did not make it smaller
}

// compressible lists the extensions worth compressing: text. Images, fonts in
// woff2 and archives are compressed already, and gzip would only add CPU.
var compressible = map[string]bool{
	".css": true, ".js": true, ".mjs": true, ".json": true, ".map": true,
	".svg": true, ".txt": true, ".html": true, ".xml": true, ".csv": true,
	".md": true, ".wasm": true, ".webmanifest": true,
}

// serveGzip answers a static file with its gzip copy when the client accepts
// it, and reports whether it did. It never compresses pages: they carry the
// CSRF token next to text the visitor controls, the shape BREACH needs, while
// a file in Public is the same bytes for everyone.
//
// The copy is made once per version of the file and kept, so the cost of
// compressing is paid by the first request after a deploy, not by each one.
// It has its own ETag ("<v>-gz"): a strong tag names one body, and the two
// encodings are two bodies (RFC 9110 §8.8.3).
func (a *App) serveGzip(w http.ResponseWriter, req *http.Request, fsys fs.FS, file, name, v string) bool {
	if !compressible[strings.ToLower(path.Ext(file))] {
		return false
	}
	w.Header().Add("Vary", "Accept-Encoding")
	if v == "" || req.Header.Get("Range") != "" || !acceptsGzip(req.Header.Get("Accept-Encoding")) {
		return false
	}
	body := a.gzipBody(fsys, file, name, v)
	if body == nil {
		return false
	}
	hd := w.Header()
	hd.Set("Content-Encoding", "gzip")
	hd.Set("ETag", `"`+v+`-gz"`)
	if hd.Get("Content-Type") == "" {
		if ct := mime.TypeByExtension(path.Ext(file)); ct != "" {
			hd.Set("Content-Type", ct)
		}
	}
	http.ServeContent(w, req, file, time.Time{}, bytes.NewReader(body))
	return true
}

// gzipBody returns the compressed copy of the file for version v, making it
// the first time. nil means "send it as it is": too small, unreadable, or not
// smaller once compressed.
func (a *App) gzipBody(fsys fs.FS, file, name, v string) []byte {
	a.assetMu.RLock()
	g, ok := a.gzipped[name]
	a.assetMu.RUnlock()
	if ok && g.v == v {
		return g.body
	}
	g = gzipAsset{v: v}
	if data, err := fs.ReadFile(fsys, file); err == nil && len(data) >= gzipMinSize {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if _, err := zw.Write(data); err == nil && zw.Close() == nil && buf.Len() < len(data) {
			g.body = buf.Bytes()
		}
	}
	a.assetMu.Lock()
	if a.gzipped == nil {
		a.gzipped = map[string]gzipAsset{}
	}
	a.gzipped[name] = g
	a.assetMu.Unlock()
	return g.body
}

// acceptsGzip reads Accept-Encoding the way RFC 9110 §12.5.3 does: gzip (or
// x-gzip, or *) with a q above zero. "gzip;q=0" is a refusal.
func acceptsGzip(header string) bool {
	star, explicit := false, false
	for _, part := range strings.Split(header, ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		coding = strings.ToLower(strings.TrimSpace(coding))
		if coding != "gzip" && coding != "x-gzip" && coding != "*" {
			continue
		}
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			k, val, _ := strings.Cut(strings.TrimSpace(p), "=")
			if strings.EqualFold(strings.TrimSpace(k), "q") {
				if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
					q = f
				}
			}
		}
		if coding == "*" {
			star = q > 0
			continue
		}
		if q <= 0 {
			return false
		}
		explicit = true
	}
	return explicit || star
}

// addVary appends a field to Vary unless it is there already. Setting the
// header would wipe what CORS or the app put there, and a shared cache would
// then serve one origin's answer to another.
func addVary(hd http.Header, field string) {
	for _, line := range hd.Values("Vary") {
		for _, f := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(f), field) {
				return
			}
		}
	}
	hd.Add("Vary", field)
}

// wantsCompression says whether a request may get a compressed answer: it
// accepts gzip, wants a body, and is not about to become another protocol.
func wantsCompression(r *http.Request) bool {
	return r.Method != http.MethodHead && r.Header.Get("Upgrade") == "" &&
		acceptsGzip(r.Header.Get("Accept-Encoding"))
}

// textTypes are the media types worth compressing besides text/*.
var textTypes = map[string]bool{
	"application/json": true, "application/problem+json": true, "application/ld+json": true,
	"application/javascript": true, "application/xml": true, "application/xhtml+xml": true,
	"application/rss+xml": true, "application/atom+xml": true, "application/manifest+json": true,
	"image/svg+xml": true,
}

func compressibleType(ct string) bool {
	base, _, _ := strings.Cut(ct, ";")
	base = strings.ToLower(strings.TrimSpace(base))
	if base == "text/event-stream" {
		return false // a stream is flushed event by event; gzip would hold it
	}
	return strings.HasPrefix(base, "text/") || textTypes[base]
}

var gzipWriters = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression); return w }}

// compressWriter gzips a dynamic response (#305). It holds the first KiB to
// decide: a body that ends before it, a status with no body, a type that is
// not text, or a body somebody already encoded (the static files' gzip copy)
// goes as it is. A Flush starts the compression right away, so a page that
// streams its slow part still arrives in parts.
type compressWriter struct {
	rw       http.ResponseWriter
	status   int
	header   bool // WriteHeader was called
	decided  bool
	gz       *gzip.Writer
	buf      []byte
	hijacked bool
}

func newCompressWriter(rw http.ResponseWriter) *compressWriter {
	return &compressWriter{rw: rw, status: http.StatusOK}
}

func (w *compressWriter) Header() http.Header { return w.rw.Header() }

func (w *compressWriter) WriteHeader(code int) {
	if w.header {
		return
	}
	w.header, w.status = true, code
	hd := w.rw.Header()
	if code < 200 || code == http.StatusNoContent || code == http.StatusNotModified ||
		code == http.StatusPartialContent || hd.Get("Content-Encoding") != "" {
		w.passThrough()
		return
	}
	if ct := hd.Get("Content-Type"); ct != "" && !compressibleType(ct) {
		w.passThrough()
		return
	}
	if n, err := strconv.Atoi(hd.Get("Content-Length")); err == nil && n < gzipMinSize {
		w.passThrough()
	}
}

func (w *compressWriter) Write(p []byte) (int, error) {
	if !w.header {
		if w.rw.Header().Get("Content-Type") == "" {
			w.rw.Header().Set("Content-Type", http.DetectContentType(p))
		}
		w.WriteHeader(http.StatusOK)
	}
	switch {
	case w.gz != nil:
		return w.gz.Write(p)
	case w.decided:
		return w.rw.Write(p)
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) >= gzipMinSize {
		if err := w.startGzip(); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// passThrough sends the held header and bytes as they are.
func (w *compressWriter) passThrough() {
	if w.decided {
		return
	}
	w.decided = true
	w.rw.WriteHeader(w.status)
	if len(w.buf) > 0 {
		_, _ = w.rw.Write(w.buf)
		w.buf = nil
	}
}

func (w *compressWriter) startGzip() error {
	w.decided = true
	hd := w.rw.Header()
	hd.Set("Content-Encoding", "gzip")
	hd.Del("Content-Length")
	addVary(hd, "Accept-Encoding")
	// The compressed body is another body: a strong tag names one (RFC 9110
	// §8.8.3), and Ctx.ETag already compares weakly.
	if tag := hd.Get("ETag"); tag != "" && !strings.HasPrefix(tag, "W/") {
		hd.Set("ETag", "W/"+tag)
	}
	w.rw.WriteHeader(w.status)
	w.gz = gzipWriters.Get().(*gzip.Writer)
	w.gz.Reset(w.rw)
	_, err := w.gz.Write(w.buf)
	w.buf = nil
	return err
}

func (w *compressWriter) Flush() {
	if !w.header {
		w.WriteHeader(http.StatusOK)
	}
	if !w.decided {
		if err := w.startGzip(); err != nil {
			return
		}
	}
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	_ = http.NewResponseController(w.rw).Flush()
}

// Hijack hands the connection over; nothing is compressed after it.
func (w *compressWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	return http.NewResponseController(w.rw).Hijack()
}

// Unwrap lets http.ResponseController reach the deadlines of the original.
func (w *compressWriter) Unwrap() http.ResponseWriter { return w.rw }

// close ends the response: a body that never reached a KiB goes as it is.
func (w *compressWriter) close() {
	switch {
	case w.hijacked:
	case w.gz != nil:
		_ = w.gz.Close()
		w.gz.Reset(io.Discard)
		gzipWriters.Put(w.gz)
		w.gz = nil
	case w.header || len(w.buf) > 0:
		w.passThrough()
	}
}
