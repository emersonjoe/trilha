package trilha

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func corsApp(t *testing.T, c CORS) *App {
	t.Helper()
	a := New(Config{Logger: quiet(), CORS: c})
	a.Register(Route{Pattern: "/api", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(http.StatusOK, "ok") },
		"PUT": func(c *Ctx) error { return c.Text(http.StatusOK, "ok") },
	}})
	return a
}

func do(a *App, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func preflight(a *App, origin, method string) *httptest.ResponseRecorder {
	return do(a, "OPTIONS", "/api", map[string]string{
		"Origin": origin, "Access-Control-Request-Method": method,
	})
}

// #29: o preflight é respondido pelo framework, com tudo que o navegador precisa.
func TestCORSPreflight(t *testing.T) {
	a := corsApp(t, CORS{Origins: []string{"https://app.exemplo.com"}, MaxAge: 10 * time.Minute})
	rec := preflight(a, "https://app.exemplo.com", "PUT")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "https://app.exemplo.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PUT") {
		t.Errorf("allow-methods = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Content-Type") {
		t.Errorf("allow-headers = %q", got)
	}
	if got := h.Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("max-age = %q, want 600", got)
	}
	if got := h.Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("vary = %q", got)
	}
}

// Origem de fora da lista e método fora da lista não passam do gancho.
func TestCORSPreflightRecusado(t *testing.T) {
	a := corsApp(t, CORS{Origins: []string{"https://app.exemplo.com"}, Methods: []string{"GET"}})
	for _, tc := range []struct{ origin, method string }{
		{"https://atacante.net", "GET"},
		{"https://app.exemplo.com", "DELETE"},
	} {
		rec := preflight(a, tc.origin, tc.method)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403", tc.origin, tc.method, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("%s %s: allow-origin = %q, want vazio", tc.origin, tc.method, got)
		}
	}
}

// Requisição simples: quem está na lista ganha o cabeçalho, quem não está é servido
// normalmente e é o navegador que esconde a resposta.
func TestCORSRequisicaoSimples(t *testing.T) {
	a := corsApp(t, CORS{
		Origins:     []string{"https://app.exemplo.com"},
		Credentials: true,
		Expose:      []string{"X-Total"},
	})
	rec := do(a, "GET", "/api", map[string]string{"Origin": "https://app.exemplo.com"})
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if got := h.Get("Access-Control-Allow-Origin"); got != "https://app.exemplo.com" {
		t.Errorf("allow-origin = %q", got)
	}
	if got := h.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("allow-credentials = %q", got)
	}
	if got := h.Get("Access-Control-Expose-Headers"); got != "X-Total" {
		t.Errorf("expose-headers = %q", got)
	}

	rec = do(a, "GET", "/api", map[string]string{"Origin": "https://atacante.net"})
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("origem de fora: status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("origem de fora ganhou allow-origin = %q", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("vary = %q, want Origin mesmo na recusa", got)
	}
}

// "*" sozinho responde "*" — é a API pública, sem cookie.
func TestCORSTodasAsOrigens(t *testing.T) {
	a := corsApp(t, CORS{Origins: []string{"*"}})
	rec := do(a, "GET", "/api", map[string]string{"Origin": "https://qualquer.net"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("allow-origin = %q, want *", got)
	}
	if rec := preflight(a, "https://qualquer.net", "PUT"); rec.Code != http.StatusNoContent {
		t.Errorf("preflight = %d, want 204", rec.Code)
	}
}

// Configuração insegura ou malformada cai no boot, não na primeira requisição de fora.
func TestCORSConfiguracaoInvalidaEntraEmPanico(t *testing.T) {
	for _, tc := range []struct {
		nome string
		cors CORS
	}{
		{"* com credencial", CORS{Origins: []string{"*"}, Credentials: true}},
		{"* misturado", CORS{Origins: []string{"*", "https://app.exemplo.com"}}},
		{"sem esquema", CORS{Origins: []string{"app.exemplo.com"}}},
		{"com caminho", CORS{Origins: []string{"https://app.exemplo.com/painel"}}},
		{"barra no fim", CORS{Origins: []string{"https://app.exemplo.com/"}}},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("esperava pânico")
				}
				if msg, _ := r.(string); !strings.Contains(msg, "CORS") {
					t.Errorf("mensagem = %v, quero o nome do campo", r)
				}
			}()
			New(Config{Logger: quiet(), CORS: tc.cors})
		})
	}
}

// Sem CORS configurado o app é o de antes: nenhum cabeçalho, nenhuma resposta nova.
func TestCORSDesligadoNaoMudaNada(t *testing.T) {
	a := corsApp(t, CORS{})
	rec := do(a, "GET", "/api", map[string]string{"Origin": "https://app.exemplo.com"})
	for _, k := range []string{"Access-Control-Allow-Origin", "Access-Control-Allow-Credentials"} {
		if got := rec.Header().Get(k); got != "" {
			t.Errorf("%s = %q, want vazio", k, got)
		}
	}
	if rec := preflight(a, "https://app.exemplo.com", "PUT"); rec.Code == http.StatusNoContent {
		t.Errorf("preflight respondido com CORS desligado")
	}
}
