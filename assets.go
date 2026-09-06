package trilha

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// immutableCache is what a correctly versioned asset gets: the URL changes
// when the content changes, so the browser never needs to ask again.
const immutableCache = "public, max-age=31536000, immutable"

// assetVersion is the fingerprint of one file in Config.Public, with what is
// needed to notice a change in dev without reading the file again.
type assetVersion struct {
	v    string
	mod  time.Time
	size int64
}

// Asset returns the URL of a file in Config.Public carrying a version derived
// from its content:
//
//	h.Link(h.Rel("stylesheet"), h.Href(c.Asset("/site.css")))
//	// → /site.css?v=8f3a1c92
//
// The point is the address changing when the content changes: a CDN or browser
// holding the old file is asked for a URL it has never seen, so a deploy cannot
// leave someone with new HTML and old CSS. It also makes a long
// StaticCacheControl safe.
//
// BasePath is applied, so Asset replaces Base()+path. An unknown file returns
// the path unchanged (with a warning in the log) rather than breaking the page.
func (a *App) Asset(p string) string { return a.cfg.BasePath + a.assetPath(p) }

// Asset is App.Asset for a request; see it for details.
func (c *Ctx) Asset(p string) string { return c.app.Asset(p) }

func (a *App) assetPath(p string) string {
	if a.cfg.Public == nil {
		return p
	}
	name := strings.TrimPrefix(path.Clean("/"+p), "/")
	if name == "" || !fs.ValidPath(name) {
		return p
	}
	v := a.assetVersion(name)
	if v == "" {
		a.warnAsset(p)
		return p
	}
	return p + "?v=" + v
}

// assetVersion returns the fingerprint of name, "" when there is no such file.
// In prod each file is read once; in dev a Stat decides whether to read again,
// so editing a stylesheet does not need a restart.
func (a *App) assetVersion(name string) string {
	a.assetMu.RLock()
	e, cached := a.assets[name]
	a.assetMu.RUnlock()
	if cached && a.cfg.Env != Dev {
		return e.v
	}
	fsys, file, ok := a.staticFile("/" + name)
	if !ok {
		return ""
	}
	st, err := fs.Stat(fsys, file)
	if err != nil || st.IsDir() {
		return ""
	}
	if cached && e.size == st.Size() && e.mod.Equal(st.ModTime()) {
		return e.v
	}
	f, err := fsys.Open(file)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := fnv.New64a()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	// Content identity, not a signature: 32 bits are plenty to tell two
	// versions of the same file apart, and short URLs read better.
	e = assetVersion{v: fmt.Sprintf("%08x", uint32(h.Sum64())), mod: st.ModTime(), size: st.Size()}
	a.assetMu.Lock()
	if a.assets == nil {
		a.assets = map[string]assetVersion{}
	}
	a.assets[name] = e
	a.assetMu.Unlock()
	return e.v
}

// versionMatches reports whether v is the current fingerprint of name.
func (a *App) versionMatches(name, v string) bool {
	return v != "" && v == a.assetVersion(name)
}

// warnOnce logs a warning the first time a key appears; a warning that
// repeats per request is a warning nobody reads.
func (a *App) warnOnce(key, msg string, args ...any) { a.logOnce(a.log.Warn, key, msg, args...) }

// infoOnce is warnOnce for a deliberate choice: it is recorded, not
// complained about, and the boot log is where someone goes to find it.
func (a *App) infoOnce(key, msg string, args ...any) { a.logOnce(a.log.Info, key, msg, args...) }

func (a *App) logOnce(write func(string, ...any), key, msg string, args ...any) {
	a.warnedMu.Lock()
	if a.warned == nil {
		a.warned = map[string]bool{}
	}
	seen := a.warned[key]
	a.warned[key] = true
	a.warnedMu.Unlock()
	if !seen {
		write(msg, args...)
	}
}

// warnAsset complains once per path: a typo in a layout would otherwise write
// a line per rendered page.
func (a *App) warnAsset(p string) {
	a.assetMu.Lock()
	if a.assetWarned == nil {
		a.assetWarned = map[string]bool{}
	}
	seen := a.assetWarned[p]
	a.assetWarned[p] = true
	a.assetMu.Unlock()
	if !seen {
		a.log.Warn("trilha: Asset sem arquivo correspondente em Public", "path", p)
	}
}
