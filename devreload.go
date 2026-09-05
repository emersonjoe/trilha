package trilha

import (
	"net/http"
	"time"
)

// devEvents is a minimal SSE endpoint so the injected dev script has
// something to connect to when the app runs without the trilha CLI. The CLI
// intercepts this path and drives reloads itself.
func (a *App) devEvents(w http.ResponseWriter, r *http.Request) {
	if a.cfg.Env != Dev {
		http.NotFound(w, r)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(": connected\n\n"))
	fl.Flush()
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-t.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
