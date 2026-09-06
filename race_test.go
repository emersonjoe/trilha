package trilha

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/emersonjoe/trilha/h"
)

// concurrentApp touches every place the runtime keeps state between requests:
// the asset hash cache, the metric counters, the rate limit buckets, the app
// values and the signer.
func concurrentApp() *App {
	files := fstest.MapFS{"site.css": &fstest.MapFile{Data: []byte("body{}")}}
	a := New(Config{
		Env:    Prod,
		Logger: quiet(),
		Public: files,
		Secret: []byte("0123456789abcdef0123456789abcdef"),
		// Wide enough that nothing is refused: what this test wants from the
		// limiter is the shared bucket map under contention, not the limit.
		RateLimit:     RateLimit{RPS: 1e6, Burst: 1e6},
		Observability: Observability{Metrics: "/_trilha/metrics", Trusted: []string{"0.0.0.0/0", "::/0"}},
	})
	a.Values()["cor"] = "azul"
	a.Register(Route{Pattern: "/entrar", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		return h.Form(h.Method("post"), CSRFInput(c)), nil
	}, Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		if err := c.SetSigned("user", c.Form("user"), time.Hour); err != nil {
			return err
		}
		return c.Redirect("/painel")
	}}})
	a.Register(Route{Pattern: "/painel", Layouts: []LayoutFunc{rootLayout}, Page: func(c *Ctx) (h.Node, error) {
		user, ok := c.Signed("user")
		if !ok {
			return nil, Errorf(http.StatusUnauthorized, "no session")
		}
		return h.Div(h.Data("css", c.Asset("/site.css")), h.P(h.Text("hello "+user))), nil
	}})
	a.Register(Route{Pattern: "/api/eco/{id}", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error {
			return c.JSON(http.StatusOK, map[string]any{"id": c.Param("id"), "cor": c.App().Values()["cor"]})
		},
	}})
	return a
}

// TestConcorrencia: várias goroutines contra o mesmo App. Sob -race é este
// teste que dá sentido à flag — sem ele o detector não vê concorrência nenhuma
// na suíte, e a etiqueta seria só etiqueta. Cada resposta confere o seu
// invariante, porque uma corrida costuma aparecer como o dado de uma
// requisição saindo na resposta de outra.
func TestConcorrencia(t *testing.T) {
	a := concurrentApp()
	handler := a.Handler()
	versao := a.Asset("/site.css")

	do := func(method, target string, cookies map[string]string, body string) *httptest.ResponseRecorder {
		var r *http.Request
		if body != "" {
			r = httptest.NewRequest(method, target, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			r = httptest.NewRequest(method, target, nil)
		}
		for name, value := range cookies {
			r.AddCookie(&http.Cookie{Name: name, Value: value})
		}
		if token, ok := cookies[CSRFCookie]; ok {
			r.Header.Set(CSRFHeader, token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		return rec
	}

	const goroutines, rodadas = 32, 20
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			token := strings.Repeat(fmt.Sprintf("%x", g%16), 32)
			for i := 0; i < rodadas; i++ {
				user := fmt.Sprintf("ana-%d-%d", g, i)

				// A sessão desta goroutine tem que voltar para esta goroutine.
				rec := do(http.MethodPost, "/entrar", map[string]string{CSRFCookie: token}, "user="+user)
				if rec.Code != http.StatusSeeOther {
					t.Errorf("POST /entrar: status = %d", rec.Code)
					return
				}
				var sessao string
				for _, ck := range rec.Result().Cookies() {
					if ck.Name == "user" {
						sessao = ck.Value
					}
				}
				if sessao == "" {
					t.Error("POST /entrar não pôs a sessão")
					return
				}
				rec = do(http.MethodGet, "/painel", map[string]string{"user": sessao}, "")
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hello "+user) {
					t.Errorf("GET /painel: status = %d, corpo = %q", rec.Code, rec.Body.String())
					return
				}
				if !strings.Contains(rec.Body.String(), versao) {
					t.Errorf("GET /painel: versão do estático = %q, corpo = %q", versao, rec.Body.String())
					return
				}

				// O parâmetro de caminho e o valor global saem na resposta certa.
				rec = do(http.MethodGet, fmt.Sprintf("/api/eco/%d-%d", g, i), nil, "")
				if want := fmt.Sprintf(`"id":"%d-%d"`, g, i); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
					t.Errorf("GET /api/eco: status = %d, corpo = %q, queria %s", rec.Code, rec.Body.String(), want)
					return
				}
				if !strings.Contains(rec.Body.String(), `"cor":"azul"`) {
					t.Errorf("GET /api/eco: valor global = %q", rec.Body.String())
					return
				}

				if rec := do(http.MethodGet, "/site.css", nil, ""); rec.Code != http.StatusOK || rec.Body.String() != "body{}" {
					t.Errorf("GET /site.css: status = %d, corpo = %q", rec.Code, rec.Body.String())
					return
				}
				if rec := do(http.MethodGet, "/_trilha/metrics", nil, ""); rec.Code != http.StatusOK {
					t.Errorf("GET /_trilha/metrics: status = %d", rec.Code)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	// As métricas contaram todo mundo: o contador é o estado mais disputado do
	// runtime, e um valor menor que o esperado é corrida que o -race pode não
	// ter pego no exato instante.
	rec := do(http.MethodGet, "/_trilha/metrics", nil, "")
	if !strings.Contains(rec.Body.String(), "trilha_requests_total") {
		t.Fatalf("métricas sem o contador de requisições:\n%s", rec.Body.String())
	}
}
