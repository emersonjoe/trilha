package auth

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// appComUso monta um app com uma rota atrás da chave e devolve o segredo.
func appComUso(t *testing.T, ks *Keys) (*trilha.App, string) {
	t.Helper()
	a := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	exige := ks.Require()
	a.Register(trilha.Route{
		Pattern: "/api/documentos/{id}", Kind: trilha.KindAPI,
		Middlewares: []trilha.MiddlewareFunc{exige},
		Methods: map[string]trilha.HandlerFunc{
			"GET": func(c *trilha.Ctx) error {
				if c.Param("id") == "ruim" {
					return &trilha.HTTPError{Code: http.StatusBadRequest, Message: "não"}
				}
				return c.Text(http.StatusOK, "ok")
			},
		}})
	_, secret, err := ks.Issue(nil, "parceiro", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return a, secret
}

// #151 — mil requisições concorrentes contam mil, e nenhuma delas esperou o
// store: o que a requisição paga é um incremento em memória.
func TestUsoContaSemPerderEsemEsperar(t *testing.T) {
	lento := &storeLento{UsageStore: UsageMemory()}
	ks := APIKeys(KeyOptions{Usage: lento})
	a, secret := appComUso(t, ks)
	h := a.Handler()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", "/api/documentos/7", nil)
			req.Header.Set("Authorization", "Bearer "+secret)
			h.ServeHTTP(descartar{}, req)
		}()
	}
	wg.Wait()
	// Nada foi para o store ainda: o incremento é em memória, e o store lento
	// não apareceu no caminho de nenhuma das mil.
	if n := lento.chamadas(); n != 0 {
		t.Fatalf("o store foi chamado %d vezes durante as requisições", n)
	}
	if err := ks.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	rel, err := ks.Usage(context.Background(), UsageQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if rel.Total != 1000 {
		t.Fatalf("contou %d", rel.Total)
	}
}

// O relatório separa por rota e por dia, e a rota é o padrão e não o caminho:
// um contador por caminho concreto é um contador com uma linha por requisição.
func TestUsoPorRotaEPorDia(t *testing.T) {
	ks := APIKeys(KeyOptions{Usage: UsageMemory()})
	a, secret := appComUso(t, ks)
	h := a.Handler()
	get := func(caminho string) {
		req, _ := http.NewRequest("GET", caminho, nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		h.ServeHTTP(descartar{}, req)
	}
	get("/api/documentos/7")
	get("/api/documentos/8f2c")
	get("/api/documentos/ruim")
	if err := ks.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}

	rel, _ := ks.Usage(context.Background(), UsageQuery{})
	if rel.Total != 3 || rel.Errors != 1 {
		t.Fatalf("relatório = %+v", rel)
	}
	if len(rel.ByRoute) != 1 {
		t.Fatalf("uma linha por caminho concreto: %+v", rel.ByRoute)
	}
	r := rel.ByRoute[0]
	if r.Route != "/api/documentos/{id}" || r.Method != "GET" || r.Count != 3 || r.Errors != 1 {
		t.Fatalf("rota = %+v", r)
	}
	if len(rel.ByDay) != 1 || rel.ByDay[0].Count != 3 {
		t.Fatalf("por dia = %+v", rel.ByDay)
	}
	if rel.Last.IsZero() {
		t.Fatal("o último uso não foi registrado")
	}
}

// Shutdown grava o que estava em memória: contagem que só existe no processo é
// contagem que some no deploy.
func TestUsoGravaNoShutdown(t *testing.T) {
	ks := APIKeys(KeyOptions{Usage: UsageMemory()})
	a, secret := appComUso(t, ks)
	if err := ks.Setup(a); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "/api/documentos/7", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	a.Handler().ServeHTTP(descartar{}, req)

	if rel, _ := ks.Usage(context.Background(), UsageQuery{}); rel.Total != 0 {
		t.Fatalf("gravou antes da hora: %+v", rel)
	}
	if err := a.RunShutdown(); err != nil {
		t.Fatal(err)
	}
	if rel, _ := ks.Usage(context.Background(), UsageQuery{}); rel.Total != 1 {
		t.Fatalf("o Shutdown não gravou: %+v", rel)
	}
}

// Uma chave revogada continua tendo histórico: a pergunta "quem estava usando
// isto?" chega depois da revogação, não antes.
func TestUsoSobreviveARevogacao(t *testing.T) {
	ks := APIKeys(KeyOptions{Usage: UsageMemory()})
	a, secret := appComUso(t, ks)
	req, _ := http.NewRequest("GET", "/api/documentos/7", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	a.Handler().ServeHTTP(descartar{}, req)
	_ = ks.Flush(context.Background())

	chaves, _ := ks.All()
	if err := ks.Revoke(nil, chaves[0].ID); err != nil {
		t.Fatal(err)
	}
	rel, _ := ks.Usage(context.Background(), UsageQuery{Key: chaves[0].ID})
	if rel.Total != 1 {
		t.Fatalf("o histórico sumiu com a chave: %+v", rel)
	}
}

// A retenção apaga o que passou do prazo, e só isso.
func TestUsoRetencao(t *testing.T) {
	store := UsageMemory()
	agora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	velho := agora.AddDate(0, 0, -30)
	if err := store.Add(context.Background(), []UsageRow{
		{KeyID: "k1", Method: "GET", Route: "/a", Day: velho.Truncate(24 * time.Hour), Count: 5, Last: velho},
		{KeyID: "k1", Method: "GET", Route: "/a", Day: agora.Truncate(24 * time.Hour), Count: 2, Last: agora},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(context.Background(), agora.AddDate(0, 0, -7)); err != nil {
		t.Fatal(err)
	}
	rel, _ := store.Query(context.Background(), UsageQuery{})
	if rel.Total != 2 {
		t.Fatalf("a poda levou o que não devia: %+v", rel)
	}
}

// Idle responde onde o dado está: quais chaves não são usadas desde quando. É a
// pergunta que a issue pedia no `trilha audit`, feita no lugar em que ela pode
// ser respondida.
func TestUsoChavesOciosas(t *testing.T) {
	ks := APIKeys(KeyOptions{Usage: UsageMemory()})
	a, secret := appComUso(t, ks)
	if _, _, err := ks.Issue(nil, "que nunca chamou", nil, 0); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "/api/documentos/7", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	a.Handler().ServeHTTP(descartar{}, req)
	_ = ks.Flush(context.Background())

	ociosas, err := ks.Idle(context.Background(), time.Now().AddDate(0, 0, -90))
	if err != nil {
		t.Fatal(err)
	}
	if len(ociosas) != 1 || ociosas[0].Name != "que nunca chamou" {
		t.Fatalf("ociosas = %+v", ociosas)
	}
}

// Sem store, nada muda: quem não pediu contagem não paga por ela.
func TestUsoDesligadoNaoContaNemFalha(t *testing.T) {
	ks := APIKeys(KeyOptions{})
	a, secret := appComUso(t, ks)
	req, _ := http.NewRequest("GET", "/api/documentos/7", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	a.Handler().ServeHTTP(descartar{}, req)
	if err := ks.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.Usage(context.Background(), UsageQuery{}); err != ErrNoUsage {
		t.Fatalf("err = %v", err)
	}
}

// storeLento conta quantas vezes foi escrito, para o teste provar que nenhuma
// requisição esperou por ele.
type storeLento struct {
	UsageStore
	mu sync.Mutex
	n  int
}

func (s *storeLento) Add(ctx context.Context, rows []UsageRow) error {
	s.mu.Lock()
	s.n++
	s.mu.Unlock()
	return s.UsageStore.Add(ctx, rows)
}

func (s *storeLento) chamadas() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.n
}

// descartar é um http.ResponseWriter que joga fora, para as mil requisições não
// alocarem mil buffers. Cada goroutine usa o seu, que é o que o -race exige.
type descartar struct{}

func (descartar) Header() http.Header         { return http.Header{} }
func (descartar) Write(b []byte) (int, error) { return len(b), nil }
func (descartar) WriteHeader(int)             {}
