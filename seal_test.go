package trilha

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func comSegredo(t *testing.T, secret, anterior string) {
	t.Helper()
	var prev []byte
	if anterior != "" {
		prev = []byte(anterior)
	}
	New(Config{Logger: quiet(), Secret: []byte(secret), PreviousSecret: prev})
	t.Cleanup(func() {
		sealMu.Lock()
		sealKeys = nil
		sealMu.Unlock()
	})
}

func TestSealAbreOQueFechou(t *testing.T) {
	comSegredo(t, strings.Repeat("k", 40), "")
	sealed, err := Seal([]byte("sk-abcdef123456"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("sk-abcdef")) {
		t.Fatal("o valor está legível no que foi gravado")
	}
	plain, err := Open(sealed)
	if err != nil || string(plain) != "sk-abcdef123456" {
		t.Fatalf("%q %v", plain, err)
	}
}

// Nonce aleatório: dois selos do mesmo valor não podem ser iguais, ou o banco
// vira um oráculo de "estes dois registros têm a mesma chave".
func TestSealNaoRepete(t *testing.T) {
	comSegredo(t, strings.Repeat("k", 40), "")
	a, _ := Seal([]byte("mesmo valor"))
	b, _ := Seal([]byte("mesmo valor"))
	if bytes.Equal(a, b) {
		t.Fatal("dois selos iguais para o mesmo valor")
	}
}

// Um byte mexido não abre: é GCM, e é essa a diferença entre cifrar e cifrar
// direito.
func TestSealRecusaOQueFoiMexido(t *testing.T) {
	comSegredo(t, strings.Repeat("k", 40), "")
	sealed, _ := Seal([]byte("valor"))
	sealed[len(sealed)-1] ^= 0x01
	if _, err := Open(sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("err = %v", err)
	}
	// E o byte de versão viaja como dado adicional: trocá-lo também invalida.
	sealed[len(sealed)-1] ^= 0x01
	sealed[0] = 9
	if _, err := Open(sealed); !errors.Is(err, ErrSealed) {
		t.Fatalf("versão trocada: %v", err)
	}
}

// A rotação: o que foi gravado com o segredo antigo continua abrindo enquanto
// o PreviousSecret existir, e para de abrir quando ele sai.
func TestSealRodaOSegredo(t *testing.T) {
	comSegredo(t, strings.Repeat("a", 40), "")
	antigo, err := Seal([]byte("chave antiga"))
	if err != nil {
		t.Fatal(err)
	}

	comSegredo(t, strings.Repeat("b", 40), strings.Repeat("a", 40))
	plain, err := Open(antigo)
	if err != nil || string(plain) != "chave antiga" {
		t.Fatalf("com PreviousSecret devia abrir: %q %v", plain, err)
	}

	comSegredo(t, strings.Repeat("b", 40), "")
	if _, err := Open(antigo); !errors.Is(err, ErrSealed) {
		t.Fatalf("sem PreviousSecret devia falhar alto: %v", err)
	}
}

func TestSealSemSegredoNaoDevolveClaro(t *testing.T) {
	sealMu.Lock()
	sealKeys = nil
	sealMu.Unlock()
	if _, err := Seal([]byte("x")); !errors.Is(err, ErrNoSecret) {
		t.Fatalf("err = %v", err)
	}
}

// ---- o tipo -----------------------------------------------------------------

type integracao struct {
	Nome  string `json:"nome"  form:"nome"`
	Token Secret `json:"token" form:"token"`
}

// As três saídas por onde um segredo vaza sem ninguém querer: JSON, log e %v.
func TestSecretNaoVazaPorDescuido(t *testing.T) {
	s := Secret("sk-abcdef123456")

	b, err := json.Marshal(integracao{Nome: "llm", Token: s})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "abcdef123456") {
		t.Fatalf("o JSON levou o segredo: %s", b)
	}
	if !strings.Contains(string(b), "sk-") || !strings.Contains(string(b), "3456") {
		t.Fatalf("a máscara não identifica a chave: %s", b)
	}

	var log bytes.Buffer
	slog.New(slog.NewTextHandler(&log, nil)).Info("integração", "token", s)
	if strings.Contains(log.String(), "abcdef123456") {
		t.Fatalf("o log levou o segredo: %s", log.String())
	}

	if strings.Contains(fmt.Sprintf("%v · %s", s, s), "abcdef123456") {
		t.Fatal("a formatação levou o segredo")
	}
	// E o Reveal é o único caminho, escrito de propósito.
	if s.Reveal() != "sk-abcdef123456" {
		t.Fatal("Reveal")
	}
}

// Segredo curto demais não mostra "quase tudo": mostra nada.
func TestSecretCurtoNaoMostraNada(t *testing.T) {
	if got := Secret("abc123").String(); strings.Contains(got, "abc") {
		t.Fatalf("máscara curta = %q", got)
	}
	if Secret("").String() != "" {
		t.Fatal("vazio devia ser vazio")
	}
}

// O que vai para a coluna é o selo. Sem o driver.Valuer, o database/sql
// converteria a string por baixo e gravaria em claro — uma linha de Exec de
// distância, e em silêncio.
func TestSecretVaiCifradoParaOBanco(t *testing.T) {
	comSegredo(t, strings.Repeat("k", 40), "")
	var v driver.Valuer = Secret("sk-abcdef123456")
	got, err := v.Value()
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := got.([]byte)
	if !ok {
		t.Fatalf("a coluna devia receber bytes, recebeu %T", got)
	}
	if bytes.Contains(raw, []byte("abcdef")) {
		t.Fatal("foi em claro para a coluna")
	}
	var volta Secret
	if err := volta.Scan(raw); err != nil || volta.Reveal() != "sk-abcdef123456" {
		t.Fatalf("%v %q", err, volta.Reveal())
	}
}

// "Deixe em branco para manter": um formulário não tem como mandar "não
// mudou", e a máscara voltando não pode virar o valor guardado.
func TestBindDeSecretVazioPreserva(t *testing.T) {
	comSegredo(t, strings.Repeat("k", 40), "")
	a := New(Config{Logger: quiet(), Secret: []byte(strings.Repeat("k", 40))})

	atual := integracao{Nome: "llm", Token: Secret("sk-abcdef123456")}
	c := bindCtx(a, "nome=llm&token=")
	got := atual
	if err := c.Bind(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "sk-abcdef123456" {
		t.Fatalf("o vazio apagou o segredo: %q", got.Token.Reveal())
	}

	// A máscara de volta também é "não mudou".
	c = bindCtx(a, "nome=llm&token="+atual.Token.String())
	got = atual
	if err := c.Bind(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "sk-abcdef123456" {
		t.Fatalf("a máscara virou o valor: %q", got.Token.Reveal())
	}

	// E um valor novo troca.
	c = bindCtx(a, "nome=llm&token=sk-novo-9999")
	got = atual
	if err := c.Bind(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "sk-novo-9999" {
		t.Fatalf("não trocou: %q", got.Token.Reveal())
	}
}

func bindCtx(a *App, body string) *Ctx {
	req := httptest.NewRequest("POST", "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	return newCtx(a, &responseWriter{ResponseWriter: rec, status: http.StatusOK}, req, kindAPI)
}
