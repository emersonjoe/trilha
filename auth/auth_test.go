package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T, o Options) (*fakeIDP, *Auth, *browser) {
	t.Helper()
	idp := newIDP(t)
	a := New(idp.provider(), o)
	return idp, a, newBrowser(t, authApp(t, a))
}

// US1: quem entra pelo provedor chega autenticado, com nome, e-mail e papéis.
func TestLoginRoundTrip(t *testing.T) {
	idp, _, b := setup(t, Options{})
	idp.claims = map[string]any{"email": "ana@exemplo.com", "name": "Ana", "roles": []any{"admin"}}

	rec := b.login(idp, "")
	if rec.Code != http.StatusFound && rec.Code != http.StatusSeeOther {
		t.Fatalf("retorno → %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("destino %q", got)
	}
	if _, ok := b.cookies["trilha_session"]; !ok {
		t.Fatal("sem cookie de sessão")
	}
	if rec := b.get("/admin", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "ana@exemplo.com") {
		t.Fatalf("/admin → %d %q", rec.Code, rec.Body.String())
	}
}

// FR-003: os cookies do fluxo são assinados, HttpOnly, Lax, curtos, e somem
// no retorno — deixá-los para trás é um convite ao replay.
func TestFlowCookiesAreShortAndCleared(t *testing.T) {
	idp, _, b := setup(t, Options{})
	rec := b.get("/entrar", nil)
	found := map[string]bool{}
	for _, c := range rec.Result().Cookies() {
		if !strings.HasPrefix(c.Name, "trilha_oidc_") {
			continue
		}
		found[c.Name] = true
		if !c.HttpOnly {
			t.Errorf("%s sem HttpOnly", c.Name)
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("%s SameSite=%v", c.Name, c.SameSite)
		}
		if c.MaxAge <= 0 || c.MaxAge > 600 {
			t.Errorf("%s Max-Age=%d", c.Name, c.MaxAge)
		}
	}
	for _, want := range []string{"trilha_oidc_state", "trilha_oidc_nonce", "trilha_oidc_verifier"} {
		if !found[want] {
			t.Errorf("faltou %s", want)
		}
	}
	// O login completo apaga os três.
	u := rec.Header().Get("Location")
	q := mustQuery(t, u)
	code := idp.authorize(q)
	b.get("/entrar/retorno?code="+code+"&state="+q.Get("state"), nil)
	for name := range found {
		if _, still := b.cookies[name]; still {
			t.Errorf("%s sobreviveu ao retorno", name)
		}
	}
}

// FR-004: a sessão é assinada, HttpOnly, Lax, e Secure quando há TLS.
func TestSessionCookieFlags(t *testing.T) {
	idp, a, b := setup(t, Options{})
	rec := b.login(idp, "")
	var sess *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "trilha_session" {
			sess = c
		}
	}
	if sess == nil {
		t.Fatal("sem cookie de sessão")
	}
	if !sess.HttpOnly || sess.SameSite != http.SameSiteLaxMode {
		t.Fatalf("flags fracas: %+v", sess)
	}
	if strings.Contains(sess.Value, "user-1") {
		t.Fatal("o cookie precisa ser assinado, não texto solto sem MAC")
	}
	// Sob HTTPS o mesmo cookie sai com Secure.
	app := authApp(t, a)
	req := httptest.NewRequest("GET", "https://exemplo.com/entrar", nil)
	rec2 := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec2, req)
	for _, c := range rec2.Result().Cookies() {
		if !c.Secure {
			t.Errorf("%s sem Secure sob TLS", c.Name)
		}
	}
}

// US2: um retorno forjado não vira sessão. Cada linha é uma regra sozinha.
func TestCallbackRefusals(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(*fakeIDP)
		query  func(state, code string) string
	}{
		{"state trocado", nil, func(state, code string) string { return "?code=" + code + "&state=outro" }},
		{"state ausente", nil, func(state, code string) string { return "?code=" + code }},
		{"code ausente", nil, func(state, code string) string { return "?state=" + state }},
		{"erro do provedor", nil, func(state, code string) string { return "?error=access_denied&state=" + state }},
		{"code desconhecido", nil, func(state, code string) string { return "?code=inventado&state=" + state }},
		{"nonce ausente no token", func(f *fakeIDP) { f.dropNonce = true }, nil},
		{"token expirado", func(f *fakeIDP) { f.expIn = -2 * time.Minute }, nil},
		{"audiência de outro app", func(f *fakeIDP) { f.tokenAud = "outro-app" }, nil},
		{"emissor trocado", func(f *fakeIDP) { f.tokenIss = "https://evil.exemplo" }, nil},
		{"assinado com outra chave", func(f *fakeIDP) {
			k, _ := rsa.GenerateKey(rand.Reader, 2048)
			f.signWith = k
		}, nil},
		{"kid desconhecido", func(f *fakeIDP) { f.signKid = "k-inexistente" }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idp, _, b := setup(t, Options{})
			if tc.break_ != nil {
				tc.break_(idp)
			}
			rec := b.get("/entrar", nil)
			q := mustQuery(t, rec.Header().Get("Location"))
			code := idp.authorize(q)
			tail := "?code=" + code + "&state=" + q.Get("state")
			if tc.query != nil {
				tail = tc.query(q.Get("state"), code)
			}
			rec = b.get("/entrar/retorno"+tail, nil)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("virou %d, esperado 401", rec.Code)
			}
			if _, ok := b.cookies["trilha_session"]; ok {
				t.Fatal("criou sessão a partir de um retorno inválido")
			}
		})
	}
}

// FR-005: nenhum token simétrico ou sem assinatura passa, por mais que o
// cabeçalho peça.
func TestSymmetricAndNoneAreRefused(t *testing.T) {
	idp := newIDP(t)
	p := idp.provider()
	for _, alg := range []string{"none", "HS256", "hs256", ""} {
		tok := enc64([]byte(`{"alg":"`+alg+`","kid":"k1"}`)) + "." +
			enc64([]byte(`{"iss":"`+idp.srv.URL+`","sub":"x","aud":"app"}`)) + "."
		if _, err := p.verify(context.Background(), tok, ""); err == nil {
			t.Fatalf("alg %q foi aceito", alg)
		}
	}
}

// FR-006: next só leva para dentro do próprio app.
func TestNextIsNotAnOpenRedirect(t *testing.T) {
	for _, tc := range []struct{ next, want string }{
		{"/painel", "/painel"},
		{"/painel?a=1", "/painel?a=1"},
		{"https://evil.exemplo", "/"},
		{"//evil.exemplo", "/"},
		{"/\\evil.exemplo", "/"},
		{"javascript:alert(1)", "/"},
	} {
		idp, _, b := setup(t, Options{})
		rec := b.login(idp, tc.next)
		if got := rec.Header().Get("Location"); got != tc.want {
			t.Errorf("next=%q levou para %q, esperado %q", tc.next, got, tc.want)
		}
	}
}

// US3: navegador anônimo vai para o login; API anônima recebe 401.
func TestAnonymousBrowserRedirectsAndAPIGets401(t *testing.T) {
	_, _, b := setup(t, Options{})
	rec := b.get("/admin", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("navegador → %d", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/entrar?next=%2Fadmin" {
		t.Fatalf("destino %q", got)
	}
	rec = b.get("/api/dados", http.Header{"Accept": {"application/json"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("API → %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("API não pode ser redirecionada para HTML: %q", loc)
	}
}

// US4: papel exigido. Quem está logado mas não tem o papel recebe 403 — 401
// mandaria de volta ao login, num laço.
func TestRequireRole(t *testing.T) {
	idp, _, b := setup(t, Options{})
	idp.claims = map[string]any{"roles": []any{"leitor"}}
	b.login(idp, "")
	if rec := b.get("/admin/relatorio", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("sem papel → %d", rec.Code)
	}

	idp2, _, b2 := setup(t, Options{})
	idp2.claims = map[string]any{"roles": []any{"admin"}}
	b2.login(idp2, "")
	if rec := b2.get("/admin/relatorio", nil); rec.Code != 200 {
		t.Fatalf("com papel → %d", rec.Code)
	}
}

// Os papéis moram em lugares diferentes em cada provedor.
func TestRolesPerProvider(t *testing.T) {
	entra := EntraID("t1", "app", "s", "https://app/cb")
	got := entra.roles(&Claims{All: map[string]any{
		"roles": []any{"Admin", "Admin"}, "groups": []any{"g1"},
	}}, nil)
	if strings.Join(got, ",") != "Admin,g1" {
		t.Fatalf("entra: %v", got)
	}
	kc := Keycloak("https://kc", "r1", "app", "s", "https://app/cb")
	got = kc.roles(&Claims{All: map[string]any{
		"realm_access":    map[string]any{"roles": []any{"user"}},
		"resource_access": map[string]any{"app": map[string]any{"roles": []any{"admin"}}, "outro": map[string]any{"roles": []any{"root"}}},
	}}, nil)
	if strings.Join(got, ",") != "user,admin" {
		t.Fatalf("keycloak: %v (papéis de outro cliente não valem aqui)", got)
	}
	gen := OIDC("https://i", "app", "s", "https://app/cb")
	got = gen.roles(&Claims{All: map[string]any{"grupos": "editor"}}, []string{"grupos"})
	if strings.Join(got, ",") != "editor" {
		t.Fatalf("claim extra: %v", got)
	}
}

// US5: a sessão termina. Prazo absoluto e ociosidade são checados na leitura.
func TestSessionExpiry(t *testing.T) {
	idp, _, b := setup(t, Options{Absolute: 300 * time.Millisecond})
	b.login(idp, "")
	if rec := b.get("/admin", nil); rec.Code != 200 {
		t.Fatal("sessão nova deveria valer")
	}
	time.Sleep(350 * time.Millisecond)
	if rec := b.get("/admin", nil); rec.Code != http.StatusFound {
		t.Fatalf("sessão vencida ainda vale: %d", rec.Code)
	}

	idp2, _, b2 := setup(t, Options{Idle: 200 * time.Millisecond})
	b2.login(idp2, "")
	time.Sleep(250 * time.Millisecond)
	if rec := b2.get("/admin", nil); rec.Code != http.StatusFound {
		t.Fatalf("sessão ociosa ainda vale: %d", rec.Code)
	}
}

// Logout apaga a sessão e passa pelo provedor quando ele oferece o endpoint.
func TestLogout(t *testing.T) {
	idp, _, b := setup(t, Options{})
	b.login(idp, "")
	rec := b.get("/sair", nil)
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, idp.srv.URL+"/logout") || !strings.Contains(loc, "post_logout_redirect_uri=") {
		t.Fatalf("logout federado ausente: %q", loc)
	}
	if _, ok := b.cookies["trilha_session"]; ok {
		t.Fatal("cookie de sessão sobreviveu ao logout")
	}
	if rec := b.get("/", nil); !strings.Contains(rec.Body.String(), "anonimo") {
		t.Fatal("continua logado")
	}

	// Sem end_session_endpoint, volta para o próprio app.
	idp2, _, b2 := setup(t, Options{AfterLogout: "/tchau"})
	idp2.endSession = false
	b2.login(idp2, "")
	if got := b2.get("/sair", nil).Header().Get("Location"); got != "/tchau" {
		t.Fatalf("destino %q", got)
	}
}

// Com Store, o logout é imediato para todo mundo, não só para o navegador.
func TestStoreGivesImmediateRevocation(t *testing.T) {
	store := NewMemoryStore()
	idp, _, b := setup(t, Options{Store: store})
	b.login(idp, "")
	if rec := b.get("/admin", nil); rec.Code != 200 {
		t.Fatal("login com store falhou")
	}
	for id := range store.data {
		_ = store.Delete(id)
	}
	if rec := b.get("/admin", nil); rec.Code != http.StatusFound {
		t.Fatalf("sessão revogada ainda vale: %d", rec.Code)
	}
}

// O identificador muda a cada login: um cookie plantado antes não é
// promovido a sessão válida (fixação).
func TestLoginRotatesSessionID(t *testing.T) {
	idp, a, _ := setup(t, Options{})
	app := authApp(t, a)
	b1, b2 := newBrowser(t, app), newBrowser(t, app)
	b1.login(idp, "")
	b2.login(idp, "")
	if b1.cookies["trilha_session"] == b2.cookies["trilha_session"] {
		t.Fatal("dois logins geraram a mesma sessão")
	}
}

// FR-002: o segredo do cliente não pode aparecer em log nem em URL.
func TestClientSecretNeverLeaks(t *testing.T) {
	idp := newIDP(t)
	idp.secret = "segredo-que-nao-pode-vazar"
	p := idp.provider()
	a := New(p, Options{})
	var log strings.Builder
	b := newBrowser(t, authAppLog(t, a, &log))

	rec := b.get("/entrar", nil)
	if strings.Contains(rec.Header().Get("Location"), idp.secret) {
		t.Fatal("segredo na URL de autorização")
	}
	// Um retorno quebrado, que é quando o framework mais escreve no log.
	b.get("/entrar/retorno?code=invalido&state=x", nil)
	q := mustQuery(t, rec.Header().Get("Location"))
	b.get("/entrar/retorno?code="+idp.authorize(q)+"&state="+q.Get("state"), nil)
	if strings.Contains(log.String(), idp.secret) {
		t.Fatalf("segredo no log: %s", log.String())
	}
}

// FR-007: toda recusa vira evento de segurança do tipo auth.
func TestRefusalEmitsSecurityEvent(t *testing.T) {
	idp := newIDP(t)
	a := New(idp.provider(), Options{})
	var log strings.Builder
	b := newBrowser(t, authAppLog(t, a, &log))
	b.get("/entrar/retorno?code=x&state=y", nil)
	if !strings.Contains(log.String(), "kind=auth") {
		t.Fatalf("sem evento de segurança: %s", log.String())
	}
}

// Um código só pode ser trocado uma vez.
func TestCodeIsSingleUse(t *testing.T) {
	idp, _, b := setup(t, Options{})
	rec := b.get("/entrar", nil)
	q := mustQuery(t, rec.Header().Get("Location"))
	code := idp.authorize(q)
	back := "/entrar/retorno?code=" + code + "&state=" + q.Get("state")
	if rec := b.get(back, nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("primeira troca → %d", rec.Code)
	}
	if rec := b.get(back, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("reuso do código → %d", rec.Code)
	}
}

// A descoberta precisa bater com o emissor configurado: um documento que
// anuncia outro emissor significa tenant ou realm errado.
func TestDiscoveryIssuerMismatch(t *testing.T) {
	idp := newIDP(t)
	idp.issuerLie = "https://outro.exemplo"
	if _, err := idp.provider().discover(context.Background()); err == nil {
		t.Fatal("emissor divergente foi aceito")
	}
}

// Rotação de chave: o provedor troca a chave e o login continua funcionando,
// mas um kid desconhecido não vira uma busca por requisição.
func TestKeyRotation(t *testing.T) {
	idp := newIDP(t)
	p := idp.provider()
	if _, err := p.discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.keys.key(context.Background(), "k1"); err != nil {
		t.Fatal(err)
	}
	novo, _ := rsa.GenerateKey(rand.Reader, 2048)
	idp.key, idp.kid = novo, "k2"

	// Dentro da janela de estrangulamento, o kid novo é recusado sem rede.
	if _, err := p.keys.key(context.Background(), "k2"); err == nil {
		t.Fatal("buscou o JWKS de novo dentro do minuto")
	}
	p.keys.mu.Lock()
	p.keys.last = time.Now().Add(-2 * time.Minute)
	p.keys.mu.Unlock()
	if _, err := p.keys.key(context.Background(), "k2"); err != nil {
		t.Fatalf("rotação não foi percebida: %v", err)
	}
}

// Uma chave RSA curta demais não serve, por mais que o provedor a anuncie.
func TestShortRSAKeyIsRefused(t *testing.T) {
	small, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	pub := small.Public().(*rsa.PublicKey)
	if _, err := parseJWK("RSA", "", enc64(pub.N.Bytes()), "AQAB", "", ""); err == nil {
		t.Fatal("chave de 1024 bits aceita")
	}
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}
