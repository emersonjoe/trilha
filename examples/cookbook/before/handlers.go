// Package before is the app of the migration guide as it was written with
// net/http alone. It compiles, so the guide can show the two sides and
// promise that both are real code.
package before

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

// Article is what the pages show.
type Article struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

var page = template.Must(template.New("article").Parse(
	`<!doctype html><html lang="en"><head><title>{{.Title}}</title></head>
<body><h1>{{.Title}}</h1></body></html>`))

// Routes is the table every net/http app grows: one mux, one line per
// address, and a handler that starts by finding out which address it is.
func Routes(find func(string) (Article, bool)) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /blog/{slug}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := find(r.PathValue("slug"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, a); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("GET /api/articles/{slug}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := find(r.PathValue("slug"))
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(a); err != nil {
			return
		}
	})
	return mux
}

// Secure is the middleware chain: the headers, the request id, the log and
// the recover that every app writes again, in the order that matters.
func Secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(r.Host, "example.com") {
			http.Error(w, "bad host", http.StatusMisdirectedRequest)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
