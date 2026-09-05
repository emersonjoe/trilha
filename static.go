package trilha

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// serveStatic serves a file from cfg.Public when it exists. Directories and
// invalid paths are not served. Returns false when nothing was written.
func (a *App) serveStatic(w http.ResponseWriter, req *http.Request) bool {
	if a.cfg.Public == nil {
		return false
	}
	p := path.Clean("/" + req.URL.Path)
	name := strings.TrimPrefix(p, "/")
	if name == "" || !fs.ValidPath(name) {
		return false
	}
	st, err := fs.Stat(a.cfg.Public, name)
	if err != nil || st.IsDir() {
		return false
	}
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
	if a.cfg.StaticHeaders != nil {
		a.cfg.StaticHeaders(name, w.Header())
	}
	http.ServeFileFS(w, req, a.cfg.Public, name)
	return true
}
