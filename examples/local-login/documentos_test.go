package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/emersonjoe/trilha"
)

// recebida is one request as the API saw it: the path, the query and the
// credential. Looking at the call from the other side is the only way to tell
// "the screen shows documents" from "the screen shows documents and the call
// went out as this person".
type recebida struct {
	Path  string
	Query string
	Auth  string
}

// acervoFalso is the API that already exists. It answers the document the
// screen renders and records every request, so a test can read what left.
type acervoFalso struct {
	*httptest.Server
	mu   sync.Mutex
	recs []recebida
}

func (a *acervoFalso) recebidas() []recebida {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]recebida(nil), a.recs...)
}

func acervo(t *testing.T) *acervoFalso {
	t.Helper()
	a := &acervoFalso{}
	a.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.recs = append(a.recs, recebida{Path: r.URL.Path, Query: r.URL.RawQuery, Auth: r.Header.Get("Authorization")})
		a.mu.Unlock()
		itens := []map[string]any{
			{"id": "d-1", "filename": "contrato-2026.pdf", "status": "processed", "pages": 12},
			{"id": "d-2", "filename": "nota-fiscal-9.pdf", "status": "pending", "pages": 1},
		}
		if q := r.URL.Query().Get("q"); q != "" {
			itens = itens[:1]
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": itens, "total": len(itens), "page": 1})
	}))
	t.Cleanup(a.Close)
	return a
}

// #94 — a listagem que era do browser roda no servidor, e a chamada sai com a
// credencial da sessão. O que este teste olha é o lado de lá: quem perguntou à
// API e com o quê.
func TestDocumentosChamaAAPIComOTokenDaSessao(t *testing.T) {
	a := acervo(t)
	c := cliente(t, a.URL)

	// Sem sessão não há tela e não há chamada: o middleware de /painel para
	// antes, então a API nem fica sabendo que alguém tentou.
	c.Get("/painel/documentos", navegador()).WantStatus(http.StatusFound).WantContains("/entrar?next=")
	if n := len(a.recebidas()); n != 0 {
		t.Fatalf("anônimo gerou %d chamadas à API", n)
	}

	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)
	body := c.Get("/painel/documentos").WantStatus(200).
		WantContains("contrato-2026.pdf", "nota-fiscal-9.pdf", "processed").Body.String()

	recs := a.recebidas()
	if len(recs) != 1 {
		t.Fatalf("chamadas à API = %d, queria 1", len(recs))
	}
	if recs[0].Path != "/api/documents" {
		t.Fatalf("caminho = %q", recs[0].Path)
	}
	if recs[0].Auth != "Bearer jwt-da-ana" {
		t.Fatalf("Authorization = %q, queria o token da sessão", recs[0].Auth)
	}
	// O token é da sessão, e a sessão o browser carrega sem ler: ele não pode
	// aparecer no HTML, que é exatamente onde estava na versão em JavaScript.
	if strings.Contains(body, "jwt-da-ana") {
		t.Fatal("o token da sessão vazou para a página")
	}
}

// O filtro da URL é o filtro da API: a busca não é feita em Go sobre a página
// inteira que a API mandou.
func TestDocumentosMandaABuscaParaAAPI(t *testing.T) {
	a := acervo(t)
	c := cliente(t, a.URL)
	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)

	c.Get("/painel/documentos?q=contrato").WantStatus(200).WantContains("contrato-2026.pdf")
	recs := a.recebidas()
	if len(recs) != 1 || !strings.Contains(recs[0].Query, "q=contrato") {
		t.Fatalf("a busca não chegou à API: %+v", recs)
	}
}

// A credencial que vale é a da sessão. Um Authorization mandado pelo browser
// não é uma credencial deste app, e não é ele que chega à API.
func TestDocumentosIgnoraOAuthorizationDoBrowser(t *testing.T) {
	a := acervo(t)
	c := cliente(t, a.URL)
	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)

	c.Get("/painel/documentos", trilha.WithHeader("Authorization", "Bearer roubado")).WantStatus(200)
	recs := a.recebidas()
	if len(recs) != 1 || recs[0].Auth != "Bearer jwt-da-ana" {
		t.Fatalf("o token do browser chegou à API: %+v", recs)
	}
}

// Sem API_URL a tela diz o que falta em vez de quebrar — o exemplo roda
// sozinho, como o resto dele.
func TestDocumentosSemAPIURLExplica(t *testing.T) {
	c := cliente(t, "")
	entrar(t, c, "ana@exemplo.com", "segredo-da-ana").WantStatus(http.StatusSeeOther)
	c.Get("/painel/documentos").WantStatus(200).WantContains("A API não está configurada")
}
