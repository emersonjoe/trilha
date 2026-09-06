package trilha

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

// flashApp: /apagar escreve os avisos e redireciona, /destino mostra o que
// chegou. É o ciclo inteiro do POST → 303 → GET num app de dez linhas.
func flashApp(t *testing.T, write func(c *Ctx)) *App {
	t.Helper()
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	a.Register(Route{Pattern: "/apagar", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		write(c)
		return c.Redirect("/destino")
	}}})
	a.Register(Route{Pattern: "/destino", Page: func(c *Ctx) (h.Node, error) {
		var n []h.Node
		for _, f := range c.Flashes() {
			n = append(n, h.Div(h.Textf("%s:%s", f.Kind, f.Text)))
		}
		return h.Div(n...), nil
	}})
	return a
}

// flashCookieOf devolve o cookie de flash escrito pela resposta.
func flashCookieOf(t *testing.T, rec interface{ Result() *http.Response }) *http.Cookie {
	t.Helper()
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == flashCookie {
			return ck
		}
	}
	return nil
}

func TestFlashSobreviveAoRedirect(t *testing.T) {
	a := flashApp(t, func(c *Ctx) { c.Flash("success", "Post apagado") })
	rec := get(t, a, "POST", "/apagar", "", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status %d", rec.Code)
	}
	ck := flashCookieOf(t, rec)
	if ck == nil || ck.Value == "" {
		t.Fatal("nenhum cookie de flash")
	}
	if !ck.HttpOnly {
		t.Fatal("o aviso é lido pelo servidor: o cookie é HttpOnly")
	}
	next := get(t, a, "GET", "/destino", "", map[string]string{"Cookie": flashCookie + "=" + ck.Value})
	if !strings.Contains(next.Body.String(), "success:Post apagado") {
		t.Fatalf("a página seguinte não mostrou o aviso: %s", next.Body.String())
	}
	// Lido é gasto: o cookie volta apagado.
	if gone := flashCookieOf(t, next); gone == nil || gone.MaxAge >= 0 {
		t.Fatal("o cookie tinha de ser apagado ao ser lido")
	}
	again := get(t, a, "GET", "/destino", "", nil)
	if strings.Contains(again.Body.String(), "Post apagado") {
		t.Fatal("o aviso apareceu duas vezes")
	}
}

func TestFlashGuardaAOrdem(t *testing.T) {
	a := flashApp(t, func(c *Ctx) {
		c.Flash("success", "primeiro")
		c.Flash("error", "segundo")
	})
	ck := flashCookieOf(t, get(t, a, "POST", "/apagar", "", nil))
	if ck == nil {
		t.Fatal("sem cookie")
	}
	body := get(t, a, "GET", "/destino", "", map[string]string{"Cookie": flashCookie + "=" + ck.Value}).Body.String()
	i, j := strings.Index(body, "primeiro"), strings.Index(body, "segundo")
	if i < 0 || j < 0 || i > j {
		t.Fatalf("ordem errada: %s", body)
	}
}

func TestFlashAdulteradoEIgnorado(t *testing.T) {
	a := flashApp(t, func(c *Ctx) { c.Flash("success", "só o dono escreve") })
	ck := flashCookieOf(t, get(t, a, "POST", "/apagar", "", nil))
	forged := encodeFlashes([]Flash{{Kind: "error", Text: "invadido"}})
	for _, v := range []string{forged, strings.Replace(ck.Value, "|", "x|", 1)} {
		rec := get(t, a, "GET", "/destino", "", map[string]string{"Cookie": flashCookie + "=" + v})
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "invadido") {
			t.Fatalf("cookie forjado aceito: %d %s", rec.Code, rec.Body.String())
		}
		if gone := flashCookieOf(t, rec); gone == nil || gone.MaxAge >= 0 {
			t.Fatal("o cookie inválido tinha de ser apagado")
		}
	}
}

func TestFlashSemSegredoAvisaUmaVez(t *testing.T) {
	var logged bytes.Buffer
	a := New(Config{Logger: slog.New(slog.NewTextHandler(&logged, nil)), Env: Prod})
	a.Register(Route{Pattern: "/apagar", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		c.Flash("success", "sem segredo não sai")
		return c.Redirect("/")
	}}})
	for range 3 {
		if ck := flashCookieOf(t, get(t, a, "POST", "/apagar", "", nil)); ck != nil {
			t.Fatal("escreveu um aviso que ninguém pode verificar")
		}
	}
	if n := strings.Count(logged.String(), "TRILHA_SECRET"); n != 1 {
		t.Fatalf("o aviso no log tem de sair uma vez, saiu %d", n)
	}
}

func TestFlashEmFragmentoVaiNoCabecalho(t *testing.T) {
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	a.Register(Route{Pattern: "/lista", Page: func(c *Ctx) (h.Node, error) {
		c.Flash("success", "Salvo")
		return h.Div(h.ID("lista"), h.Text("ok")), nil
	}})
	rec := get(t, a, "GET", "/lista", "", map[string]string{fragmentHeader: "lista"})
	v := rec.Header().Get(flashHeader)
	if v == "" {
		t.Fatal("o fragmento não tem redirect: o aviso vai no cabeçalho")
	}
	if flashCookieOf(t, rec) != nil {
		t.Fatal("cookie num fragmento não serve para nada")
	}
	b, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		t.Fatalf("o cabeçalho tem de ser base64url: %v", err)
	}
	var got []Flash
	if err := json.Unmarshal(b, &got); err != nil || len(got) != 1 || got[0].Text != "Salvo" {
		t.Fatalf("%s %v", b, err)
	}
}

func TestFlashEmFragmentoQueRedirecionaVaiNoCookie(t *testing.T) {
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	a.Register(Route{Pattern: "/salvar", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		c.Flash("success", "Salvo")
		return c.Redirect("/pronto")
	}}})
	rec := get(t, a, "POST", "/salvar", "", map[string]string{fragmentHeader: "lista"})
	if rec.Header().Get(locationHeader) == "" {
		t.Fatal("o fragmento tinha de mandar navegar")
	}
	if rec.Header().Get(flashHeader) != "" {
		t.Fatal("quem vai navegar mostra o aviso na página que chega")
	}
	if flashCookieOf(t, rec) == nil {
		t.Fatal("sem cookie o aviso morre no caminho")
	}
}

func TestFlashesLidoDuasVezesEOMesmo(t *testing.T) {
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	var first, second []Flash
	a.Register(Route{Pattern: "/ler", Page: func(c *Ctx) (h.Node, error) {
		first, second = c.Flashes(), c.Flashes()
		return h.Text("ok"), nil
	}})
	src := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	src.Register(Route{Pattern: "/apagar", Methods: map[string]HandlerFunc{"POST": func(c *Ctx) error {
		c.Flash("success", "um")
		return c.Redirect("/ler")
	}}})
	ck := flashCookieOf(t, get(t, src, "POST", "/apagar", "", nil))
	get(t, a, "GET", "/ler", "", map[string]string{"Cookie": flashCookie + "=" + ck.Value})
	if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
		t.Fatalf("%v %v", first, second)
	}
}

func TestFlashSemRedirectApareceNaMesmaPagina(t *testing.T) {
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 32))})
	a.Register(Route{Pattern: "/aqui", Page: func(c *Ctx) (h.Node, error) {
		c.Flash("error", "não deu")
		var n []h.Node
		for _, f := range c.Flashes() {
			n = append(n, h.Text(f.Text))
		}
		return h.Div(n...), nil
	}})
	rec := get(t, a, "GET", "/aqui", "", nil)
	if !strings.Contains(rec.Body.String(), "não deu") {
		t.Fatalf("%s", rec.Body.String())
	}
	if flashCookieOf(t, rec) != nil {
		t.Fatal("já foi mostrado: não pode aparecer de novo na próxima página")
	}
}

func TestFlashEscapaOTexto(t *testing.T) {
	a := flashApp(t, func(c *Ctx) { c.Flash("error", "<b>ops</b>") })
	ck := flashCookieOf(t, get(t, a, "POST", "/apagar", "", nil))
	body := get(t, a, "GET", "/destino", "", map[string]string{"Cookie": flashCookie + "=" + ck.Value}).Body.String()
	if strings.Contains(body, "<b>ops</b>") || !strings.Contains(body, "&lt;b&gt;ops&lt;/b&gt;") {
		t.Fatalf("texto de flash não é HTML: %s", body)
	}
}

func TestFlashLimitaOTamanho(t *testing.T) {
	a := flashApp(t, func(c *Ctx) {
		for i := range 8 {
			c.Flash("", string(rune('a'+i))+strings.Repeat("x", 500))
		}
	})
	ck := flashCookieOf(t, get(t, a, "POST", "/apagar", "", nil))
	if ck == nil {
		t.Fatal("sem cookie")
	}
	if len(ck.Value) > 4096 {
		t.Fatalf("cookie de %d bytes: o navegador recusaria", len(ck.Value))
	}
	body := get(t, a, "GET", "/destino", "", map[string]string{"Cookie": flashCookie + "=" + ck.Value}).Body.String()
	if strings.Count(body, "<div>:") != maxFlashes {
		t.Fatalf("guardou avisos demais: %s", body)
	}
	if strings.Contains(body, ">:a") {
		t.Fatal("o mais antigo é o que sai quando não cabe")
	}
	if n := strings.Count(body, "x"); n != maxFlashes*(maxFlashRunes-1) {
		t.Fatalf("cada aviso é cortado em %d runas; contei %d x", maxFlashRunes, n)
	}
}
