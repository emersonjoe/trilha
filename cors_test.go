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

// #76/#78: a política de uma rota só, declarada no route.go dela. O app inteiro
// segue de mesma origem; o documento buscado de fora responde ao preflight.
func rotaComCORS(t *testing.T) *App {
	t.Helper()
	a := New(Config{Logger: quiet()})
	a.Register(Route{
		Pattern: "/.well-known/oauth-protected-resource",
		CORS:    &CORS{Origins: []string{"*"}, Methods: []string{"GET"}, MaxAge: time.Hour},
		Methods: map[string]HandlerFunc{
			"GET": func(c *Ctx) error { return c.JSON(http.StatusOK, map[string]string{"resource": "x"}) },
		},
	})
	a.Register(Route{Pattern: "/api", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(http.StatusOK, "ok") },
	}})
	return a
}

func TestCORSDaRotaRespondePreflight(t *testing.T) {
	a := rotaComCORS(t)
	rec := do(a, "OPTIONS", "/.well-known/oauth-protected-resource", map[string]string{
		"Origin": "https://cliente.exemplo.com", "Access-Control-Request-Method": "GET",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight respondeu %d, queria 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Fatalf("Max-Age %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET" {
		t.Fatalf("Allow-Methods %q", got)
	}
}

func TestCORSDaRotaRecusaMetodoDeFora(t *testing.T) {
	a := rotaComCORS(t)
	rec := do(a, "OPTIONS", "/.well-known/oauth-protected-resource", map[string]string{
		"Origin": "https://cliente.exemplo.com", "Access-Control-Request-Method": "DELETE",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("método fora da política respondeu %d, queria 403", rec.Code)
	}
}

func TestCORSDaRotaNaRequisicaoSimples(t *testing.T) {
	a := rotaComCORS(t)
	rec := do(a, "GET", "/.well-known/oauth-protected-resource", map[string]string{
		"Origin": "https://cliente.exemplo.com",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("GET respondeu %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin %q", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Fatalf("sem Vary: Origin um cache serve a resposta de uma origem para outra: %q", rec.Header().Get("Vary"))
	}
}

// A rota vizinha não muda: é a diferença entre política por rota e Config.CORS.
func TestCORSDaRotaNaoVazaParaAsOutras(t *testing.T) {
	a := rotaComCORS(t)
	rec := do(a, "GET", "/api", map[string]string{"Origin": "https://cliente.exemplo.com"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("a rota sem política ganhou %q", got)
	}
	if rec := do(a, "OPTIONS", "/api", map[string]string{
		"Origin": "https://cliente.exemplo.com", "Access-Control-Request-Method": "GET",
	}); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("preflight numa rota sem política respondeu %d, queria 405", rec.Code)
	}
}

// Um OPTIONS sem Origin não é preflight: é a pergunta do que o caminho aceita.
func TestCORSDaRotaResponde204ComAllow(t *testing.T) {
	a := rotaComCORS(t)
	rec := do(a, "OPTIONS", "/.well-known/oauth-protected-resource", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS sem Origin respondeu %d", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET, OPTIONS" {
		t.Fatalf("Allow %q", got)
	}
}

// O caso esquisito continua possível: quem escreve o próprio OPTIONS manda.
func TestOPTIONSEscritoAMaoPrevalece(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{
		Pattern: "/api",
		CORS:    &CORS{Origins: []string{"*"}},
		Methods: map[string]HandlerFunc{
			"GET":     func(c *Ctx) error { return c.Text(http.StatusOK, "ok") },
			"OPTIONS": func(c *Ctx) error { return c.Text(http.StatusTeapot, "meu") },
		},
	})
	rec := do(a, "OPTIONS", "/api", nil)
	if rec.Code != http.StatusTeapot || rec.Body.String() != "meu" {
		t.Fatalf("o handler do arquivo respondeu %d %q", rec.Code, rec.Body.String())
	}
}

// E o preflight de verdade continua sendo do framework, mesmo assim: o handler
// escrito à mão só vê o que não é preflight.
func TestOPTIONSAMaoNaoAtrapalhaOPreflight(t *testing.T) {
	a := New(Config{Logger: quiet()})
	a.Register(Route{
		Pattern: "/api",
		CORS:    &CORS{Origins: []string{"https://app.exemplo.com"}},
		Methods: map[string]HandlerFunc{
			"GET":     func(c *Ctx) error { return c.Text(http.StatusOK, "ok") },
			"OPTIONS": func(c *Ctx) error { return c.Text(http.StatusTeapot, "meu") },
		},
	})
	rec := do(a, "OPTIONS", "/api", map[string]string{
		"Origin": "https://app.exemplo.com", "Access-Control-Request-Method": "GET",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight respondeu %d, queria 204", rec.Code)
	}
}

// A rota que declara a própria política decide sozinha: sem isso, um app que já
// tem Config.CORS nunca conseguiria abrir três caminhos, que é o caso das #76 e
// #78 — o preflight morreria no 403 do app antes de chegar à rota.
func TestCORSDaRotaVenceAPoliticaDoApp(t *testing.T) {
	a := New(Config{Logger: quiet(), CORS: CORS{Origins: []string{"https://painel.exemplo.com"}}})
	a.Register(Route{
		Pattern: "/.well-known/oauth-protected-resource",
		CORS:    &CORS{Origins: []string{"*"}, Methods: []string{"GET"}},
		Methods: map[string]HandlerFunc{
			"GET": func(c *Ctx) error { return c.JSON(http.StatusOK, "x") },
		},
	})
	a.Register(Route{Pattern: "/api", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(http.StatusOK, "ok") },
	}})
	rec := do(a, "OPTIONS", "/.well-known/oauth-protected-resource", map[string]string{
		"Origin": "https://cliente.exemplo.com", "Access-Control-Request-Method": "GET",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight respondeu %d, queria 204", rec.Code)
	}
	// E o resto do app segue com a política do app: a lista continua exata.
	if rec := do(a, "OPTIONS", "/api", map[string]string{
		"Origin": "https://cliente.exemplo.com", "Access-Control-Request-Method": "GET",
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("a política do app respondeu %d para uma origem de fora da lista", rec.Code)
	}
}
