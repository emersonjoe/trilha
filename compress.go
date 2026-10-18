package trilha

import (
	"bufio"
	"bytes"
	"compress/gzip"
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

// Compression tunes the gzip the runtime applies to text responses: HTML,
// fragments, CSS, JavaScript, JSON, SVG. The zero value compresses, because a
// framework that serves 150 KB of kit and SSR uncompressed makes every app
// either configure its proxy or ship the regression (#305).
//
// What is never compressed: text/event-stream (the gzip buffer would hold the
// events back), a response that already carries Content-Encoding, types that
// are already compressed (images other than SVG, PDF, archives, media), a
// HEAD, 1xx/204/206/304, a request with Range, and a body under MinSize.
type Compression struct {
	// Disabled turns compression off — for an app behind a proxy that
	// already compresses and should keep doing it alone.
	Disabled bool
	// MinSize is the smallest body worth compressing (default 1024 bytes).
	// Under it, the gzip header costs more than it saves.
	MinSize int
}

const defaultCompressMin = 1024

func (c Compression) minSize() int {
	if c.MinSize > 0 {
		return c.MinSize
	}
	return defaultCompressMin
}

// acceptsGzip reads Accept-Encoding: gzip, or *, with a q other than 0.
func acceptsGzip(r *http.Request) bool {
	star := false
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "gzip" && name != "*" {
			continue
		}
		q := 1.0
		for _, p := range strings.Split(params, ";") {
			k, v, ok := strings.Cut(strings.TrimSpace(p), "=")
			if ok && strings.EqualFold(strings.TrimSpace(k), "q") {
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					q = f
				}
			}
		}
		if name == "gzip" {
			return q > 0 // an explicit gzip;q=0 overrules the star
		}
		star = q > 0
	}
	return star
}

// compressible says whether a Content-Type is text that gzip shrinks.
func compressible(ct string) bool {
	mt, _, _ := strings.Cut(ct, ";")
	mt = strings.ToLower(strings.TrimSpace(mt))
	switch {
	case mt == "":
		return false
	case mt == "text/event-stream":
		return false
	case strings.HasPrefix(mt, "text/"):
		return true
	case mt == "image/svg+xml", mt == "application/wasm",
		mt == "application/javascript", mt == "application/ecmascript",
		mt == "application/json", mt == "application/xml":
		return true
	case strings.HasPrefix(mt, "application/") &&
		(strings.HasSuffix(mt, "+json") || strings.HasSuffix(mt, "+xml")):
		return true
	}
	return false
}

// weakETag turns a strong validator weak: the compressed body is not the same
// bytes the strong tag promised (RFC 9110 §8.8.3), and If-None-Match compares
// weakly, so the 304 keeps working.
func weakETag(h http.Header) {
	if e := h.Get("ETag"); e != "" && !strings.HasPrefix(e, "W/") {
		h.Set("ETag", "W/"+e)
	}
}

func markGzip(h http.Header) {
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	addVary(h, "Accept-Encoding")
	weakETag(h)
}

var gzipPool = sync.Pool{New: func() any {
	w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
	return w
}}

// gzipWriter decides at the first byte — or at Flush, or at the end — whether
// the response is compressed. Until then it holds up to MinSize bytes, so a
// short answer goes out as it is.
type gzipWriter struct {
	http.ResponseWriter
	req      *http.Request
	min      int
	status   int
	decided  bool
	on       bool
	hijacked bool
	buf      []byte
	gz       *gzip.Writer
}

func newGzipWriter(w http.ResponseWriter, r *http.Request, min int) *gzipWriter {
	return &gzipWriter{ResponseWriter: w, req: r, min: min}
}

func (w *gzipWriter) WriteHeader(code int) {
	if w.decided || w.hijacked {
		return // the header is already out, or the connection is not ours
	}
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(code) // 103 Early Hints and friends pass through
		return
	}
	w.status = code
	if !bodyAllowed(code) || w.req.Method == http.MethodHead {
		w.decide(nil, true)
	}
}

func bodyAllowed(code int) bool {
	return code != http.StatusNoContent && code != http.StatusNotModified &&
		code != http.StatusSwitchingProtocols
}

// decide settles compression and sends the header. final means the whole body
// is in buf, so its size can decide.
func (w *gzipWriter) decide(first []byte, final bool) {
	w.decided = true
	if w.status == 0 {
		w.status = http.StatusOK
	}
	h := w.Header()
	if ct := h.Get("Content-Type"); ct == "" && len(first) > 0 && h.Get("Content-Encoding") == "" {
		// The same sniff net/http would do on the first write, done here so the
		// decision sees it.
		h.Set("Content-Type", http.DetectContentType(first))
	}
	switch {
	case w.req.Method == http.MethodHead, !bodyAllowed(w.status),
		w.status == http.StatusPartialContent, w.req.Header.Get("Range") != "",
		h.Get("Content-Encoding") != "", h.Get("Content-Range") != "",
		!compressible(h.Get("Content-Type")):
	case final && len(first) < w.min:
	default:
		if cl, err := strconv.Atoi(h.Get("Content-Length")); err == nil && cl < w.min {
			break
		}
		w.on = true
		markGzip(h)
		w.gz = gzipPool.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(w.status)
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if w.hijacked {
		return len(b), nil
	}
	if !w.decided {
		w.buf = append(w.buf, b...)
		if len(w.buf) < w.min {
			return len(b), nil
		}
		if err := w.release(false); err != nil {
			return 0, err
		}
		return len(b), nil
	}
	if w.on {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// release decides with what is held and writes it.
func (w *gzipWriter) release(final bool) error {
	held := w.buf
	w.buf = nil
	w.decide(held, final)
	if len(held) == 0 {
		return nil
	}
	if w.on {
		_, err := w.gz.Write(held)
		return err
	}
	_, err := w.ResponseWriter.Write(held)
	return err
}

// Flush sends what is held: a stream (ui.Defer, SSE) that flushes before
// MinSize is decided on its type, not on a size it has not reached yet.
func (w *gzipWriter) Flush() {
	if w.hijacked {
		return
	}
	if !w.decided {
		_ = w.release(false)
	}
	if w.on {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// close ends the response: a body that never reached MinSize goes out as it is.
func (w *gzipWriter) close() {
	if w.hijacked {
		return
	}
	if !w.decided {
		if w.status == 0 && len(w.buf) == 0 {
			// The handler wrote nothing: net/http answers 200 with no body,
			// and so do we, without deciding anything.
			return
		}
		_ = w.release(true)
	}
	if w.on {
		_ = w.gz.Close()
		w.gz.Reset(nil)
		gzipPool.Put(w.gz)
		w.gz = nil
	}
}

func (w *gzipWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.hijacked = true
	}
	return conn, rw, err
}

// Unwrap lets http.ResponseController reach deadlines on the original writer.
func (w *gzipWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ---- static files: compressed once per version ----------------------------

// A versioned static file is the same bytes until the deploy changes its
// hash, so it is compressed once and served from memory. The cap keeps a
// public/ full of large text files from becoming a heap problem; past it the
// file is compressed per request by the writer above, which is correct, only
// slower.
const (
	staticGzipMaxFile  = 4 << 20
	staticGzipMaxTotal = 64 << 20
)

type staticGzip struct {
	mu    sync.Mutex
	files map[string][]byte
	total int
}

func (s *staticGzip) get(key string, load func() ([]byte, bool)) ([]byte, bool) {
	s.mu.Lock()
	if b, ok := s.files[key]; ok {
		s.mu.Unlock()
		return b, true
	}
	s.mu.Unlock()
	b, ok := load()
	if !ok {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	if s.total+len(b) <= staticGzipMaxTotal {
		s.files[key] = b
		s.total += len(b)
	}
	return b, true
}

// serveStaticGzip answers a versioned, compressible file from the cache.
// Returns false when the file is not a candidate and should be served plain.
func (a *App) serveStaticGzip(w http.ResponseWriter, req *http.Request, fsys fs.FS, file, name, version string) bool {
	if a.cfg.Compression.Disabled || version == "" || !acceptsGzip(req) ||
		req.Header.Get("Range") != "" ||
		(req.Method != http.MethodGet && req.Method != http.MethodHead) {
		return false
	}
	ct := mime.TypeByExtension(path.Ext(file))
	if !compressible(ct) {
		return false
	}
	gz, ok := a.staticGzip.get(name+"\x00"+version, func() ([]byte, bool) {
		raw, err := fs.ReadFile(fsys, file)
		if err != nil || len(raw) < a.cfg.Compression.minSize() || len(raw) > staticGzipMaxFile {
			return nil, false
		}
		var out bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&out, gzip.BestCompression)
		_, _ = zw.Write(raw)
		_ = zw.Close()
		return out.Bytes(), true
	})
	if !ok {
		return false
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	markGzip(h)
	http.ServeContent(w, req, file, time.Time{}, bytes.NewReader(gz))
	return true
}
