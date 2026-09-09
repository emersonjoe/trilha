package auth

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/emersonjoe/trilha"
)

// tenantApp é uma app de uma coluna: quem entra escolhe (ou não) a organização,
// e as telas que mostram dados exigem que ela exista.
func tenantApp(t *testing.T, a *Auth, trilhaLog *[]trilha.AuditRecord) *trilha.App {
	t.Helper()
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Audit: trilha.AuditFunc(func(r trilha.AuditRecord) error {
			*trilhaLog = append(*trilhaLog, r)
			return nil
		})})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Email: "ana@exemplo.com", Tenant: c.Query("org")})
	})
	route("/documentos", func(c *trilha.Ctx) error {
		// É assim que a consulta da app fica: uma chamada, e o WHERE é dela.
		c.Audit("documentos.listou", "todos")
		return c.Text(200, "org="+Tenant(c))
	}, a.Require(), a.RequireTenant())
	route("/trocar", func(c *trilha.Ctx) error {
		if err := a.SwitchTenant(c, c.Query("para")); err != nil {
			return err
		}
		return c.Text(200, "agora "+Tenant(c))
	}, a.Require())
	route("/escolher", func(c *trilha.Ctx) error { return c.Text(200, "escolha uma organização") }, a.Require())
	return app
}

// O tenant viaja com a sessão e chega no handler sem a app carregar nada.
func TestTenantViajaNaSessao(t *testing.T) {
	var log []trilha.AuditRecord
	a := Sessions(Options{LoginPath: "/entrar"})
	b := newBrowser(t, tenantApp(t, a, &log))

	b.get("/entrar?org=acme", nil)
	rec := b.get("/documentos", nil)
	rec2 := rec
	if rec2.Code != 200 || rec2.Body.String() != "org=acme" {
		t.Fatalf("%d %q", rec2.Code, rec2.Body.String())
	}
	// E entra na trilha, que é de onde uma investigação começa.
	if len(log) == 0 || log[len(log)-1].Actor.Tenant != "acme" {
		t.Fatalf("trilha = %+v", log)
	}
}

// Quem entrou e ainda não escolheu organização não é intruso: é um
// administrador no primeiro login. O navegador vai escolher; a API leva 403.
func TestSemOrganizacaoVaiEscolher(t *testing.T) {
	var log []trilha.AuditRecord
	a := Sessions(Options{LoginPath: "/entrar", ChooseTenantPath: "/escolher"})
	b := newBrowser(t, tenantApp(t, a, &log))

	b.get("/entrar", nil) // sem org
	rec := b.get("/documentos", http.Header{"Accept": {"text/html"}})
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/escolher" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	// Sem navegador, um redirecionamento não é resposta.
	api := b.get("/documentos", http.Header{"Accept": {"application/json"}})
	if api.Code != http.StatusForbidden {
		t.Fatalf("api = %d", api.Code)
	}
}

// Sem caminho de escolha configurado, a resposta é 403 e não um redirect para
// lugar nenhum.
func TestSemCaminhoDeEscolhaEh403(t *testing.T) {
	var log []trilha.AuditRecord
	a := Sessions(Options{LoginPath: "/entrar"})
	b := newBrowser(t, tenantApp(t, a, &log))
	b.get("/entrar", nil)
	if rec := b.get("/documentos", http.Header{"Accept": {"text/html"}}); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
	}
}

// Trocar de organização muda o que a pessoa enxerga, então fica registrado dos
// dois lados: de onde e para onde.
func TestTrocarDeOrganizacaoFicaNaTrilha(t *testing.T) {
	var log []trilha.AuditRecord
	a := Sessions(Options{LoginPath: "/entrar"})
	b := newBrowser(t, tenantApp(t, a, &log))

	b.get("/entrar?org=acme", nil)
	rec := b.get("/trocar?para=outra", nil)
	if rec.Code != 200 || rec.Body.String() != "agora outra" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	// E a sessão seguinte já está na nova.
	if got := b.get("/documentos", nil).Body.String(); got != "org=outra" {
		t.Fatalf("depois da troca: %q", got)
	}
	var troca *trilha.AuditRecord
	for i := range log {
		if log[i].Action == "tenant.trocou" {
			troca = &log[i]
		}
	}
	if troca == nil || troca.Fields["de"] != "acme" || troca.Fields["para"] != "outra" {
		t.Fatalf("trilha da troca = %+v", troca)
	}
}
