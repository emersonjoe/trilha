package trilha

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
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
