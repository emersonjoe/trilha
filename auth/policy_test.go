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

// #271 — the unit inside the organisation. The matrix is role × module ×
// level; a city hall also asks "where": the analyst edits their own sector's
// demands, the secretary sees everything under theirs, the auditor sees the
// whole organisation.
var unitPolicy = Policy{
	Modules: []string{"demandas", "relatorios"},
	Levels:  Levels{"view", "edit", "manage"},
	Roles: map[string]Grants{
		"analista":    {"demandas": Grant("edit", ScopeUnit)},
		"secretario":  {"demandas": Grant("manage", ScopeUnitTree)},
		"auditor":     {"demandas": "view"},
		"corregedor":  All(Grant("view", ScopeUnitTree)),
		"controlador": All("manage"),
	},
}

func inUnits(role string, units ...string) *User {
	return &User{Subject: "u1", Roles: []string{role}, Units: units}
}

// Grant and ScopeOf are one round trip, and a level with no scope stays what
// it always was: the whole organisation.
func TestGrantAndScopeRoundTrip(t *testing.T) {
	if got := Grant("edit", ScopeOrg); got != "edit" {
		t.Errorf("Grant(edit, org) = %q", got)
	}
	if got := Grant("edit", ScopeUnit); got != "edit/unit" {
		t.Errorf("Grant(edit, unit) = %q", got)
	}
	if got := Grant("manage", ScopeUnitTree); got != "manage/tree" {
		t.Errorf("Grant(manage, tree) = %q", got)
	}
	for _, c := range []struct {
		role, module string
		scope        Scope
	}{
		{"analista", "demandas", ScopeUnit},
		{"secretario", "demandas", ScopeUnitTree},
		{"auditor", "demandas", ScopeOrg},
		{"corregedor", "relatorios", ScopeUnitTree}, // All carries the scope too
		{"controlador", "demandas", ScopeOrg},
	} {
		if got := unitPolicy.ScopeOf(c.role, c.module); got != c.scope {
			t.Errorf("ScopeOf(%s, %s) = %q, want %q", c.role, c.module, got, c.scope)
		}
		if got := unitPolicy.ScopeNameOf(c.role, c.module); got != string(c.scope) {
			t.Errorf("ScopeNameOf(%s, %s) = %q", c.role, c.module, got)
		}
	}
	// The level is the level: the scope is not part of the name a screen shows
	// or a rank the levels are compared by.
	if got := unitPolicy.LevelOf("analista", "demandas"); got != "edit" {
		t.Errorf("LevelOf did not strip the scope: %q", got)
	}
	if got := unitPolicy.Level(inUnits("secretario", "sec-adm"), "demandas"); got != "manage" {
		t.Errorf("Level did not strip the scope: %q", got)
	}
	// Can still answers "may do it somewhere", which is what a menu asks.
	if !unitPolicy.Can(inUnits("analista", "sec-adm/protocolo"), "demandas", "edit") {
		t.Error("Can stopped answering for a scoped grant")
	}
	if !unitPolicy.UnitScoped("demandas") || unitPolicy.UnitScoped("relatorios") == false {
		t.Error("UnitScoped did not see the scoped modules")
	}
	flat := Policy{Modules: []string{"docs"}, Levels: Levels{"view"},
		Roles: map[string]Grants{"leitor": {"docs": "view"}}}
	if flat.UnitScoped("docs") {
		t.Error("a matrix with no scopes reported one")
	}
}

// The three scopes, answered per unit: a sibling is somebody else's, a unit
// grant does not inherit downwards, a tree grant does, and an organisation
// grant ignores units altogether.
func TestCanInAnswersPerUnit(t *testing.T) {
	for _, c := range []struct {
		name string
		u    *User
		unit string
		want bool
	}{
		{"own unit", inUnits("analista", "sec-adm/protocolo"), "sec-adm/protocolo", true},
		{"sibling unit", inUnits("analista", "sec-adm/protocolo"), "sec-adm/compras", false},
		{"child of own unit, unit scope", inUnits("analista", "sec-adm"), "sec-adm/protocolo", false},
		{"parent of own unit", inUnits("analista", "sec-adm/protocolo"), "sec-adm", false},
		{"child of own unit, tree scope", inUnits("secretario", "sec-adm"), "sec-adm/protocolo", true},
		{"deep child, tree scope", inUnits("secretario", "sec-adm"), "sec-adm/protocolo/arquivo", true},
		{"another secretariat, tree scope", inUnits("secretario", "sec-adm"), "sec-fin", false},
		{"org scope ignores the unit", inUnits("auditor", "sec-adm"), "sec-fin/tesouraria", true},
		{"org scope with no unit at all", inUnits("auditor"), "sec-fin", true},
		{"unit scope with no unit at all", inUnits("analista"), "sec-adm/protocolo", false},
	} {
		level := "view"
		if c.u.Roles[0] == "analista" {
			level = "edit"
		}
		if got := unitPolicy.CanIn(c.u, "demandas", level, c.unit); got != c.want {
			t.Errorf("%s: CanIn = %v, want %v", c.name, got, c.want)
		}
	}
	// Anonymous, an undeclared level and an undeclared role are no, as in Can.
	if unitPolicy.CanIn(nil, "demandas", "view", "sec-adm") {
		t.Error("anonymous got through CanIn")
	}
	if unitPolicy.CanIn(inUnits("analista", "sec-adm"), "demandas", "root", "sec-adm") {
		t.Error("a level nobody declared got through CanIn")
	}
	// Default is the organisation: a default about one unit would be a default
	// nobody could read.
	open := unitPolicy
	open.Default = Grants{"demandas": "view"}
	if !open.CanIn(inUnits("fantasma"), "demandas", "view", "sec-fin") {
		t.Error("Default did not count as the organisation")
	}
}

// The guard, through a real app: the route reads the record, and the record's
// unit is what decides. Sibling units are 403, the chief sees the child, and
// the trail says which unit it happened in.
func TestRequirementInGuardsPerUnitAndAudits(t *testing.T) {
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
	edit := a.Policy(unitPolicy, "demandas", "edit")
	route("/entrar", func(c *trilha.Ctx) error {
		return a.Login(c, &User{Subject: "u-1", Roles: []string{c.Query("papel")},
			Units: []string{c.Query("unidade")}})
	})
	// The unit of the record, which the handler has just read: the route is
	// entered once and each demand has its own.
	route("/demandas", func(c *trilha.Ctx) error {
		if err := edit.In(c, c.Query("u")); err != nil {
			return err
		}
		c.Audit("demanda.editou", "d-1")
		return c.Text(http.StatusOK, "ok")
	}, a.RequirePolicy(unitPolicy, "demandas", "edit"))

	// Anonymous is the login, not a 403: the door of the subtree answers first.
	b := newBrowser(t, app)
	if rec := b.get("/demandas?u=sec-adm/protocolo", nil); rec.Code == http.StatusOK {
		t.Fatal("anonymous edited a demand")
	}

	b.get("/entrar?papel=analista&unidade=sec-adm/protocolo", nil)
	if rec := b.get("/demandas?u=sec-adm/protocolo", nil); rec.Code != http.StatusOK {
		t.Fatalf("own unit = %d", rec.Code)
	}
	// The trail carries the unit, without the handler repeating it.
	if len(recs) != 1 || recs[0].Actor.Unit != "sec-adm/protocolo" {
		t.Fatalf("trail = %+v", recs)
	}
	// A sibling unit is somebody else's work.
	rec := b.get("/demandas?u=sec-adm/compras", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("sibling unit = %d, want 403", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "sec-adm/compras") || !strings.Contains(body, "demandas") {
		t.Fatalf("the 403 does not name the unit: %q", body)
	}
	if len(recs) != 1 {
		t.Fatalf("a refused edit was written down as an edit: %+v", recs)
	}

	// The chief of the secretariat sees what is under it — and not what is
	// under the one next door.
	b2 := newBrowser(t, app)
	b2.get("/entrar?papel=secretario&unidade=sec-adm", nil)
	if rec := b2.get("/demandas?u=sec-adm/protocolo", nil); rec.Code != http.StatusOK {
		t.Fatalf("the chief cannot see the child unit: %d", rec.Code)
	}
	if rec := b2.get("/demandas?u=sec-fin/tesouraria", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("the chief reached another secretariat: %d", rec.Code)
	}
	if got := recs[len(recs)-1].Actor.Unit; got != "sec-adm/protocolo" {
		t.Fatalf("the trail of the chief says unit %q", got)
	}
}

// The grid draws the scope beside the level for a policy that has one, and
// BindPolicy reads both back — the names in between are the whole contract.
func TestPolicyGridScopeRoundTrip(t *testing.T) {
	grid, err := h.Render(ui.PolicyGrid(unitPolicy, ui.PolicyGridOpts{Action: "/admin/permissoes"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`name="scope.analista.demandas"`,
		`<option value="unit" selected>unit`,
		`<option value="tree">unit and below`,
		`<option value="" selected>organisation`,
	} {
		if !strings.Contains(grid, want) {
			t.Errorf("the grid does not carry %q", want)
		}
	}
	// A matrix with no scopes is drawn exactly as before.
	flat, err := h.Render(ui.PolicyGrid(flatPolicy{}, ui.PolicyGridOpts{Action: "/x"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(flat, "scope.") {
		t.Error("a policy with no scopes got a scope column")
	}

	form := url.Values{
		"grant.analista.demandas": {"manage"},
		"scope.analista.demandas": {"tree"},
		"grant.auditor.demandas":  {"view"},
		"scope.auditor.demandas":  {""}, // the organisation
		"grant.leitor.demandas":   {"view"},
		"scope.leitor.demandas":   {"planeta"}, // a scope nobody declared
	}
	var roles map[string]Grants
	trilha.TestRoute(t, trilha.Route{Pattern: "/admin/permissoes", Kind: trilha.KindAPI,
		Methods: map[string]trilha.HandlerFunc{"POST": func(c *trilha.Ctx) error {
			var err error
			if roles, err = BindPolicy(c, unitPolicy); err != nil {
				return err
			}
			return c.Text(200, "ok")
		}}}, "POST", "/admin/permissoes",
		trilha.WithForm(form), trilha.WithoutCSRF()).WantStatus(200)
	next := unitPolicy
	next.Roles = roles
	if got := roles["analista"]["demandas"]; got != "manage/tree" {
		t.Fatalf("the promoted cell came back %q", got)
	}
	if got := roles["auditor"]["demandas"]; got != "view" {
		t.Fatalf("the organisation cell came back %q", got)
	}
	// A scope nobody declared changes no level: it reads as the organisation.
	if got := roles["leitor"]["demandas"]; got != "view" {
		t.Fatalf("a forged scope was written down: %q", got)
	}
	if !next.CanIn(inUnits("analista", "sec-adm"), "demandas", "manage", "sec-adm/protocolo") {
		t.Error("the new tree grant does not reach the child unit")
	}
	if next.CanIn(inUnits("analista", "sec-adm"), "demandas", "manage", "sec-fin") {
		t.Error("the new tree grant reached another secretariat")
	}
}

// flatPolicy is a matrix that knows nothing about units, which is what the
// grid has to keep drawing unchanged.
type flatPolicy struct{}

func (flatPolicy) ModuleNames() []string              { return []string{"docs"} }
func (flatPolicy) LevelNames() []string               { return []string{"view"} }
func (flatPolicy) RolesSorted() []string              { return []string{"leitor"} }
func (flatPolicy) LevelOf(role, module string) string { return "view" }
