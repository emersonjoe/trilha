package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assinaHMAC(h func() hash.Hash, secret string, body []byte) string {
	m := hmac.New(h, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func pedidoHMAC(header, value string, body []byte) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/hook", bytes.NewReader(body))
	if header != "" {
		r.Header.Set(header, value)
	}
	return r
}

// A assinatura certa devolve o corpo; a errada, o corpo alterado, o corpo
// grande demais, o cabeçalho ausente, o hex inválido e o segredo vazio caem
// todos no mesmo ErrSignature — um erro só, de propósito.
func TestVerifyHMAC(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account","entry":[]}`)
	const secret = "meta-app-secret"
	boa := "sha256=" + assinaHMAC(sha256.New, secret, body)

	got, err := VerifyHMAC(pedidoHMAC("X-Hub-Signature-256", boa, body), HMACOpts{Secret: secret})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("corpo devolvido = %q", got)
	}

	alterado := append(append([]byte(nil), body...), ' ')
	grande := bytes.Repeat([]byte("a"), 65)
	falhas := map[string]struct {
		r *http.Request
		o HMACOpts
	}{
		"assinatura errada":     {pedidoHMAC("X-Hub-Signature-256", "sha256="+assinaHMAC(sha256.New, "outro", body), body), HMACOpts{Secret: secret}},
		"corpo alterado":        {pedidoHMAC("X-Hub-Signature-256", boa, alterado), HMACOpts{Secret: secret}},
		"sem cabeçalho":         {pedidoHMAC("", "", body), HMACOpts{Secret: secret}},
		"prefixo faltando":      {pedidoHMAC("X-Hub-Signature-256", assinaHMAC(sha256.New, secret, body), body), HMACOpts{Secret: secret}},
		"hex inválido":          {pedidoHMAC("X-Hub-Signature-256", "sha256=zz", body), HMACOpts{Secret: secret}},
		"segredo vazio":         {pedidoHMAC("X-Hub-Signature-256", boa, body), HMACOpts{}},
		"corpo acima do limite": {pedidoHMAC("X-Hub-Signature-256", "sha256="+assinaHMAC(sha256.New, secret, grande), grande), HMACOpts{Secret: secret, MaxBody: 64}},
	}
	for nome, f := range falhas {
		if _, err := VerifyHMAC(f.r, f.o); !errors.Is(err, ErrSignature) {
			t.Errorf("%s: err = %v, quero ErrSignature", nome, err)
		}
	}

	// Exatamente no limite passa: o limite é inclusivo.
	no := bytes.Repeat([]byte("a"), 64)
	if _, err := VerifyHMAC(pedidoHMAC("X-Hub-Signature-256", "sha256="+assinaHMAC(sha256.New, secret, no), no), HMACOpts{Secret: secret, MaxBody: 64}); err != nil {
		t.Fatalf("64 bytes com MaxBody 64: %v", err)
	}
}

// O esquema do GitHub é o mesmo cabeçalho e o mesmo prefixo; o do Stripe e
// de quem manda o hex nu se descreve com Header, Prefix e Hash.
func TestVerifyHMACOutrosProvedores(t *testing.T) {
	body := []byte(`{"action":"opened"}`)
	const secret = "gh-secret"

	// GitHub: X-Hub-Signature-256: sha256=<hex>.
	r := pedidoHMAC("X-Hub-Signature-256", "sha256="+assinaHMAC(sha256.New, secret, body), body)
	if _, err := VerifyHMAC(r, HMACOpts{Header: "X-Hub-Signature-256", Prefix: "sha256=", Secret: secret}); err != nil {
		t.Fatalf("github: %v", err)
	}
	// O legado do GitHub, SHA-1, só para mostrar que Hash é configurável.
	r = pedidoHMAC("X-Hub-Signature", "sha1="+assinaHMAC(sha1.New, secret, body), body)
	if _, err := VerifyHMAC(r, HMACOpts{Header: "X-Hub-Signature", Prefix: "sha1=", Secret: secret, Hash: sha1.New}); err != nil {
		t.Fatalf("github sha1: %v", err)
	}
	// Um provedor que manda o hex sem prefixo nenhum.
	r = pedidoHMAC("X-Signature", assinaHMAC(sha256.New, secret, body), body)
	if _, err := VerifyHMAC(r, HMACOpts{Header: "X-Signature", Prefix: HMACNoPrefix, Secret: secret}); err != nil {
		t.Fatalf("sem prefixo: %v", err)
	}
	// E um MAC de outro segredo falha, e não passa por acaso: a comparação é
	// do MAC decodificado, não do texto.
	r = pedidoHMAC("X-Signature", strings.ToUpper(assinaHMAC(sha256.New, "x", body)), body)
	if _, err := VerifyHMAC(r, HMACOpts{Header: "X-Signature", Prefix: HMACNoPrefix, Secret: secret}); !errors.Is(err, ErrSignature) {
		t.Fatalf("err = %v", err)
	}
}
