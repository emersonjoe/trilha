package trilha

import (
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
)

// mount is one entry of Config.Mounts, with the prefix already normalized.
type mount struct {
	prefix string // "/icons/", always with both slashes
	fsys   fs.FS
}

// parseMounts sorts the mounts from the longest prefix to the shortest, so
// "/icons/" is tried before "/".
func (a *App) parseMounts() {
	a.mounts = a.mounts[:0]
	for prefix, fsys := range a.cfg.Mounts {
		if fsys == nil {
			continue
		}
		p := "/" + strings.Trim(prefix, "/")
		if p != "/" {
			p += "/"
		}
		a.mounts = append(a.mounts, mount{prefix: p, fsys: fsys})
	}
	sort.Slice(a.mounts, func(i, j int) bool { return len(a.mounts[i].prefix) > len(a.mounts[j].prefix) })
}

// staticFile resolves a URL path to the file system that has the file: the
// mounts first, longest prefix first, then Public. A mount that matches the
// prefix but not the file falls through, so "/" is a usable catch-all.
func (a *App) staticFile(urlPath string) (fsys fs.FS, name string, ok bool) {
	p := path.Clean("/" + urlPath)
	for _, m := range a.mounts {
		if !strings.HasPrefix(p, m.prefix) {
			continue
		}
		if n := strings.TrimPrefix(p, m.prefix); n != "" && fs.ValidPath(n) {
			if st, err := fs.Stat(m.fsys, n); err == nil && !st.IsDir() {
				return m.fsys, n, true
			}
		}
	}
	if a.cfg.Public == nil {
		return nil, "", false
	}
	n := strings.TrimPrefix(p, "/")
	if n == "" || !fs.ValidPath(n) {
		return nil, "", false
	}
	if st, err := fs.Stat(a.cfg.Public, n); err != nil || st.IsDir() {
		return nil, "", false
	}
	return a.cfg.Public, n, true
}

// serveStatic serves a file from cfg.Mounts or cfg.Public when it exists.
// Directories and invalid paths are not served. Returns false when nothing
// was written.
func (a *App) serveStatic(w http.ResponseWriter, req *http.Request) bool {
	fsys, file, ok := a.staticFile(req.URL.Path)
	if !ok {
		return false
	}
	// The name given out is the one from the URL, not the one inside the
	// mount: it is what tells two mounts apart in StaticHeaders.
	name := strings.TrimPrefix(path.Clean("/"+req.URL.Path), "/")
	switch {
	case a.cfg.Env == Dev:
		w.Header().Set("Cache-Control", "no-cache")
	case req.URL.RawQuery != "" && a.versionMatches(name, req.URL.Query().Get("v")):
		// Só a versão certa ganha o cache de um ano: um "?v=" adivinhado
		// congelaria o arquivo errado no navegador de quem clicou.
		w.Header().Set("Cache-Control", immutableCache)
	case a.cfg.StaticCacheControl != "":
		w.Header().Set("Cache-Control", a.cfg.StaticCacheControl)
	default:
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	// A mesma impressão digital que vai no "?v=" serve de ETag: dois deploys
	// do mesmo conteúdo respondem a mesma etiqueta, e http.ServeFileFS cuida
	// da comparação e do 304 a partir daqui.
	if v := a.assetVersion(name); v != "" {
		w.Header().Set("ETag", `"`+v+`"`)
	}
	if a.cfg.StaticHeaders != nil {
		a.cfg.StaticHeaders(name, w.Header())
	}
	http.ServeFileFS(w, req, fsys, file)
	return true
}
