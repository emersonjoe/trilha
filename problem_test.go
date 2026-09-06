package trilha

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func problemApp(t *testing.T, env Env) *App {
	t.Helper()
	a := New(Config{Logger: quiet(), Env: env})
	a.Register(Route{Pattern: "/api/x", Kind: KindAPI, Methods: map[string]HandlerFunc{
		"GET":    func(c *Ctx) error { return Errorf(http.StatusForbidden, "não é sua") },
		"POST":   func(c *Ctx) error { return FieldErrors{"title": "obrigatório"} },
		"DELETE": func(c *Ctx) error { return errors.New("a conexão com o banco caiu: dsn=user:senha@db") },
		"PUT": func(c *Ctx) error {
			return &Problem{
				Type:   "https://exemplo.com/probs/sem-saldo",
				Title:  "Sem saldo",
				Status: http.StatusPaymentRequired,
				Detail: "A conta tem R$ 3,00.",
				Extra:  map[string]any{"saldo": 300},
			}
		},
	}})
	return a
}

func problemBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content-type = %q", ct)
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("corpo não é JSON: %v\n%s", err, rec.Body.String())
	}
	return m
}

func req(a *App, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, r)
	return rec
}

// #30: o erro de API é um problem+json com os membros do RFC 9457.
func TestProblemCorpo(t *testing.T) {
	a := problemApp(t, Prod)
	rec := req(a, "GET", "/api/x", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rec.Code)
	}
	m := problemBody(t, rec)
	want := map[string]any{
		"type": "about:blank", "title": "Forbidden", "status": float64(403),
		"detail": "não é sua", "instance": "/api/x",
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, quero %v", k, m[k], v)
		}
	}
	if m["request_id"] == "" || m["request_id"] == nil {
		t.Error("sem request_id no corpo")
	}
}

// FieldErrors continua virando 422 com o mapa em fields.
func TestProblemMantemFields(t *testing.T) {
	rec := req(problemApp(t, Prod), "POST", "/api/x", nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
	m := problemBody(t, rec)
	f, ok := m["fields"].(map[string]any)
	if !ok || f["title"] != "obrigatório" {
		t.Fatalf("fields = %v", m["fields"])
	}
}

// ASVS V7.4.1: produção não conta o que quebrou; dev conta.
func TestProblemNaoVazaEmProducao(t *testing.T) {
	rec := req(problemApp(t, Prod), "DELETE", "/api/x", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "senha") || strings.Contains(rec.Body.String(), "banco") {
		t.Fatalf("vazou detalhe interno: %s", rec.Body.String())
	}
	m := problemBody(t, rec)
	if m["title"] != "Internal Server Error" || m["detail"] != nil {
		t.Errorf("title = %v, detail = %v", m["title"], m["detail"])
	}
	rec = req(problemApp(t, Dev), "DELETE", "/api/x", nil)
	if d, _ := problemBody(t, rec)["detail"].(string); !strings.Contains(d, "banco") {
		t.Errorf("em dev o detail = %q", d)
	}
}

// O handler devolve o problema pronto, e Extra vai achatado no objeto de cima.
func TestProblemDoHandler(t *testing.T) {
	rec := req(problemApp(t, Prod), "PUT", "/api/x", nil)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d", rec.Code)
	}
	m := problemBody(t, rec)
	if m["type"] != "https://exemplo.com/probs/sem-saldo" || m["title"] != "Sem saldo" ||
		m["detail"] != "A conta tem R$ 3,00." || m["saldo"] != float64(300) {
		t.Fatalf("corpo = %v", m)
	}
}

// A negociação é ranqueada por q, e entende tipo/* e */*.
func TestAccepts(t *testing.T) {
	offers := []string{"text/html", "application/json"}
	for _, tc := range []struct{ accept, want string }{
		{"", "text/html"},
		{"*/*", "text/html"},
		{"application/json", "application/json"},
		{"text/html;q=0.2, application/json;q=0.8", "application/json"},
		{"text/html,application/xhtml+xml,*/*;q=0.8", "text/html"},
		{"application/*", "application/json"},
		{"text/html;q=0, */*", "application/json"},
		{"text/html;q=0", ""},
		{"image/png", ""},
	} {
		c := &Ctx{r: httptest.NewRequest("GET", "/", nil)}
		if tc.accept != "" {
			c.r.Header.Set("Accept", tc.accept)
		}
		if got := c.Accepts(offers...); got != tc.want {
			t.Errorf("Accept %q: %q, quero %q", tc.accept, got, tc.want)
		}
	}
}

// O formato do erro segue o Accept, não o caminho: a mesma rota de route.go
// responde página para o navegador e problem+json para o cliente.
func TestNegociacaoNaoOlhaOCaminho(t *testing.T) {
	a := New(Config{Logger: quiet()})
	boom := func(c *Ctx) error { return Errorf(http.StatusForbidden, "não é sua") }
	a.Register(Route{Pattern: "/v2/coisas", Methods: map[string]HandlerFunc{"GET": boom}})
	a.Register(Route{Pattern: "/api/y", Methods: map[string]HandlerFunc{"GET": boom}})
	navegador := map[string]string{"Accept": "text/html,application/xhtml+xml,*/*;q=0.8"}

	if body := req(a, "GET", "/v2/coisas", navegador).Body.String(); !strings.Contains(body, "<html") {
		t.Errorf("fora de /api/ o navegador tem que ver HTML: %s", body)
	}
	if body := req(a, "GET", "/api/y", navegador).Body.String(); !strings.Contains(body, "<html") {
		t.Errorf("dentro de /api/ o navegador também: %s", body)
	}
	for _, accept := range []string{"", "*/*", "application/json"} {
		rec := req(a, "GET", "/v2/coisas", map[string]string{"Accept": accept})
		if !strings.HasPrefix(rec.Body.String(), `{"type"`) {
			t.Errorf("accept %q: %s", accept, rec.Body.String())
		}
	}
}

// 404 sem rota: o Accept decide; mudo, o prefixo /api/ decide.
func TestNegociacaoNoNotFound(t *testing.T) {
	a := New(Config{Logger: quiet()})
	for _, tc := range []struct {
		path, accept string
		problem      bool
	}{
		{"/nada", "", false},
		{"/nada", "text/html", false},
		{"/nada", "application/json", true},
		{"/api/nada", "", true},
		{"/api/nada", "text/html,*/*;q=0.8", false},
	} {
		rec := req(a, "GET", tc.path, map[string]string{"Accept": tc.accept})
		got := strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json")
		if rec.Code != http.StatusNotFound || got != tc.problem {
			t.Errorf("%s (accept %q): %d %q", tc.path, tc.accept, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}
