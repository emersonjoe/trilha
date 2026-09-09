package trilha

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type rascunho struct {
	Nome  string `json:"nome"`
	Passo int    `json:"passo"`
}

// draftCtx é um Ctx sobre um recorder, com segredo: assinar é o que faz um
// rascunho ser rascunho e não um campo escondido que qualquer um edita.
func draftCtx(t *testing.T, a *App, cookies ...*http.Cookie) (*Ctx, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest("GET", "/passo", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	return newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindPage), rec
}

func draftApp(t *testing.T) *App {
	t.Helper()
	return New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("s", 40))})
}

// setados devolve os cookies que a resposta mandou, para a requisição seguinte
// — que é o que o navegador faria.
func setados(rec *httptest.ResponseRecorder) []*http.Cookie {
	return (&http.Response{Header: rec.Header()}).Cookies()
}

func TestDraftAtravessaOsPassos(t *testing.T) {
	a := draftApp(t)
	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save(rascunho{Nome: "Ada", Passo: 1}, time.Hour); err != nil {
		t.Fatal(err)
	}

	c2, _ := draftCtx(t, a, setados(rec)...)
	var got rascunho
	if err := c2.Draft("cadastro").Load(&got); err != nil {
		t.Fatal(err)
	}
	if got.Nome != "Ada" || got.Passo != 1 {
		t.Fatalf("voltou %+v", got)
	}
}

// Sem rascunho não é falha: é a resposta que manda alguém para o passo 1.
func TestDraftSemNadaEhErrNoDraft(t *testing.T) {
	c, _ := draftCtx(t, draftApp(t))
	var got rascunho
	if err := c.Draft("cadastro").Load(&got); !errors.Is(err, ErrNoDraft) {
		t.Fatalf("err = %v", err)
	}
}

// Um rascunho editado à mão não é um rascunho: a assinatura falha e não há o
// que continuar.
func TestDraftAdulteradoNaoVolta(t *testing.T) {
	a := draftApp(t)
	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save(rascunho{Nome: "Ada"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	cks := setados(rec)
	cks[0].Value = "x" + cks[0].Value[1:]

	c2, _ := draftCtx(t, a, cks...)
	var got rascunho
	if err := c2.Draft("cadastro").Load(&got); !errors.Is(err, ErrNoDraft) {
		t.Fatalf("aceitou um cookie mexido: %v", err)
	}
}

func TestDraftVencidoVoltaAoComeco(t *testing.T) {
	a := draftApp(t)
	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save(rascunho{Nome: "Ada"}, -time.Minute); err != nil {
		t.Fatal(err)
	}
	c2, _ := draftCtx(t, a, setados(rec)...)
	var got rascunho
	if err := c2.Draft("cadastro").Load(&got); !errors.Is(err, ErrNoDraft) {
		t.Fatalf("rascunho vencido continuou de pé: %v", err)
	}
}

// O campo que mudou de nome entre dois deploys chega como um cookie que não
// casa mais com a struct. Isso é o passo 1 de novo, não um 500 na cara de
// quem estava no meio do formulário.
func TestDraftDeOutroFormatoVoltaAoComeco(t *testing.T) {
	a := draftApp(t)
	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save([]string{"lista", "onde", "era", "struct"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	c2, _ := draftCtx(t, a, setados(rec)...)
	var got rascunho
	if err := c2.Draft("cadastro").Load(&got); !errors.Is(err, ErrNoDraft) {
		t.Fatalf("err = %v", err)
	}
}

func TestDraftClearEncerra(t *testing.T) {
	a := draftApp(t)
	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save(rascunho{Nome: "Ada"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	c2, rec2 := draftCtx(t, a, setados(rec)...)
	c2.Draft("cadastro").Clear()

	c3, _ := draftCtx(t, a, setados(rec2)...)
	var got rascunho
	if err := c3.Draft("cadastro").Load(&got); !errors.Is(err, ErrNoDraft) {
		t.Fatalf("o rascunho sobreviveu ao Clear: %v", err)
	}
}

// Dois rascunhos na mesma pessoa não se pisam: o nome é do formulário.
func TestDraftPorNome(t *testing.T) {
	a := draftApp(t)
	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save(rascunho{Nome: "Ada"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	c.Draft("importacao").Save(rascunho{Nome: "planilha"}, time.Hour)

	c2, _ := draftCtx(t, a, setados(rec)...)
	var um, outro rascunho
	if err := c2.Draft("cadastro").Load(&um); err != nil || um.Nome != "Ada" {
		t.Fatalf("%v %+v", err, um)
	}
	if err := c2.Draft("importacao").Load(&outro); err != nil || outro.Nome != "planilha" {
		t.Fatalf("%v %+v", err, outro)
	}
}

// ---- o que não cabe num cookie ---------------------------------------------

type memDrafts struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (s *memDrafts) Save(key string, data []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string][]byte{}
	}
	s.m[key] = data
	return nil
}

func (s *memDrafts) Load(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[key]
	return b, ok
}

func (s *memDrafts) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

func grande() rascunho { return rascunho{Nome: strings.Repeat("n", maxDraftCookie+1)} }

// Sem store, um rascunho grande demais é um erro que diz o que fazer — não um
// cookie que o navegador descarta em silêncio, deixando o passo 2 sem o passo
// 1 e ninguém sabendo por quê.
func TestDraftGrandeSemStoreExplica(t *testing.T) {
	c, rec := draftCtx(t, draftApp(t))
	err := c.Draft("cadastro").Save(grande(), time.Hour)
	if err == nil {
		t.Fatal("aceitou o que não cabe")
	}
	if !strings.Contains(err.Error(), "Config.Drafts") {
		t.Fatalf("o erro tem de dizer onde mexer: %v", err)
	}
	if len(setados(rec)) != 0 {
		t.Fatal("mandou um cookie que o navegador ia jogar fora")
	}
}

func TestDraftGrandeVaiParaOStore(t *testing.T) {
	store := &memDrafts{}
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("s", 40)), Drafts: store})

	c, rec := draftCtx(t, a)
	if err := c.Draft("cadastro").Save(grande(), time.Hour); err != nil {
		t.Fatal(err)
	}
	if len(store.m) != 1 {
		t.Fatalf("o store devia ter uma entrada: %d", len(store.m))
	}
	// O cookie leva a chave, não o rascunho.
	cks := setados(rec)
	if len(cks) != 1 || len(cks[0].Value) > 200 {
		t.Fatalf("o cookie devia ser só a chave: %d bytes", len(cks[0].Value))
	}

	c2, _ := draftCtx(t, a, cks...)
	var got rascunho
	if err := c2.Draft("cadastro").Load(&got); err != nil {
		t.Fatal(err)
	}
	if got.Nome != grande().Nome {
		t.Fatal("voltou diferente do que entrou")
	}

	// E o Clear tira dos dois lugares: o cookie some e a entrada também — um
	// rascunho que virou registro não pode continuar ocupando o store.
	c3, _ := draftCtx(t, a, cks...)
	c3.Draft("cadastro").Clear()
	if len(store.m) != 0 {
		t.Fatalf("o Clear deixou lixo no store: %d", len(store.m))
	}
}

// Nome de rascunho é escrito pelo desenvolvedor, mas um deslize nele não pode
// virar um segundo atributo do cookie.
func TestDraftNomeTortoNaoVazaParaOCookie(t *testing.T) {
	c, rec := draftCtx(t, draftApp(t))
	if err := c.Draft("cadastro; Path=/; HttpOnly").Save(rascunho{Nome: "Ada"}, time.Hour); err != nil {
		t.Fatal(err)
	}
	cks := setados(rec)
	if len(cks) != 1 {
		t.Fatalf("cookies = %d", len(cks))
	}
	if strings.ContainsAny(cks[0].Name, "; =") {
		t.Fatalf("nome do cookie: %q", cks[0].Name)
	}
}
