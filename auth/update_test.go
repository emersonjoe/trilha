package auth

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
)

// updateApp é uma app que guarda na sessão algo que muda *durante* a sessão: a
// marca da organização, que um administrador edita na própria tela. Sem uma
// porta de escrita, a cor nova só apareceria no login seguinte.
func updateApp(t *testing.T, a *Auth) *trilha.App {
	t.Helper()
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	estado := func(c *trilha.Ctx) string {
		u := a.User(c)
		return u.Extra["marca"] + " sid=" + u.SessionID + " org=" + Tenant(c)
	}
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Email: "ana@exemplo.com", Tenant: "acme",
			Extra: map[string]string{"marca": "azul"}})
	})
	route("/marca", func(c *trilha.Ctx) error { return c.Text(200, estado(c)) }, a.Require())
	route("/marca/nova", func(c *trilha.Ctx) error {
		if err := a.Update(c, func(u *User) { u.Extra["marca"] = c.Query("cor") }); err != nil {
			return err
		}
		// O resto da requisição já lê o valor novo, sem reler nada.
		return c.Text(200, estado(c))
	}, a.Require())
	// Sem guarda: é aqui que se vê o que o Update responde quando não há sessão.
	route("/tenta", func(c *trilha.Ctx) error {
		err := a.Update(c, func(u *User) { u.Extra["marca"] = "verde" })
		switch {
		case err == nil:
			return c.Text(200, "gravou")
		case errors.Is(err, ErrNoSession):
			return c.Text(200, "sem sessão")
		default:
			return c.Text(200, "erro: "+err.Error())
		}
	})
	return app
}

// O caso da issue #206: a app grava no Extra no meio de um POST e a mesma
// sessão — mesmo cookie, mesmo identificador — passa a ler o valor novo.
func TestUpdateGravaNaSessaoViva(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	b := newBrowser(t, updateApp(t, a))

	b.get("/entrar", nil)
	antes := b.get("/marca", nil).Body.String()
	cookie := b.cookies["trilha_session"]

	rec := b.get("/marca/nova?cor=verde", nil)
	if rec.Code != 200 {
		t.Fatalf("/marca/nova → %d", rec.Code)
	}
	// O handler que chamou o Update já enxerga o valor novo.
	if got := rec.Body.String(); got != "verde sid="+sid(antes)+" org=acme" {
		t.Fatalf("na própria requisição = %q (antes %q)", got, antes)
	}
	// O cookie carrega só o identificador, e o identificador não mudou: não há
	// nada para mandar ao navegador.
	if set := rec.Header().Values("Set-Cookie"); len(set) != 0 {
		t.Fatalf("Update mexeu no cookie: %q", set)
	}
	if b.cookies["trilha_session"] != cookie {
		t.Fatal("o cookie da sessão mudou")
	}
	// E a requisição seguinte, com o mesmo cookie, lê o valor novo.
	depois := b.get("/marca", nil).Body.String()
	if depois != "verde sid="+sid(antes)+" org=acme" {
		t.Fatalf("na requisição seguinte = %q", depois)
	}
}

// Sem store a sessão *é* o cookie, então o cookie é reescrito — com o mesmo
// identificador, que é o que separa o Update de um login.
func TestUpdateSemStoreGravaNoCookie(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar"})
	b := newBrowser(t, updateApp(t, a))

	b.get("/entrar", nil)
	antes := b.get("/marca", nil).Body.String()

	rec := b.get("/marca/nova?cor=verde", nil)
	if len(rec.Header().Values("Set-Cookie")) == 0 {
		t.Fatal("sem store, a sessão é o cookie: ele tinha de ser reescrito")
	}
	if got := b.get("/marca", nil).Body.String(); got != "verde sid="+sid(antes)+" org=acme" {
		t.Fatalf("depois do Update = %q (antes %q)", got, antes)
	}
}

// O Tenant é campo de primeira classe e o Update também o alcança: o que fica
// no contexto depois da escrita é o valor novo, para auth.Tenant e para a
// trilha de auditoria do resto da requisição.
func TestUpdateAlcancaOTenantEOContexto(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	app := updateApp(t, a)
	app.Register(trilha.Route{Pattern: "/mudar-org", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			if err := a.Update(c, func(u *User) { u.Tenant = "outra" }); err != nil {
				return err
			}
			return c.Text(200, "org="+Tenant(c)+" user="+a.User(c).Tenant)
		}}, Middlewares: []trilha.MiddlewareFunc{a.Require()}})
	b := newBrowser(t, app)

	b.get("/entrar", nil)
	if got := b.get("/mudar-org", nil).Body.String(); got != "org=outra user=outra" {
		t.Fatalf("no resto da requisição = %q", got)
	}
	if got := b.get("/marca", nil).Body.String(); !strings.Contains(got, "org=outra") {
		t.Fatalf("na requisição seguinte = %q", got)
	}
}

// Quem chama o Update está no meio de um POST e decide o que fazer: a falta de
// sessão é um erro devolvido, nunca um redirecionamento.
func TestUpdateSemSessaoNaoRedireciona(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	b := newBrowser(t, updateApp(t, a))

	rec := b.get("/tenta", nil)
	if got := rec.Body.String(); got != "sem sessão" {
		t.Fatalf("anônimo = %q", got)
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("virou redirect para %q", loc)
	}
}

// lojaQueNaoGrava lê como o MemoryStore e falha ao escrever: é o banco que caiu
// entre o login e o Update.
type lojaQueNaoGrava struct {
	*MemoryStore
	erro error
}

func (l *lojaQueNaoGrava) Save(id string, u *User, ttl time.Duration) error {
	if l.erro != nil {
		return l.erro
	}
	return l.MemoryStore.Save(id, u, ttl)
}

// O store que falha na escrita devolve o erro dele. Responder nil diria à app
// que o valor está gravado quando não está, e ela cacharia o que o próximo
// request não vai encontrar.
func TestUpdateDevolveOErroDoStore(t *testing.T) {
	loja := &lojaQueNaoGrava{MemoryStore: NewMemoryStore()}
	a := Sessions(Options{LoginPath: "/entrar", Store: loja})
	b := newBrowser(t, updateApp(t, a))

	b.get("/entrar", nil)
	loja.erro = errors.New("dial tcp: connection refused")
	if got := b.get("/tenta", nil).Body.String(); got != "erro: dial tcp: connection refused" {
		t.Fatalf("com o store fora do ar = %q", got)
	}
}

// Um Update sem função é bug de quem chamou, não um no-op silencioso.
func TestUpdateSemFuncaoEhErro(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	app := updateApp(t, a)
	app.Register(trilha.Route{Pattern: "/nil", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			err := a.Update(c, nil)
			if err == nil {
				return c.Text(200, "aceitou")
			}
			return c.Text(200, err.Error())
		}}, Middlewares: []trilha.MiddlewareFunc{a.Require()}})
	b := newBrowser(t, app)

	b.get("/entrar", nil)
	if got := b.get("/nil", nil).Body.String(); got == "aceitou" {
		t.Fatal("Update(c, nil) foi aceito")
	}
}

// A função recebe o Extra pronto para escrita: uma sessão que nasceu sem ele
// não obriga a app a lembrar de um make(map...) dentro do callback.
func TestUpdateEntregaOExtraPronto(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	app := updateApp(t, a)
	app.Register(trilha.Route{Pattern: "/sem-extra", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			return a.Login(c, &User{Subject: "u-2", Email: "bia@exemplo.com"}) // Extra nil
		}}})
	b := newBrowser(t, app)

	b.get("/sem-extra", nil)
	if got := b.get("/tenta", nil).Body.String(); got != "gravou" {
		t.Fatalf("sessão sem Extra = %q", got)
	}
	if got := b.get("/marca", nil).Body.String(); !strings.Contains(got, "verde") {
		t.Fatalf("depois do Update = %q", got)
	}
}

// O identificador é o nome da sessão no store: trocá-lo dentro de um Update
// deixaria a linha antiga órfã e o cookie apontando para o vazio — logout no
// meio da requisição. Quem rotaciona é o login.
func TestUpdateNaoRotacionaOIdentificador(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar", Store: NewMemoryStore()})
	app := updateApp(t, a)
	app.Register(trilha.Route{Pattern: "/tenta-rotacionar", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			if err := a.Update(c, func(u *User) { u.SessionID = "outro" }); err != nil {
				return err
			}
			return c.Text(200, a.User(c).SessionID)
		}}, Middlewares: []trilha.MiddlewareFunc{a.Require()}})
	b := newBrowser(t, app)

	b.get("/entrar", nil)
	antes := sid(b.get("/marca", nil).Body.String())
	if got := b.get("/tenta-rotacionar", nil).Body.String(); got != antes {
		t.Fatalf("identificador = %q, era %q", got, antes)
	}
	// E a sessão continua lá, com o cookie de sempre.
	if rec := b.get("/marca", nil); rec.Code != 200 {
		t.Fatalf("/marca depois → %d", rec.Code)
	}
}

// sid extrai o "sid=..." do corpo que updateApp escreve.
func sid(body string) string {
	for _, part := range strings.Fields(body) {
		if rest, ok := strings.CutPrefix(part, "sid="); ok {
			return rest
		}
	}
	return ""
}
