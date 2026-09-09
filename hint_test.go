package trilha

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func quieto() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// #118 — o open redirect. O destino de um redirecionamento quase sempre veio
// de um formulário ou de um `?next=`, que é como se volta para a página que
// pediu login; um Redirect que segue para qualquer lugar é um link de phishing
// no seu próprio domínio.
func TestRedirectRecusaSairDoSite(t *testing.T) {
	recusados := []string{
		"https://evil.example/x",
		"http://evil.example",
		// Protocolo-relativo: o navegador vai para evil.example, e o valor
		// começa com barra, que é o que uma checagem ingênua olha.
		"//evil.example/x",
		// As variantes com contrabarra existem porque cada uma passou por uma
		// checagem que alguém achou suficiente.
		"/\\evil.example",
		"https:/\\evil.example",
		"javascript:alert(1)",
		"mailto:alguem@exemplo.com",
		"evil.example/x",
		"",
	}
	for _, url := range recusados {
		err := Redirect(url)
		if err == nil {
			t.Fatalf("%q passou", url)
		}
		var re *RedirectError
		if errors.As(err, &re) {
			t.Fatalf("%q virou redirecionamento para %q", url, re.URL)
		}
		h := HintOf(err)
		if h == nil || h.Code != ErrRedirectAbsolute {
			t.Fatalf("%q: sem hint (%v)", url, err)
		}
		// A primeira pergunta de quem lê o erro é "recusou o quê?" — e o valor
		// vai citado, que é como uma contrabarra chega legível ao terminal.
		if !strings.Contains(err.Error(), strconv.Quote(url)) {
			t.Fatalf("o erro não diz o que foi recusado: %v", err)
		}
		if h.Repair == "" || !strings.Contains(h.Repair, "RedirectExternal") {
			t.Fatalf("%q: o conserto não diz o que fazer: %q", url, h.Repair)
		}
	}
}

// E o caminho de sempre continua sendo o caminho de sempre — senão a recusa
// teria trocado um problema por outro.
func TestRedirectAceitaCaminho(t *testing.T) {
	for _, url := range []string{"/", "/painel", "/painel?ok=1", "/a/b/c#x", "/painel?next=%2Fx"} {
		err := Redirect(url)
		var re *RedirectError
		if !errors.As(err, &re) || re.URL != url || re.Code != 303 {
			t.Fatalf("%q: %v", url, err)
		}
	}
	if err := RedirectCode("/x", 301); err != nil {
		var re *RedirectError
		if !errors.As(err, &re) || re.Code != 301 {
			t.Fatalf("RedirectCode: %v", err)
		}
	}
}

// Sair de propósito tem nome, e o nome é o registro. Quem revisa lendo
// RedirectExternal sabe que alguém quis; lendo Redirect, sabe que ninguém
// poderia ter conseguido sem querer.
func TestRedirectExternalSaiEDizQueSaiu(t *testing.T) {
	err := RedirectExternal("https://gov.example/pagar")
	var re *RedirectError
	if !errors.As(err, &re) || re.URL != "https://gov.example/pagar" {
		t.Fatalf("%v", err)
	}
}

// O Hint embrulha: quem já tratava um erro não passa a tratar outro.
func TestHintDeixaVerOErroDeBaixo(t *testing.T) {
	raiz := errors.New("a causa")
	h := NewHint("E_TESTE", raiz).Fix("faça assim").Doc("/reference/errors")
	if !errors.Is(h, raiz) {
		t.Fatal("errors.Is não atravessa o hint")
	}
	if !strings.Contains(h.Error(), "E_TESTE") || !strings.Contains(h.Error(), "a causa") {
		t.Fatalf("mensagem = %q", h.Error())
	}
	if HintOf(errors.New("outro")) != nil {
		t.Fatal("achou hint onde não há")
	}
}

// Em dev a página de erro mostra o conserto; em produção, nada disso vaza —
// a frase é para quem escreve o código, e quem está do outro lado não escreveu.
func TestPaginaDeErroMostraOConsertoSoEmDev(t *testing.T) {
	pagina := func(env Env) string {
		a := New(Config{Env: env, Logger: quieto(),
			Secret: []byte("0123456789abcdef0123456789abcdef")})
		a.Register(Route{Pattern: "/x", Kind: KindPage,
			Methods: map[string]HandlerFunc{"GET": func(c *Ctx) error {
				return NewHint("E_TESTE", errors.New("a causa")).
					Fix("faça assim, e não daquele jeito").Doc("/reference/errors")
			}}})
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
		if rec.Code != 500 {
			t.Fatalf("status = %d", rec.Code)
		}
		return rec.Body.String()
	}

	dev := pagina(Dev)
	for _, quero := range []string{"E_TESTE", "faça assim, e não daquele jeito", "/reference/errors"} {
		if !strings.Contains(dev, quero) {
			t.Fatalf("dev não mostrou %q:\n%s", quero, dev)
		}
	}
	prod := pagina(Prod)
	for _, nao := range []string{"E_TESTE", "faça assim", "a causa"} {
		if strings.Contains(prod, nao) {
			t.Fatalf("produção vazou %q:\n%s", nao, prod)
		}
	}
}

// #118 — segredo curto em produção não sobe. Tudo que a assinatura protege
// vale exatamente o que a chave vale.
func TestSegredoCurtoNaoSobeEmProducao(t *testing.T) {
	a := New(Config{Env: Prod, Logger: quieto(), Secret: []byte("curto")})
	err := a.checkSecret()
	if err == nil {
		t.Fatal("subiu com cinco bytes de segredo")
	}
	h := HintOf(err)
	if h == nil || h.Code != ErrSecretShort {
		t.Fatalf("sem hint: %v", err)
	}
	// O número está na mensagem porque "curto demais" deixa alguém contando
	// caracteres, e o comando está nela porque "gere um" sem dizer como é meia
	// instrução.
	if !strings.Contains(err.Error(), "5 bytes") || !strings.Contains(h.Repair, "trilha secret") {
		t.Fatalf("mensagem = %v / %q", err, h.Repair)
	}

	// Em dev sobe: um segredo de desenvolvimento é um segredo de
	// desenvolvimento, e recusar seria o framework atrapalhar no único lugar
	// onde não deve.
	dev := New(Config{Env: Dev, Logger: quieto(), Secret: []byte("curto")})
	if err := dev.checkSecret(); err != nil {
		t.Fatalf("dev recusou: %v", err)
	}
	// Segredo nenhum é outra coisa, e já tem resposta: a assinatura falha alto
	// onde é tentada, e o audit é crítico sobre isso para quem assina e calado
	// para quem não assina. Recusar aqui também pararia um app que não assina
	// nada, que é o framework atrapalhar quem não lhe deve nada.
	vazio := New(Config{Env: Prod, Logger: quieto()})
	if err := vazio.checkSecret(); err != nil {
		t.Fatalf("recusou um app sem segredo nenhum: %v", err)
	}
	// E o tamanho certo passa em qualquer ambiente.
	bom := New(Config{Env: Prod, Logger: quieto(), Secret: []byte(strings.Repeat("a", MinSecretLen))})
	if err := bom.checkSecret(); err != nil {
		t.Fatalf("recusou um segredo do tamanho certo: %v", err)
	}
}
