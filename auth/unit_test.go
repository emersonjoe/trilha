package auth

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

// A path is within another by segment and never by string prefix: "sec" is not
// an ancestor of "sec-adm", and a rule that said otherwise would hand a whole
// secretariat to whoever named a unit carefully.
func TestUnitWithinComparesSegments(t *testing.T) {
	for _, c := range []struct {
		unit, ancestor string
		want           bool
	}{
		{"sec-adm/protocolo", "sec-adm", true},
		{"sec-adm/protocolo", "sec-adm/protocolo", true},
		{"sec-adm/protocolo/arquivo", "sec-adm", true},
		{"sec-adm/protocolo", "sec", false},      // not a prefix by segment
		{"sec-adm", "sec-adm/protocolo", false},  // upwards is not within
		{"sec-fin/protocolo", "sec-adm", false},  // siblings
		{"sec-adm/protocolo", "", true},          // the organisation contains everything
		{"/sec-adm/protocolo/", "sec-adm", true}, // a stray slash is the same unit
		{"sec-adm/protocolo", " sec-adm ", true}, // and so is a stray space
	} {
		if got := UnitWithin(c.unit, c.ancestor); got != c.want {
			t.Errorf("UnitWithin(%q, %q) = %v, want %v", c.unit, c.ancestor, got, c.want)
		}
	}
}

// The units travel with the session and are read the way the tenant is: by a
// function with no instance, from the slot the guard wrote.
func TestUnitsTravelInTheSession(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar"})
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Tenant: "prefeitura",
			Units: []string{"sec-adm/protocolo", "sec-fin"}})
	})
	route("/minha-unidade", func(c *trilha.Ctx) error {
		return c.Text(http.StatusOK, Unit(c)+" de "+strings.Join(Units(c), ","))
	}, a.Require())

	b := newBrowser(t, app)
	b.get("/entrar", nil)
	rec := b.get("/minha-unidade", nil)
	if got, want := rec.Body.String(), "sec-adm/protocolo de sec-adm/protocolo,sec-fin"; got != want {
		t.Fatalf("units = %q, want %q", got, want)
	}
}

// No session is no unit, and not a panic.
func TestUnitOfAnonymousIsEmpty(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar"})
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	app.Register(trilha.Route{Pattern: "/quem", Kind: trilha.KindPage,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			if Unit(c) != "" || Units(c) != nil {
				t.Errorf("anonymous has units: %q %v", Unit(c), Units(c))
			}
			return c.Text(http.StatusOK, "ok")
		}}})
	_ = a
	newBrowser(t, app).get("/quem", nil)
}
