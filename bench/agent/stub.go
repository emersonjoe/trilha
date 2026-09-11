package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// The service the api-call scenario talks to. It is the one thing no other
// scenario has: a second process of its own, up while the agent works and
// while the hidden test runs. A task about the path a call takes, measured
// with nothing at the other end, measures the compiler.

// Recebida is one request as the API saw it. What matters about a call is not
// visible from the side that made it: the ruler has to read the credential
// that arrived, which is why the stub records instead of only answering.
type Recebida struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Auth   string `json:"auth"`
}

// InspectPath is where the stub tells what it received. It is under a prefix
// the API's own document never mentions, so it cannot be mistaken for part of
// the surface the agent is porting against.
const InspectPath = "/__bench/received"

// acervo is the API of the openapi.json the fixture carries: one listing, one
// document, and the log of what came in.
type acervo struct {
	mu   sync.Mutex
	recs []Recebida
}

// serveAcervo brings the API up and says where it is. The stop it returns is
// the caller's to defer.
func serveAcervo() ([]string, func()) {
	a := &acervo{}
	s := httptest.NewServer(a)
	return []string{"API_URL=" + s.URL}, s.Close
}

func (a *acervo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == InspectPath {
		a.mu.Lock()
		recs := append([]Recebida(nil), a.recs...)
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(recs)
		return
	}
	a.mu.Lock()
	a.recs = append(a.recs, Recebida{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Auth: r.Header.Get("Authorization")})
	a.mu.Unlock()

	// The answer does not depend on the credential, on purpose. An API that
	// refused an anonymous call would make "went out without the session's
	// token" and "never went out" look the same on the screen — and those are
	// the two failures this scenario exists to tell apart.
	switch {
	case r.URL.Path == "/api/documents":
		itens := documentosDoAcervo
		if q := r.URL.Query().Get("q"); q != "" {
			itens = filtrar(itens, q)
		}
		writeJSON(w, map[string]any{"items": itens, "total": len(itens), "page": 1})
	case strings.HasPrefix(r.URL.Path, "/api/documents/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/documents/")
		for _, d := range documentosDoAcervo {
			if d["id"] == id {
				writeJSON(w, d)
				return
			}
		}
		http.Error(w, `{"detail":"no such document"}`, http.StatusNotFound)
	default:
		http.Error(w, `{"detail":"no such path"}`, http.StatusNotFound)
	}
}

// documentosDoAcervo is what the API has. The names are what the hidden test
// looks for on the page: rendering them is proof the call happened, because
// they exist nowhere in the project.
var documentosDoAcervo = []map[string]any{
	{"id": "d-1", "filename": "contrato-2026.pdf", "status": "processed", "pages": 12, "owner": "ana@exemplo.com"},
	{"id": "d-2", "filename": "nota-fiscal-9.pdf", "status": "pending", "pages": 1, "owner": "bia@exemplo.com"},
	{"id": "d-3", "filename": "recibo-arquivado.pdf", "status": "failed", "pages": 3, "owner": "ana@exemplo.com"},
}

func filtrar(itens []map[string]any, q string) []map[string]any {
	var out []map[string]any
	for _, d := range itens {
		if strings.Contains(strings.ToLower(d["filename"].(string)), strings.ToLower(q)) {
			out = append(out, d)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
