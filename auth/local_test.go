package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// localApp is the app of someone who owns their users: a login that checks a
// password, a protected page, a logout, and a page guarded by a predicate.
func localApp(t *testing.T, a *Auth) *trilha.App {
	t.Helper()
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	route("/entrar", func(c *trilha.Ctx) error {
		if c.Query("senha") != "certa" {
			return trilha.Errorf(422, "wrong e-mail or password")
		}
		return a.Login(c, &User{Subject: "u-1", Email: "ana@exemplo.com", Roles: []string{"analista"},
			Extra: map[string]string{"api_token": "jwt-da-api", "tenant": "acme"}})
	})
	route("/sair", a.Logout)
	route("/painel", func(c *trilha.Ctx) error {
		u := a.User(c)
		return c.Text(200, "ola "+u.Email+" "+u.Extra["api_token"])
	}, a.Require())
	route("/acme", func(c *trilha.Ctx) error { return c.Text(200, "acme") },
		a.RequireFunc(func(u *User, c *trilha.Ctx) bool { return u.Extra["tenant"] == "acme" }))
	route("/outra", func(c *trilha.Ctx) error { return c.Text(200, "outra") },
		a.RequireFunc(func(u *User, c *trilha.Ctx) bool { return u.Extra["tenant"] == "outra" }))
	route("/admin", func(c *trilha.Ctx) error { return c.Text(200, "admin") }, a.RequireRole("admin"))
	return app
}

// SC-010, SC-012 — the password is the app's business; the session, the cookie
// and the guard are the framework's, and Extra crosses with them.
func TestLoginLocalCriaSessao(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar"})
	b := newBrowser(t, localApp(t, a))
	if rec := b.get("/painel", nil); rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
		t.Fatalf("anônimo devia ir para o login: %d", rec.Code)
	}
	if rec := b.get("/entrar?senha=errada", nil); rec.Code != 422 {
		t.Fatalf("senha errada: %d", rec.Code)
	}
	if _, ok := b.cookies["trilha_session"]; ok {
		t.Fatal("senha errada não abre sessão")
	}
	rec := b.get("/entrar?senha=certa", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("login → %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := b.get("/painel", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "jwt-da-api") {
		t.Fatalf("/painel → %d %q", rec.Code, rec.Body.String())
	}
	// RequireRole keeps working: it is the same session.
	if rec := b.get("/admin", nil); rec.Code != 403 {
		t.Fatalf("/admin → %d", rec.Code)
	}
}

// SC-012 — Extra survives the Store too, where the session is not in the cookie.
func TestExtraSobreviveAoStore(t *testing.T) {
	a := Sessions(Options{Store: NewMemoryStore()})
	b := newBrowser(t, localApp(t, a))
	b.get("/entrar?senha=certa", nil)
	if rec := b.get("/painel", nil); !strings.Contains(rec.Body.String(), "jwt-da-api") {
		t.Fatalf("%q", rec.Body.String())
	}
	// SC-011 — logout without a provider clears the session and lands, and the
	// store forgets it right away.
	rec := b.get("/sair", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("saída → %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := b.get("/painel", nil); rec.Code == 200 {
		t.Fatal("a sessão continuou de pé depois da saída")
	}
}

// SC-014 — the predicate is the rule the app writes; anonymous never reaches it.
func TestRequireFuncDecideComOPredicado(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar"})
	b := newBrowser(t, localApp(t, a))
	if rec := b.get("/acme", nil); rec.Code == 403 {
		t.Fatal("anônimo vai para o login, não para o 403")
	}
	b.get("/entrar?senha=certa", nil)
	if rec := b.get("/acme", nil); rec.Code != 200 {
		t.Fatalf("/acme → %d", rec.Code)
	}
	if rec := b.get("/outra", nil); rec.Code != 403 {
		t.Fatalf("/outra → %d", rec.Code)
	}
}

// SC-015 — OnLogin runs before the session exists, and its error stops it.
func TestOnLoginPodeRecusar(t *testing.T) {
	var seen string
	a := Sessions(Options{OnLogin: func(c *trilha.Ctx, u *User) error {
		seen = u.Subject
		return trilha.Errorf(403, "conta suspensa")
	}})
	b := newBrowser(t, localApp(t, a))
	if rec := b.get("/entrar?senha=certa", nil); rec.Code != 403 {
		t.Fatalf("→ %d", rec.Code)
	}
	if seen != "u-1" || len(b.cookies) != 0 {
		t.Fatalf("OnLogin viu %q, cookies %v", seen, b.cookies)
	}
}

// The OIDC half of the same type says what is wrong instead of dying on a nil
// provider.
func TestFluxoOIDCSemProvedorRespondeErroClaro(t *testing.T) {
	a := Sessions(Options{})
	app := localApp(t, a)
	app.Register(trilha.Route{Pattern: "/oidc", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": a.Start}})
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/oidc", nil))
	if rec.Code != 500 {
		t.Fatalf("→ %d", rec.Code)
	}
	if err := a.Start(nil); err == nil || !strings.Contains(err.Error(), "no provider") {
		t.Fatalf("erro: %v", err)
	}
}

// SC-013 — the hash a Python app already wrote is read as it is: the point of
// the format is not migrating the password table.
func TestCheckPBKDF2LeOHashDoPython(t *testing.T) {
	// python3: hashlib.pbkdf2_hmac("sha256", b"senha-correta", b"sal-de-teste", 200000)
	const doPython = "pbkdf2_sha256$200000$sal-de-teste$gqaWWQDwg8wpKWesTvWWOwcv+3zlh2aOCSEPYPdB0aI="
	if !CheckPBKDF2(doPython, "senha-correta") {
		t.Fatal("o hash do Python não foi aceito")
	}
	if CheckPBKDF2(doPython, "senha-errada") {
		t.Fatal("senha errada aceita")
	}
	for _, bad := range []string{"", "x", "bcrypt$1$a$b", "pbkdf2_sha256$0$a$YQ==", "pbkdf2_sha256$1$a$???"} {
		if CheckPBKDF2(bad, "x") {
			t.Errorf("hash ilegível aceito: %q", bad)
		}
	}
}

// A new hash is written in the same format and read back.
func TestHashPBKDF2VaiEVolta(t *testing.T) {
	enc, err := HashPBKDF2("uma senha longa o suficiente")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "pbkdf2_sha256$600000$") || len(strings.Split(enc, "$")) != 4 {
		t.Fatalf("formato: %q", enc)
	}
	if !CheckPBKDF2(enc, "uma senha longa o suficiente") || CheckPBKDF2(enc, "outra") {
		t.Fatal("ida e volta")
	}
	outro, _ := HashPBKDF2("uma senha longa o suficiente")
	if outro == enc {
		t.Fatal("dois hashes da mesma senha com o mesmo sal")
	}
}
