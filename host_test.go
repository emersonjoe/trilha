package trilha

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestHostPermitido: as regras de casamento, uma a uma, sem passar pelo HTTP.
func TestHostPermitido(t *testing.T) {
	casos := []struct {
		nome  string
		lista []string
		host  string
		env   Env
		quer  bool
	}{
		{"lista vazia aceita tudo", nil, "qualquer.example", Prod, true},
		{"lista vazia aceita host vazio", nil, "", Prod, true},
		{"exato", []string{"exemplo.com"}, "exemplo.com", Prod, true},
		{"porta não conta", []string{"exemplo.com"}, "exemplo.com:8443", Prod, true},
		{"caixa não conta", []string{"exemplo.com"}, "EXEMPLO.Com", Prod, true},
		{"ponto final do FQDN", []string{"exemplo.com"}, "exemplo.com.", Prod, true},
		{"padrão com porta e caixa", []string{"Exemplo.COM:443"}, "exemplo.com", Prod, true},
		{"segundo da lista", []string{"a.example", "b.example"}, "b.example", Prod, true},
		{"fora da lista", []string{"exemplo.com"}, "atacante.example", Prod, false},
		{"host vazio com lista", []string{"exemplo.com"}, "", Prod, false},
		{"sufixo não basta", []string{"exemplo.com"}, "mauexemplo.com", Prod, false},
		{"prefixo não basta", []string{"exemplo.com"}, "exemplo.com.atacante.example", Prod, false},
		{"curinga casa um rótulo", []string{"*.exemplo.com"}, "app.exemplo.com", Prod, true},
		{"curinga com porta", []string{"*.exemplo.com"}, "app.exemplo.com:3000", Prod, true},
		{"curinga não casa o apex", []string{"*.exemplo.com"}, "exemplo.com", Prod, false},
		{"curinga não atravessa ponto", []string{"*.exemplo.com"}, "a.b.exemplo.com", Prod, false},
		{"curinga não casa vazio", []string{"*.exemplo.com"}, ".exemplo.com", Prod, false},
		{"IPv6 entre colchetes", []string{"::1"}, "[::1]:3000", Prod, true},
		{"localhost em dev", []string{"exemplo.com"}, "localhost:3000", Dev, true},
		{"127.0.0.1 em dev", []string{"exemplo.com"}, "127.0.0.1:3000", Dev, true},
		{"[::1] em dev", []string{"exemplo.com"}, "[::1]:3000", Dev, true},
		{"localhost em produção não", []string{"exemplo.com"}, "localhost:3000", Prod, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := hostAllowed(c.lista, c.host, c.env); got != c.quer {
				t.Errorf("hostAllowed(%q, %q, %q) = %v", c.lista, c.host, c.env, got)
			}
		})
	}
}

func hostApp(t *testing.T, lista []string, eventos *[]SecurityEvent) http.Handler {
	t.Helper()
	a := New(Config{Env: Prod, Logger: quiet(), AllowedHosts: lista,
		Observability:   Observability{Metrics: "/_trilha/metrics", Trusted: []string{"0.0.0.0/0", "::/0"}},
		OnSecurityEvent: func(e SecurityEvent) { *eventos = append(*eventos, e) }})
	a.Register(Route{Pattern: "/api/x", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(http.StatusOK, "chegou") },
	}})
	return a.Handler()
}

func pedeComHost(handler http.Handler, alvo, host string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, alvo, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestHostNaBorda: o pedido com Host de fora morre antes da rota, do health e
// da métrica, e deixa rastro.
func TestHostNaBorda(t *testing.T) {
	var eventos []SecurityEvent
	handler := hostApp(t, []string{"exemplo.com", "*.exemplo.com"}, &eventos)

	for _, host := range []string{"exemplo.com", "exemplo.com:8443", "app.exemplo.com"} {
		if rec := pedeComHost(handler, "/api/x", host); rec.Code != http.StatusOK || rec.Body.String() != "chegou" {
			t.Errorf("host %q devia passar: %d %q", host, rec.Code, rec.Body.String())
		}
	}
	if len(eventos) != 0 {
		t.Errorf("host da lista não é evento de segurança: %v", eventos)
	}

	for _, alvo := range []string{"/api/x", "/_trilha/health", "/_trilha/metrics", "/nao-existe"} {
		rec := pedeComHost(handler, alvo, "atacante.example")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s com Host de fora = %d, queria 400", alvo, rec.Code)
		}
		if rec.Body.String() == "chegou" {
			t.Errorf("%s: o handler rodou mesmo com o Host recusado", alvo)
		}
	}
	if len(eventos) != 4 {
		t.Fatalf("queria um evento por recusa, veio %d: %v", len(eventos), eventos)
	}
	ev := eventos[0]
	if ev.Kind != "host" || ev.Status != http.StatusBadRequest || ev.Method != http.MethodGet || ev.Path != "/api/x" {
		t.Errorf("evento = %+v", ev)
	}
	if ev.IP == "" {
		t.Error("evento sem IP")
	}
}

// TestHostSemLista: sem AllowedHosts nada muda — é o app de hoje.
func TestHostSemLista(t *testing.T) {
	var eventos []SecurityEvent
	handler := hostApp(t, nil, &eventos)
	if rec := pedeComHost(handler, "/api/x", "atacante.example"); rec.Code != http.StatusOK {
		t.Errorf("sem lista o Host não é conferido: %d", rec.Code)
	}
	if len(eventos) != 0 {
		t.Errorf("eventos = %v", eventos)
	}
}

func TestAllowedHostsDoAmbiente(t *testing.T) {
	t.Setenv("TRILHA_ALLOWED_HOSTS", "exemplo.com, *.exemplo.com ")
	cfg := ConfigFromEnv()
	if len(cfg.AllowedHosts) != 2 || cfg.AllowedHosts[0] != "exemplo.com" || cfg.AllowedHosts[1] != "*.exemplo.com" {
		t.Fatalf("AllowedHosts = %q", cfg.AllowedHosts)
	}
	os.Unsetenv("TRILHA_ALLOWED_HOSTS")
	if cfg := ConfigFromEnv(); cfg.AllowedHosts != nil {
		t.Fatalf("sem a variável a lista fica vazia: %q", cfg.AllowedHosts)
	}
}
