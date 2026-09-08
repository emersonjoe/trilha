package auth

import (
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
)

var testPolicy = Policy{
	Modules: []string{"docs", "processos", "rh"},
	Levels:  Levels{"view", "edit", "manage"},
	Roles: map[string]Grants{
		"admin":    All("manage"),
		"analista": {"docs": "edit", "processos": "view"},
		"leitor":   All("view"),
	},
}

func user(roles ...string) *User { return &User{Subject: "u1", Roles: roles} }

// The order of the levels is the whole meaning: manage covers edit covers view.
// Without that a matrix is three unrelated booleans per cell and every screen
// has to spell out which ones imply which.
func TestPolicyLevelsCoverTheOnesBelow(t *testing.T) {
	for _, c := range []struct {
		role, module, level string
		want                bool
	}{
		{"admin", "docs", "manage", true},
		{"admin", "rh", "view", true}, // All grants every module
		{"analista", "docs", "view", true},
		{"analista", "docs", "edit", true},
		{"analista", "docs", "manage", false},
		{"analista", "processos", "edit", false},
		{"analista", "rh", "view", false}, // a module the role does not name
		{"leitor", "docs", "view", true},
		{"leitor", "docs", "edit", false},
	} {
		if got := testPolicy.Can(user(c.role), c.module, c.level); got != c.want {
			t.Errorf("%s on %s/%s = %v, want %v", c.role, c.module, c.level, got, c.want)
		}
	}
}

// The answers that have to be no, because each one is a way in.
func TestPolicyDeniesWhatItDoesNotKnow(t *testing.T) {
	if testPolicy.Can(nil, "docs", "view") {
		t.Error("anonymous got through")
	}
	if testPolicy.Can(user("fantasma"), "docs", "view") {
		t.Error("a role nobody declared got through — a deleted role must lose access, not inherit")
	}
	if testPolicy.Can(user("admin"), "docs", "root") {
		t.Error("a level nobody declared got through")
	}
	if testPolicy.Can(user(), "docs", "view") {
		t.Error("a user with no role at all got through")
	}
	// A directory that hands back Admin is the same role as admin.
	if !testPolicy.Can(user("Admin"), "rh", "manage") {
		t.Error("the role did not match case-insensitively")
	}
	// Two roles: the strongest wins, and neither takes the other away.
	if !testPolicy.Can(user("leitor", "analista"), "docs", "edit") {
		t.Error("the stronger of two roles did not win")
	}
}

// Default is what a signed-in user with no known role may do, and empty — the
// value somebody gets by not writing the field — has to be nothing.
func TestPolicyDefaultIsEmptyUntilItIsNot(t *testing.T) {
	if testPolicy.Level(user("fantasma"), "docs") != "" {
		t.Error("an unknown role has a level")
	}
	open := testPolicy
	open.Default = Grants{"docs": "view"}
	if !open.Can(user("fantasma"), "docs", "view") || open.Can(user("fantasma"), "docs", "edit") {
		t.Error("Default did not grant exactly what it says")
	}
}

// The guard, through a real app: anonymous goes to the login, the signed-in
// user who is not allowed gets a 403, and the 403 says what was missing. That
// sentence is what the person repeats to whoever administers the application;
// "forbidden" turns a two-minute fix into a support thread.
func TestRequirePolicyGuardsAndSaysWhatIsMissing(t *testing.T) {
	a := Sessions(Options{LoginPath: "/entrar"})
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	ok := func(c *trilha.Ctx) error { return c.Text(200, "ok") }
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Roles: []string{"analista"}})
	})
	route("/docs", ok, a.RequirePolicy(testPolicy, "docs", "view"))
	route("/docs/nova", ok, a.RequirePolicy(testPolicy, "docs", "edit"))
	route("/rh", ok, a.RequirePolicy(testPolicy, "rh", "view"))

	b := newBrowser(t, app)
	if rec := b.get("/docs", nil); rec.Code == 200 {
		t.Fatal("anonymous read the module")
	}
	b.get("/entrar", nil)

	if rec := b.get("/docs", nil); rec.Code != 200 {
		t.Fatalf("the analyst cannot view docs: %d", rec.Code)
	}
	if rec := b.get("/docs/nova", nil); rec.Code != 200 {
		t.Fatalf("the analyst cannot edit docs, and the matrix says they can: %d", rec.Code)
	}
	rec := b.get("/rh", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("/rh → %d, want 403: known user, not allowed, and a login would loop", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "view") || !strings.Contains(body, "rh") {
		t.Fatalf("the 403 does not say what was missing: %q", body)
	}
}

// The grid and BindPolicy are two halves of one round trip: what the screen
// draws has to be what the handler reads back, and the names in between are the
// only contract holding them together.
func TestPolicyGridRoundTrip(t *testing.T) {
	grid, err := h.Render(ui.PolicyGrid(testPolicy, ui.PolicyGridOpts{Action: "/admin/permissoes"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`name="grant.analista.docs"`,         // one field per cell
		`name="grant.admin.rh"`,              //
		`action="/admin/permissoes"`,         //
		`<option value="edit" selected>edit`, // the cell shows what the role has
	} {
		if !strings.Contains(grid, want) {
			t.Errorf("the grid does not carry %q", want)
		}
	}

	// What that form posts, read back.
	form := url.Values{
		"grant.analista.docs":      {"manage"}, // promoted
		"grant.analista.processos": {""},       // taken away
		"grant.analista.rh":        {"view"},   // granted
		"grant.leitor.docs":        {"view"},
		"grant.leitor.processos":   {""},
		"grant.leitor.rh":          {""},
		// What a forged form would add, and what has to be dropped in silence:
		// answering 400 only tells whoever wrote it which name to try next.
		"grant.analista.cofre": {"manage"}, // a module nobody declared
		"grant.analista.docs2": {"manage"}, // nor this one
		"grant.invasor.docs":   {"root"},   // a level nobody declared
		"outro":                {"x"},      // not a cell at all
	}
	var roles map[string]Grants
	trilha.TestRoute(t, trilha.Route{Pattern: "/admin/permissoes", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{"POST": func(c *trilha.Ctx) error {
			var err error
			if roles, err = BindPolicy(c, testPolicy); err != nil {
				return err
			}
			return c.Text(200, "ok")
		}}}, "POST", "/admin/permissoes",
		trilha.WithForm(form), trilha.WithoutCSRF()).WantStatus(200)
	next := testPolicy
	next.Roles = roles

	if !next.Can(user("analista"), "docs", "manage") {
		t.Error("the promotion did not take")
	}
	if next.Can(user("analista"), "processos", "view") {
		t.Error("the empty cell did not take the access away")
	}
	if !next.Can(user("analista"), "rh", "view") {
		t.Error("the new grant did not take")
	}
	if next.Can(user("invasor"), "docs", "view") {
		t.Error("a level nobody declared was written down")
	}
	if _, ok := roles["analista"]["cofre"]; ok {
		t.Error("a module nobody declared was written down")
	}
	// A role with no cell at all is gone: that is how the grid deletes one.
	if _, ok := roles["admin"]; ok {
		t.Error("a role the form did not carry survived")
	}
}

// Spec 067 (#104): the session is what makes c.Audit one line. The application
// writes the sentence; who did it, from where and on which route are already
// known — and forgetting to attribute them is what every hand-rolled audit does
// in half its handlers.
func TestSessionAttributesTheAuditTrail(t *testing.T) {
	var recs []trilha.AuditRecord
	a := Sessions(Options{LoginPath: "/entrar"})
	app := trilha.New(trilha.Config{Env: trilha.Prod, Secret: []byte("0123456789abcdef0123456789abcdef"),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Audit: trilha.AuditFunc(func(r trilha.AuditRecord) error {
			recs = append(recs, r)
			return nil
		})})
	route := func(pattern string, h trilha.HandlerFunc, mw ...trilha.MiddlewareFunc) {
		app.Register(trilha.Route{Pattern: pattern, Kind: trilha.KindPage,
			Methods: map[string]trilha.HandlerFunc{"GET": h}, Middlewares: mw})
	}
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Email: "ana@exemplo.com", Roles: []string{"analista"}})
	})
	route("/excluir", func(c *trilha.Ctx) error {
		c.Audit("documento.excluiu", "42")
		return c.Text(200, "ok")
	}, a.Require())

	b := newBrowser(t, app)
	b.get("/entrar", nil)
	b.get("/excluir", nil)

	if len(recs) != 1 {
		t.Fatalf("gravou %d registros", len(recs))
	}
	if recs[0].Actor.Subject != "u-1" || recs[0].Actor.Email != "ana@exemplo.com" {
		t.Fatalf("o ator não veio da sessão: %+v", recs[0].Actor)
	}
	if recs[0].Actor.Via != "session" {
		t.Fatalf("via = %q", recs[0].Actor.Via)
	}
}
