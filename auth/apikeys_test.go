package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

func comChaves(t *testing.T, o KeyOptions) (*trilha.App, *Keys) {
	t.Helper()
	a := trilha.New(trilha.Config{Secret: []byte(strings.Repeat("k", 40))})
	return a, APIKeys(o)
}

// serve monta uma rota guardada pela chave e devolve o que ela respondeu.
func serve(t *testing.T, a *trilha.App, mw trilha.MiddlewareFunc, header string) *httptest.ResponseRecorder {
	t.Helper()
	a.Register(trilha.Route{
		Pattern:     "/api/coisa",
		Kind:        trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{mw},
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error { return c.Text(http.StatusOK, "ok") },
		},
	})
	req := httptest.NewRequest("GET", "/api/coisa", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

// O segredo existe uma vez. O que fica guardado é o hash — e é isso que faz um
// vazamento ser recuperável revogando, em vez de na esperança.
func TestChaveGuardaSoOHash(t *testing.T) {
	_, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}})
	k, secret, err := ks.Issue(nil, "integração", []string{"docs:read"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, "ak_"+k.Handle+"_") {
		t.Fatalf("formato do segredo: %q", secret)
	}
	if strings.Contains(string(k.Hash), secret) {
		t.Fatal("o segredo está no que ficou guardado")
	}
	guardada, _ := ks.store.Find(k.Handle)
	if len(guardada.Hash) != 32 {
		t.Fatalf("hash de %d bytes", len(guardada.Hash))
	}
}

func TestChaveAutenticaEAplicaEscopo(t *testing.T) {
	a, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read", "docs:write"}})
	_, secret, err := ks.Issue(nil, "leitura", []string{"docs:read"}, 0)
	if err != nil {
		t.Fatal(err)
	}

	if rec := serve(t, a, ks.Require("docs:read"), "Bearer "+secret); rec.Code != 200 {
		t.Fatalf("chave boa = %d", rec.Code)
	}
	// Conhecida, mas sem o escopo: 403. Ela é quem diz ser, só não pode.
	b, ks2 := comChaves(t, KeyOptions{Scopes: []string{"docs:read", "docs:write"}})
	_, s2, _ := ks2.Issue(nil, "leitura", []string{"docs:read"}, 0)
	if rec := serve(t, b, ks2.Require("docs:write"), "Bearer "+s2); rec.Code != 403 {
		t.Fatalf("sem escopo = %d", rec.Code)
	}
}

// Chave errada, chave inexistente e nenhuma chave respondem a mesma coisa: 401
// com o WWW-Authenticate, e nada que diga qual das três aconteceu.
func TestChaveInvalidaNaoContaQualErroFoi(t *testing.T) {
	_, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}})
	k, secret, _ := ks.Issue(nil, "x", []string{"docs:read"}, 0)

	casos := map[string]string{
		"sem chave":     "",
		"lixo":          "Bearer nao-e-uma-chave",
		"não existe":    "Bearer ak_zzzzzz_" + strings.Repeat("a", 32),
		"segredo certo": "Bearer ak_" + k.Handle + "_" + strings.Repeat("a", 32),
		"outro prefixo": "Bearer xx_" + k.Handle + "_" + strings.Split(secret, "_")[2],
	}
	for nome, header := range casos {
		t.Run(nome, func(t *testing.T) {
			b, ks2 := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}})
			ks2.Issue(nil, "x", []string{"docs:read"}, 0)
			rec := serve(t, b, ks2.Require("docs:read"), header)
			if rec.Code != 401 {
				t.Fatalf("%s = %d", nome, rec.Code)
			}
			if rec.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("sem WWW-Authenticate o cliente não sabe que devia mandar chave")
			}
		})
	}
}

// Revogar vale no mesmo segundo: sem cache e sem janela.
func TestChaveRevogadaNaoEntraMais(t *testing.T) {
	a, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}})
	k, secret, _ := ks.Issue(nil, "x", []string{"docs:read"}, 0)
	if rec := serve(t, a, ks.Require("docs:read"), "Bearer "+secret); rec.Code != 200 {
		t.Fatalf("antes = %d", rec.Code)
	}
	if err := ks.Revoke(nil, k.ID); err != nil {
		t.Fatal(err)
	}
	b, _ := comChaves(t, KeyOptions{})
	b.Register(trilha.Route{Pattern: "/api/coisa", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{ks.Require("docs:read")},
		Methods:     map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.Text(200, "ok") }}})
	req := httptest.NewRequest("GET", "/api/coisa", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	b.Handler().ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("depois de revogar = %d", rec.Code)
	}
}

func TestChaveVencidaNaoEntra(t *testing.T) {
	a, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}})
	_, secret, err := ks.Issue(nil, "curta", []string{"docs:read"}, time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if rec := serve(t, a, ks.Require("docs:read"), "Bearer "+secret); rec.Code != 401 {
		t.Fatalf("vencida = %d", rec.Code)
	}
}

// O limite é por chave e não por endereço: duas chaves atrás do mesmo IP têm
// orçamentos separados, e a mesma chave de dois IPs tem um só.
func TestLimiteEhPorChave(t *testing.T) {
	a, ks := comChaves(t, KeyOptions{
		Scopes: []string{"docs:read"}, RateLimit: trilha.RateLimit{RPS: 0.001, Burst: 1},
	})
	_, uma, _ := ks.Issue(nil, "uma", []string{"docs:read"}, 0)
	_, outra, _ := ks.Issue(nil, "outra", []string{"docs:read"}, 0)
	mw := ks.Require("docs:read")

	a.Register(trilha.Route{Pattern: "/api/coisa", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{mw},
		Methods:     map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.Text(200, "ok") }}})
	pede := func(secret string) int {
		req := httptest.NewRequest("GET", "/api/coisa", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		req.RemoteAddr = "10.0.0.1:1234" // o mesmo endereço em todas
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	if got := pede(uma); got != 200 {
		t.Fatalf("primeira = %d", got)
	}
	if got := pede(uma); got != 429 {
		t.Fatalf("segunda da mesma chave = %d", got)
	}
	// Mesmo IP, outra chave: orçamento próprio.
	if got := pede(outra); got != 200 {
		t.Fatalf("outra chave no mesmo IP = %d", got)
	}
}

// Registrar o uso a cada requisição transforma um GET em uma escrita por
// chamada. A pergunta que isso responde não precisa do segundo.
func TestUsoEhGravadoUmaVezPorMinuto(t *testing.T) {
	store := MemoryKeyStore()
	a, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}, Store: store})
	k, secret, _ := ks.Issue(nil, "x", []string{"docs:read"}, 0)
	mw := ks.Require("docs:read")
	a.Register(trilha.Route{Pattern: "/api/coisa", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{mw},
		Methods:     map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error { return c.Text(200, "ok") }}})
	pede := func() {
		req := httptest.NewRequest("GET", "/api/coisa", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		a.Handler().ServeHTTP(httptest.NewRecorder(), req)
	}
	pede()
	primeira, _ := store.Find(k.Handle)
	if primeira.LastUsed.IsZero() {
		t.Fatal("o primeiro uso devia ficar registrado")
	}
	pede()
	pede()
	segunda, _ := store.Find(k.Handle)
	if !segunda.LastUsed.Equal(primeira.LastUsed) {
		t.Fatal("gravou de novo dentro do mesmo minuto")
	}
}

// Escopo que a app nunca declarou é erro de programação, e explode na hora de
// montar a rota — não numa auditoria depois de um vazamento.
func TestRequireRecusaEscopoQueNaoExiste(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("devia ter explodido")
		}
	}()
	_, ks := comChaves(t, KeyOptions{Scopes: []string{"docs:read"}})
	_ = ks.Require("docs:reed")
}

// A chave é um ator como qualquer outro: a trilha diz quem foi, e diz que foi
// uma chave.
func TestChaveViraAtorNaTrilha(t *testing.T) {
	var trilhaLog []trilha.AuditRecord
	a := trilha.New(trilha.Config{Secret: []byte(strings.Repeat("k", 40)),
		Audit: trilha.AuditFunc(func(r trilha.AuditRecord) error {
			trilhaLog = append(trilhaLog, r)
			return nil
		})})
	ks := APIKeys(KeyOptions{Scopes: []string{"docs:read"}})
	k, secret, _ := ks.Issue(nil, "integração", []string{"docs:read"}, 0)

	a.Register(trilha.Route{Pattern: "/api/coisa", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{ks.Require("docs:read")},
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			c.Audit("coisa.leu", "42")
			return c.Text(200, "ok")
		}}})
	req := httptest.NewRequest("GET", "/api/coisa", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	a.Handler().ServeHTTP(httptest.NewRecorder(), req)

	if len(trilhaLog) != 1 {
		t.Fatalf("trilha: %+v", trilhaLog)
	}
	if got := trilhaLog[0].Actor; got.Via != "api_key" || got.Subject != "key:"+k.ID {
		t.Fatalf("ator = %+v", got)
	}
}
