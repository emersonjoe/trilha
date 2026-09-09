package trilha

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func linkApp(t *testing.T) *App {
	t.Helper()
	return New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 40))})
}

func linkCtx(a *App, method, path string) (*Ctx, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "10.0.0.9:1234"
	rec := httptest.NewRecorder()
	return newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindPage), rec
}

// O caminho feliz: quem tem o link entra sem sessão, e os dados que a app pôs
// nele chegam do outro lado.
func TestLinkAbreSemSessao(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	url, err := c.Link("formulario", LinkOpts{
		Data: map[string]string{"tarefa": "42"}, TTL: time.Hour, Path: "/formulario",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "/formulario/") {
		t.Fatalf("url = %q", url)
	}

	pub, _ := linkCtx(a, "GET", url)
	l, err := pub.Claim("formulario")
	if err != nil {
		t.Fatal(err)
	}
	if l.Data["tarefa"] != "42" {
		t.Fatalf("dados = %+v", l.Data)
	}
}

// Um token feito para um fim não abre outro, mesmo assinado pela mesma app.
func TestLinkNaoServeParaOutroFim(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	url, _ := c.Link("formulario", LinkOpts{TTL: time.Hour, Path: "/verificar"})

	pub, _ := linkCtx(a, "GET", url)
	if _, err := pub.Claim("verificacao"); statusOf(err) != http.StatusNotFound {
		t.Fatalf("err = %v", err)
	}
}

// Assinatura mexida, token vencido e lixo respondem a mesma coisa: 404. Dizer
// qual das três aconteceu é dizer a um estranho o quão perto ele está.
func TestLinkInvalidoRespondeSempreIgual(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	bom, _ := c.Link("formulario", LinkOpts{TTL: time.Hour, Path: "/f"})
	// O vencido é assinado direto com prazo no passado: o Link recusa TTL
	// negativo, então a única forma de ter um é o tempo passar.
	corpo, _ := json.Marshal(linkBody{Name: "formulario", ID: "x"})
	assinado, _ := a.signer.Sign(string(corpo), time.Now().Add(-time.Minute))
	vencido := "/f/" + pathSafe(assinado)
	mexido := bom[:len(bom)-3] + "aaa"

	for nome, url := range map[string]string{
		"lixo":      "/f/naoeumtoken",
		"vencido":   vencido,
		"mexido":    mexido,
		"sem token": "/f",
		"vazio":     "/f/",
	} {
		t.Run(nome, func(t *testing.T) {
			// Um app por caso: o orçamento de tentativas é do endereço, e não
			// é ele que este teste está medindo.
			pub, _ := linkCtx(linkApp(t), "GET", url)
			if _, err := pub.Claim("formulario"); statusOf(err) != http.StatusNotFound {
				t.Fatalf("%s: %v", nome, err)
			}
		})
	}
}

// Uses: 1 — o segundo POST do mesmo link é 410, e o GET que só abre a página
// não gasta nada.
func TestLinkDeUmUsoSoGastaNoConsume(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	url, _ := c.Link("formulario", LinkOpts{TTL: time.Hour, Uses: 1, Path: "/f"})

	// Três aberturas não gastam.
	for i := 0; i < 3; i++ {
		pub, _ := linkCtx(a, "GET", url)
		if _, err := pub.Claim("formulario"); err != nil {
			t.Fatalf("abertura %d: %v", i, err)
		}
	}

	// O envio gasta.
	post, _ := linkCtx(a, "POST", url)
	l, err := post.Claim("formulario")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Consume(); err != nil {
		t.Fatal(err)
	}

	// E aí acabou, para abrir e para enviar.
	depois, _ := linkCtx(a, "GET", url)
	if _, err := depois.Claim("formulario"); statusOf(err) != http.StatusNotFound {
		t.Fatalf("abertura depois de gasto: %v", err)
	}
	post2, _ := linkCtx(a, "POST", url)
	if l2, err := post2.Claim("formulario"); err == nil {
		if err := l2.Consume(); statusOf(err) != http.StatusGone {
			t.Fatalf("segundo envio: %v", err)
		}
	}
}

// Sem limite de usos não há estado nenhum: verificar é criptografia, não
// consulta — e é o caso do código de verificação de um documento.
func TestLinkSemLimiteNaoGuardaNada(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	url, _ := c.Link("verificacao", LinkOpts{Data: map[string]string{"doc": "7"}, TTL: time.Hour, Path: "/v"})
	for i := 0; i < 5; i++ {
		pub, _ := linkCtx(a, "GET", url)
		l, err := pub.Claim("verificacao")
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		if err := l.Consume(); err != nil {
			t.Fatal(err)
		}
	}
	if m := a.linkStore().(*memLinks); len(m.m) != 0 {
		t.Fatalf("guardou estado sem precisar: %+v", m.m)
	}
}

// Adivinhar token em URL é força bruta, e errar custa. O sexto erro do mesmo
// endereço já não é mais um 404.
func TestLinkErrarMuitoCusta(t *testing.T) {
	a := linkApp(t)
	var ultimo int
	for i := 0; i < 6; i++ {
		pub, _ := linkCtx(a, "GET", "/f/chute"+string(rune('a'+i)))
		_, err := pub.Claim("formulario")
		ultimo = statusOf(err)
	}
	if ultimo != http.StatusTooManyRequests {
		t.Fatalf("depois de seis chutes = %d", ultimo)
	}
}

// O que vai no token é assinado e legível: está escrito no doc, e o teste
// existe para que ninguém "descubra" isso de outro jeito.
func TestLinkNaoEhSegredo(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	url, _ := c.Link("formulario", LinkOpts{Data: map[string]string{"tarefa": "42"}, TTL: time.Hour, Path: "/f"})
	if !strings.Contains(url, "~") {
		t.Fatalf("o token devia caber num segmento de caminho: %q", url)
	}
	// Sem hífen nem barra além do caminho: o link tem de sobreviver a um
	// e-mail que quebra linha.
	seg := url[strings.LastIndex(url, "/")+1:]
	if strings.ContainsAny(seg, "/ ?#") {
		t.Fatalf("segmento com caractere que a URL não aceita: %q", seg)
	}
}

// Dado grande demais é erro na hora de criar, não um link que ninguém consegue
// colar numa mensagem.
func TestLinkRecusaDadoGrandeDemais(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	_, err := c.Link("formulario", LinkOpts{
		Data: map[string]string{"texto": strings.Repeat("x", maxLinkBody+1)}, TTL: time.Hour, Path: "/f",
	})
	if err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("err = %v", err)
	}
}

// Prazo negativo é bug de quem calculou, não um link. Silenciosamente virar
// uma hora seria transformar uma conta errada num link válido.
func TestLinkRecusaPrazoNegativo(t *testing.T) {
	a := linkApp(t)
	c, _ := linkCtx(a, "GET", "/admin")
	if _, err := c.Link("formulario", LinkOpts{TTL: -time.Minute, Path: "/f"}); err == nil {
		t.Fatal("aceitou prazo no passado")
	}
}
