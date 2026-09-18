package trilha

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// catalogFS is the i18n/ folder of an app that serves migrants: Portuguese is
// the default, Haitian Creole falls back to French.
func catalogFS() fstest.MapFS {
	return fstest.MapFS{
		"i18n/pt-BR.json": {Data: []byte(`{
			"protocolo.recebido": "Protocolo %s recebido",
			"protocolo.pendentes": {"one": "%d protocolo pendente", "other": "%d protocolos pendentes"},
			"so.no.padrao": "Só o padrão tem"
		}`)},
		"i18n/fr.json": {Data: []byte(`{
			"protocolo.recebido": "Dossier %s reçu",
			"protocolo.pendentes": {"one": "%d dossier en attente", "other": "%d dossiers en attente"}
		}`)},
		"i18n/ht.json": {Data: []byte(`{"protocolo.recebido": "Dosye %s resevwa"}`)},
	}
}

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	k, err := LoadCatalog(catalogFS(), "i18n")
	if err != nil {
		t.Fatal(err)
	}
	k.Fallback = map[string]string{"ht": "fr"}
	return k
}

// catalogApp answers with c.T(key, args...) in the locale of the request.
func catalogApp(t *testing.T, k *Catalog, log *slog.Logger, key string, args ...any) *App {
	t.Helper()
	a := New(Config{Locales: []string{"pt-BR", "ht", "fr"}, Catalog: k, Logger: log})
	a.Register(Route{Pattern: "/", Methods: map[string]HandlerFunc{
		"GET": func(c *Ctx) error { return c.Text(200, c.T(key, args...)) },
	}})
	return a
}

func say(t *testing.T, a *App, lang string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/?lang="+lang, nil))
	return rec.Body.String()
}

// #270 — a mensagem sai no idioma do pedido, com os argumentos no lugar.
func TestCatalogLookup(t *testing.T) {
	a := catalogApp(t, testCatalog(t), quiet(), "protocolo.recebido", "A-12")
	if got := say(t, a, "pt-BR"); got != "Protocolo A-12 recebido" {
		t.Errorf("pt-BR: %q", got)
	}
	if got := say(t, a, "fr"); got != "Dossier A-12 reçu" {
		t.Errorf("fr: %q", got)
	}
	if got := say(t, a, "ht"); got != "Dosye A-12 resevwa" {
		t.Errorf("ht: %q", got)
	}
}

// Plural simples: o número lidera os argumentos, 1 é "one" e o resto é "other".
func TestCatalogPlural(t *testing.T) {
	k := testCatalog(t)
	one := catalogApp(t, k, quiet(), "protocolo.pendentes", 1)
	many := catalogApp(t, k, quiet(), "protocolo.pendentes", 4)
	if got := say(t, one, "pt-BR"); got != "1 protocolo pendente" {
		t.Errorf("um: %q", got)
	}
	if got := say(t, many, "pt-BR"); got != "4 protocolos pendentes" {
		t.Errorf("vários: %q", got)
	}
	if got := say(t, many, "fr"); got != "4 dossiers en attente" {
		t.Errorf("fr: %q", got)
	}
}

// A cadeia de fallback: o crioulo que não traduziu cai no francês, e o que o
// francês também não tem cai no padrão.
func TestCatalogFallbackChain(t *testing.T) {
	k := testCatalog(t)
	a := catalogApp(t, k, quiet(), "protocolo.pendentes", 2)
	if got := say(t, a, "ht"); got != "2 dossiers en attente" {
		t.Errorf("ht não caiu no francês: %q", got)
	}
	b := catalogApp(t, k, quiet(), "so.no.padrao")
	if got := say(t, b, "ht"); got != "Só o padrão tem" {
		t.Errorf("ht não caiu no padrão: %q", got)
	}
}

// Chave que ninguém definiu não vira tela em branco: volta a própria chave e
// o aviso sai uma vez só, não uma por requisição.
func TestCatalogMissingKeyWarnsOnce(t *testing.T) {
	var buf bytes.Buffer
	a := catalogApp(t, testCatalog(t), slog.New(slog.NewTextHandler(&buf, nil)), "nao.existe")
	for i := 0; i < 3; i++ {
		if got := say(t, a, "ht"); got != "nao.existe" {
			t.Fatalf("chave ausente devolveu %q", got)
		}
	}
	if n := strings.Count(buf.String(), "nao.existe"); n != 1 {
		t.Fatalf("avisou %d vezes:\n%s", n, buf.String())
	}
}

// Sem catálogo configurado, c.T devolve a chave e nada explode.
func TestCatalogAbsent(t *testing.T) {
	a := catalogApp(t, nil, quiet(), "protocolo.recebido", "A-12")
	if got := say(t, a, "pt-BR"); got != "protocolo.recebido" {
		t.Fatalf("sem catálogo: %q", got)
	}
}

// Um arquivo que não é um objeto JSON para a carga dizendo qual é o arquivo.
func TestLoadCatalogRefusesBadFile(t *testing.T) {
	bad := fstest.MapFS{
		"i18n/pt-BR.json": {Data: []byte(`{"ok": "ok"}`)},
		"i18n/fr.json":    {Data: []byte(`["nao", "e", "objeto"]`)},
	}
	_, err := LoadCatalog(bad, "i18n")
	if err == nil || !strings.Contains(err.Error(), "i18n/fr.json") {
		t.Fatalf("erro sem o nome do arquivo: %v", err)
	}
	worse := fstest.MapFS{"i18n/en.json": {Data: []byte(`{"k": {"algum": "coisa"}}`)}}
	if _, err := LoadCatalog(worse, "i18n"); err == nil || !strings.Contains(err.Error(), `"k"`) {
		t.Fatalf("erro sem a chave: %v", err)
	}
}

// Locales e Missing são o que a CLI pergunta.
func TestCatalogLocalesAndMissing(t *testing.T) {
	k := testCatalog(t)
	if got := strings.Join(k.Locales(), ","); got != "fr,ht,pt-BR" {
		t.Errorf("Locales: %q", got)
	}
	keys := []string{"protocolo.recebido", "protocolo.pendentes", "so.no.padrao"}
	if got := k.Missing("pt-BR", keys); len(got) != 0 {
		t.Errorf("faltando no padrão: %v", got)
	}
	got := k.Missing("ht", keys)
	if len(got) != 2 || got[0] != "protocolo.pendentes" || got[1] != "so.no.padrao" {
		t.Errorf("faltando no ht: %v", got)
	}
}
