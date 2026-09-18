package auth

import (
	"net/http"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha"
)

// Levels are the names of the levels of access, from the weakest to the
// strongest. The order is the whole meaning: a role granted "manage" also has
// "edit" and "view", because a level covers every level before it.
//
//	auth.Levels{"view", "edit", "manage"}
//
// Three is what most applications need and what the measured one had. Two works
// ("read", "write"); so does five. What does not work is a set with no order,
// and that is why this is a slice and not a map.
type Levels []string

// rank is the position of a level, and -1 when the level is not one of these.
func (l Levels) rank(level string) int {
	for i, name := range l {
		if strings.EqualFold(name, level) {
			return i
		}
	}
	return -1
}

// Grants is what one role may do, by module. A module the role does not name is
// a module it cannot reach: the absence is a denial, never an inheritance.
//
// The value is the level, optionally followed by the scope it holds in:
// "edit" is the whole organisation, "edit/unit" is the person's own unit, and
// "edit/tree" is their unit and everything under it. Write it with Grant
// rather than by hand.
type Grants map[string]string

// Scope is how far a grant reaches inside the organisation: everywhere, the
// person's own unit, or that unit and its children.
//
// It is the second half of a cell, and not a second matrix, because the
// question a screen asks is one question — "may this role edit documents, and
// where" — and two matrices side by side are two things to keep in step.
type Scope string

const (
	// ScopeOrg is the whole organisation, which is what a grant with no scope
	// written on it means. It is the empty string so that a policy written
	// before units existed keeps meaning exactly what it meant.
	ScopeOrg Scope = ""
	// ScopeUnit is the person's own unit and nothing else: a sibling unit is
	// somebody else's, and that is the answer a demands screen needs.
	ScopeUnit Scope = "unit"
	// ScopeUnitTree is the person's unit and everything under it, which is the
	// secretary seeing what their departments do.
	ScopeUnitTree Scope = "tree"
)

// Grant is the value of one cell: a level, and the scope it holds in.
//
//	Roles: map[string]auth.Grants{
//		"secretary": {"demands": auth.Grant("manage", auth.ScopeUnitTree)},
//		"analyst":   {"demands": auth.Grant("edit", auth.ScopeUnit)},
//		"auditor":   {"demands": "view"},  // the whole organisation
//	},
//
// A level with no scope is the organisation, so the declarations that existed
// before units go on meaning what they meant.
func Grant(level string, s Scope) string {
	if level == "" || s == ScopeOrg {
		return level
	}
	return level + "/" + string(s)
}

// splitGrant reads a cell back: the level, and the scope written after the
// slash. A scope nobody declared reads as the organisation — the value came
// from a table somebody edits, and the safe reading of a word this package
// does not know is the one that changes nothing about the levels.
func splitGrant(value string) (string, Scope) {
	level, rest, ok := strings.Cut(value, "/")
	if !ok {
		return value, ScopeOrg
	}
	switch Scope(rest) {
	case ScopeUnit:
		return level, ScopeUnit
	case ScopeUnitTree:
		return level, ScopeUnitTree
	}
	return level, ScopeOrg
}

// ScopeNames is the scopes a cell may hold, in the order a select shows them.
// It is what ui.PolicyGrid asks of a policy to draw the second column of a
// cell.
func (p Policy) ScopeNames() []string {
	return []string{string(ScopeOrg), string(ScopeUnit), string(ScopeUnitTree)}
}

// Policy is the permission matrix of an application, declared once as data.
//
//	var Policy = auth.Policy{
//		Modules: []string{"docs", "processes", "hr"},
//		Levels:  auth.Levels{"view", "edit", "manage"},
//		Roles: map[string]auth.Grants{
//			"admin":   auth.All("manage"),
//			"analyst": {"docs": "edit", "processes": "view"},
//		},
//	}
//
// It is a matrix and not a list of predicates because a predicate is written
// once per screen and a matrix is read once per application. The third screen
// is where a hand-written `u.HasRole("admin") || …` starts to drift from the
// first two, and the fourth is where it is wrong.
//
// What this does not express is a rule about one record — the owner of a
// document, a row of a tenant. That stays RequireFunc, deliberately: a policy
// that tried to reach into the data would need the data, and then it would be a
// query and not a declaration.
type Policy struct {
	// Modules are the areas of the application. A module named here and never
	// required by any route is what `trilha audit` warns about.
	Modules []string
	Levels  Levels
	Roles   map[string]Grants
	// Default is what a signed-in user with no known role may do. Empty is the
	// safe answer and the one to keep: a user whose role was deleted should
	// lose access, not inherit somebody else's.
	Default Grants
}

// All grants the same level on every module, which is what an administrator
// looks like. The modules are filled in by the policy that holds it, so this
// cannot drift from the list above it.
func All(level string) Grants { return Grants{allModules: level} }

// allModules is the key All writes. It is not a module name anybody can type:
// a real one would have to survive being a URL segment.
const allModules = "*"

// Can reports whether the user may do level on module.
//
//	if acesso.Policy.Can(u, "docs", "edit") { … }
//
// A nil user cannot do anything, which is the answer for anonymous. Hiding a
// button with this is cosmetic and correct — the rule that actually holds is
// the middleware, and a page that only hides is a page that can be reached by
// typing the address.
func (p Policy) Can(u *User, module, level string) bool {
	if u == nil {
		return false
	}
	want := p.Levels.rank(level)
	if want < 0 {
		return false // a level nobody declared is not one anybody has
	}
	best := -1
	for _, role := range u.Roles {
		if got := p.rankOf(p.Roles[p.roleKey(role)], module); got > best {
			best = got
		}
	}
	if got := p.rankOf(p.Default, module); got > best {
		best = got
	}
	return best >= want
}

// roleKey finds the declared role that matches, case-insensitively: a directory
// that hands back "Admin" must not be a different role from "admin".
func (p Policy) roleKey(role string) string {
	if _, ok := p.Roles[role]; ok {
		return role
	}
	for name := range p.Roles {
		if strings.EqualFold(name, role) {
			return name
		}
	}
	return role
}

// rankOf is the level a set of grants gives on a module, as a position,
// wherever it holds: Can answers "may do it somewhere", and where is CanIn.
func (p Policy) rankOf(g Grants, module string) int {
	rank, _ := p.bestOf(g, module)
	return rank
}

// bestOf is the strongest grant a set gives on a module: its position and the
// scope it holds in. The module's own cell wins a tie against the "*" of All,
// because the cell somebody wrote about this module is the more specific
// sentence.
func (p Policy) bestOf(g Grants, module string) (int, Scope) {
	if g == nil {
		return -1, ScopeOrg
	}
	best, scope := -1, ScopeOrg
	if lvl, ok := g[allModules]; ok {
		level, s := splitGrant(lvl)
		best, scope = p.Levels.rank(level), s
	}
	if lvl, ok := g[module]; ok {
		level, s := splitGrant(lvl)
		if r := p.Levels.rank(level); r >= best {
			best, scope = r, s
		}
	}
	return best, scope
}

// rankIn is the level a set of grants gives on a module for one unit: a grant
// whose scope does not reach that unit is not a grant here at all.
func (p Policy) rankIn(g Grants, u *User, module, unit string) int {
	if g == nil {
		return -1
	}
	best := -1
	consider := func(value string) {
		level, scope := splitGrant(value)
		r := p.Levels.rank(level)
		if r <= best || r < 0 || !admits(u, scope, unit) {
			return
		}
		best = r
	}
	if lvl, ok := g[allModules]; ok {
		consider(lvl)
	}
	if lvl, ok := g[module]; ok {
		consider(lvl)
	}
	return best
}

// admits reports whether a grant of this scope reaches this unit.
func admits(u *User, s Scope, unit string) bool {
	switch s {
	case ScopeUnit:
		return unitIn(u, unit)
	case ScopeUnitTree:
		return unitUnder(u, unit)
	}
	return true // the organisation contains every unit of it
}

// CanIn reports whether the user may do level on module inside one unit.
//
//	if acesso.Policy.CanIn(u, "demands", "edit", demand.Unit) { … }
//
// It is the question Can cannot answer: Can says "somewhere", and a screen
// about one record needs "here". A grant on the whole organisation admits
// every unit; a unit grant admits the person's own units and not their
// siblings; a tree grant admits those and everything under them. Default
// counts as the organisation, because a default about one unit would be a
// default nobody could read.
//
// The guard that goes with it is Auth.Policy(...).In, which answers 403 and
// records the unit on the trail.
func (p Policy) CanIn(u *User, module, level, unit string) bool {
	if u == nil {
		return false
	}
	want := p.Levels.rank(level)
	if want < 0 {
		return false
	}
	best := -1
	for _, role := range u.Roles {
		if got := p.rankIn(p.Roles[p.roleKey(role)], u, module, unit); got > best {
			best = got
		}
	}
	if got := p.rankOf(p.Default, module); got > best {
		best = got
	}
	return best >= want
}

// ScopeOf is the scope of the grant a role holds on a module — what the grid
// draws in the second half of a cell.
func (p Policy) ScopeOf(role, module string) Scope {
	_, scope := p.bestOf(p.Roles[p.roleKey(role)], module)
	return scope
}

// ScopeNameOf is ScopeOf as the string the grid posts back, which is what
// ui.PolicyGrid asks of a policy: the kit does not import this package, and an
// interface over a named type of it would be one it could not spell.
func (p Policy) ScopeNameOf(role, module string) string {
	return string(p.ScopeOf(role, module))
}

// UnitScoped reports whether any role holds this module by unit rather than by
// organisation. It is what tells a screen to show the unit column and what
// tells `trilha audit` that a route guarding this module owes an In.
func (p Policy) UnitScoped(module string) bool {
	for role := range p.Roles {
		if p.ScopeOf(role, module) != ScopeOrg {
			return true
		}
	}
	return false
}

// Level is the level the user has on a module, or "" for none. It is what a
// screen shows next to a name, and what the grid below fills its selects with.
func (p Policy) Level(u *User, module string) string {
	if u == nil {
		return ""
	}
	best := -1
	for _, role := range u.Roles {
		if got := p.rankOf(p.Roles[p.roleKey(role)], module); got > best {
			best = got
		}
	}
	if got := p.rankOf(p.Default, module); got > best {
		best = got
	}
	if best < 0 {
		return ""
	}
	return p.Levels[best]
}

// RolesSorted is the role names in a stable order, which is what a grid needs
// to render the same way twice.
func (p Policy) RolesSorted() []string {
	out := make([]string, 0, len(p.Roles))
	for name := range p.Roles {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RequirePolicy guards a route or a whole subtree with one line.
//
//	// app/docs/middleware.go
//	var ver = acesso.Auth.RequirePolicy(acesso.Policy, "docs", "view")
//
//	func Middleware(c *trilha.Ctx, next trilha.Next) error { return ver(c, next) }
//
// Two lines and not one: the convention is a function with a fixed signature,
// and a var of the right type is not one — the scanner says so, in those words.
//
// Anonymous is sent to the login, as Require does. A signed-in user who is not
// allowed gets 403 — they are known, just not permitted, and sending them to
// the login would loop.
//
// The message says what was missing: "needs edit on docs". That sentence is
// what the person repeats to whoever administers the application, and a bare
// "forbidden" turns a two-minute fix into a support thread.
func (a *Auth) RequirePolicy(p Policy, module, level string) trilha.MiddlewareFunc {
	return func(c *trilha.Ctx, next trilha.Next) error {
		u, err := a.Session(c)
		if err != nil {
			return a.refuse(c, err)
		}
		if !p.Can(u, module, level) {
			c.Log().Warn("auth: policy denied",
				"sub", u.Subject, "module", module, "need", level, "has", p.Level(u, module))
			return &trilha.HTTPError{
				Code:    http.StatusForbidden,
				Message: "needs " + level + " on " + module,
			}
		}
		a.remember(c, u)
		return next()
	}
}

// The three methods below are what ui.PolicyGrid asks of a policy. They exist
// as an interface there, and not as an import of this package, so that the kit
// does not drag authentication into every application that draws a button.

// ModuleNames is the modules, in the order they were declared.
func (p Policy) ModuleNames() []string { return p.Modules }

// LevelNames is the levels, weakest first.
func (p Policy) LevelNames() []string { return []string(p.Levels) }

// LevelOf is the level one role has on one module, or "" for none. Unlike
// Level, it asks about a role and not about a user: it is what a grid draws in
// a cell, where there is no user yet.
func (p Policy) LevelOf(role, module string) string {
	r := p.rankOf(p.Roles[p.roleKey(role)], module)
	if r < 0 {
		return ""
	}
	return p.Levels[r]
}

// Requirement is one question about a module and a level, asked again per
// record: the route is entered once and then each demand, task or document has
// its own unit. RequirePolicy is the door of the subtree; this is the check
// inside it.
//
// It is a value and not a middleware because the unit is not known when the
// chain is built — it comes out of the record the handler has just read.
type Requirement struct {
	a      *Auth
	p      Policy
	module string
	level  string
}

// Policy is the requirement a handler checks per record.
//
//	// app/demands/id_/page.go
//	var edit = acesso.Auth.Policy(acesso.Policy, "demands", "edit")
//
//	func POST(c *trilha.Ctx) error {
//		d, err := demands.Find(c.Context(), c.Param("id"))
//		if err != nil {
//			return err
//		}
//		if err := edit.In(c, d.Unit); err != nil {
//			return err
//		}
//		…
//	}
//
// The name is Policy and not Require because Require already means "block
// anonymous" here, and two guards with one name is how a chain ends up doing
// the weaker of the two.
func (a *Auth) Policy(p Policy, module, level string) Requirement {
	return Requirement{a: a, p: p, module: module, level: level}
}

// In answers nil when the session may do this inside that unit, and otherwise
// the refusal: the login for anonymous, 403 for somebody known who is in
// another unit.
//
// The message names the unit — "needs edit on demands in sec-adm/protocolo" —
// because "forbidden" in an application with a hierarchy is a support thread
// about which of forty units the person is actually in.
//
// On the way through it writes the unit onto the actor, so every c.Audit after
// this line says which unit the action happened in without the handler
// repeating it.
func (r Requirement) In(c *trilha.Ctx, unit string) error {
	if r.a == nil {
		return &trilha.HTTPError{Code: http.StatusForbidden, Message: "no policy guard"}
	}
	u := r.a.User(c)
	if u == nil {
		return r.a.challenge(c)
	}
	if !r.p.CanIn(u, r.module, r.level, unit) {
		c.Log().Warn("auth: policy denied in unit",
			"sub", u.Subject, "module", r.module, "need", r.level, "unit", unit)
		return &trilha.HTTPError{
			Code:    http.StatusForbidden,
			Message: "needs " + r.level + " on " + r.module + " in " + unit,
		}
	}
	act := c.Actor()
	act.Unit = unit
	c.SetActor(act)
	return nil
}
