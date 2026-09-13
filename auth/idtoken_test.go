package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// O caso da issue #208: a app Go é a frente de uma API que já existe e que
// emite a própria sessão a partir do id_token — o backend tem a mesma
// client_id e verifica o token contra o JWKS do provedor, então nenhum segredo
// compartilhado novo nasce. Para isso o callback precisa ver o token.
func TestOnLoginTokenVeOTokenEAsClaims(t *testing.T) {
	idp := newIDP(t)
	// "hd" é o domínio do Google Workspace: uma claim que o kit não mapeia, e o
	// segundo motivo da issue para entregar as claims cruas.
	idp.claims = map[string]any{"email": "ana@exemplo.com", "name": "Ana", "hd": "exemplo.com"}

	var ordem []string
	var visto *IDToken
	a := New(idp.provider(), Options{
		Store: NewMemoryStore(),
		OnLogin: func(c *trilha.Ctx, u *User) error {
			ordem = append(ordem, "OnLogin")
			return nil
		},
		OnLoginToken: func(c *trilha.Ctx, u *User, tok *IDToken) error {
			ordem = append(ordem, "OnLoginToken")
			visto = tok
			// É isto que a app faz com o token: troca-o pela sessão da API e
			// guarda *essa* na sessão daqui. O token não persiste.
			u.Extra = map[string]string{"sessao_da_api": "tk-" + tok.Claims.Subject}
			return nil
		},
	})
	b := newBrowser(t, authApp(t, a))

	if rec := b.login(idp, ""); rec.Code != http.StatusFound && rec.Code != http.StatusSeeOther {
		t.Fatalf("callback → %d", rec.Code)
	}
	if strings.Join(ordem, ",") != "OnLogin,OnLoginToken" {
		t.Fatalf("ordem dos callbacks = %v", ordem)
	}
	if visto == nil {
		t.Fatal("OnLoginToken não foi chamado")
	}
	// O Raw é o token que o provedor assinou: confere contra o JWKS dele.
	claims, err := a.Provider().verify(context.Background(), visto.Raw, visto.Claims.Nonce)
	if err != nil {
		t.Fatalf("o Raw não verifica contra o provedor: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("sub do Raw = %q", claims.Subject)
	}
	// E as claims já verificadas chegam inteiras, inclusive a que o kit não mapeia.
	if visto.Claims == nil || visto.Claims.All["hd"] != "exemplo.com" {
		t.Fatalf("claims = %+v", visto.Claims)
	}
	if visto.Claims.Email != "ana@exemplo.com" {
		t.Fatalf("email das claims = %q", visto.Claims.Email)
	}
	// O que o callback escreveu no Extra está na sessão gravada.
	sessoes := a.opts.Store.(*MemoryStore).Sessions("user-1")
	if len(sessoes) != 1 || sessoes[0].Extra["sessao_da_api"] != "tk-user-1" {
		t.Fatalf("sessão gravada = %+v", sessoes)
	}
}

// O token do provedor não entra na sessão: ele existe durante a requisição do
// callback e morre nela. Guardá-lo no Extra o faria viajar no cookie do
// navegador, com o prazo da sessão e não o dele.
func TestIDTokenNaoEntraNaSessao(t *testing.T) {
	idp := newIDP(t)
	idp.claims = map[string]any{"email": "ana@exemplo.com"}
	loja := NewMemoryStore()
	var raw string
	a := New(idp.provider(), Options{Store: loja,
		OnLoginToken: func(c *trilha.Ctx, u *User, tok *IDToken) error {
			raw = tok.Raw
			return nil
		}})
	b := newBrowser(t, authApp(t, a))
	b.login(idp, "")

	if raw == "" {
		t.Fatal("sem token para conferir")
	}
	gravada, err := json.Marshal(loja.Sessions("user-1"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gravada), raw) {
		t.Fatal("o id_token foi gravado na sessão")
	}
	for name, value := range b.cookies {
		if strings.Contains(value, raw) {
			t.Fatalf("o id_token viajou no cookie %q", name)
		}
	}
}

// Um erro do OnLoginToken para o login, como o do OnLogin: é onde a app diz
// "o backend recusou esta conta" — lista de espera, conta desativada.
func TestOnLoginTokenQueFalhaRecusaOLogin(t *testing.T) {
	idp := newIDP(t)
	idp.claims = map[string]any{"email": "ana@exemplo.com"}
	a := New(idp.provider(), Options{
		OnLoginToken: func(c *trilha.Ctx, u *User, tok *IDToken) error {
			return errors.New("o backend não abriu sessão para esta conta")
		}})
	b := newBrowser(t, authApp(t, a))

	if rec := b.login(idp, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("callback → %d, queria 401", rec.Code)
	}
	if rec := b.get("/", nil); rec.Body.String() != "anonimo" {
		t.Fatalf("depois da recusa = %q", rec.Body.String())
	}
}

// O login local não tem id_token, e chamar o callback com um Raw vazio
// convidaria a app a tratar "não houve token" como "o token é isto".
func TestLoginLocalNaoChamaOnLoginToken(t *testing.T) {
	chamou := false
	a := Sessions(Options{LoginPath: "/entrar",
		OnLoginToken: func(c *trilha.Ctx, u *User, tok *IDToken) error {
			chamou = true
			return nil
		}})
	b := newBrowser(t, updateApp(t, a))

	b.get("/entrar", nil)
	if chamou {
		t.Fatal("OnLoginToken foi chamado por um login sem provedor")
	}
	if rec := b.get("/marca", nil); rec.Code != 200 {
		t.Fatalf("/marca → %d", rec.Code)
	}
}
