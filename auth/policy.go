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
type Grants map[string]string

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

// rankOf is the level a set of grants gives on a module, as a position.
func (p Policy) rankOf(g Grants, module string) int {
	if g == nil {
		return -1
	}
	best := -1
	if lvl, ok := g[module]; ok {
		best = p.Levels.rank(lvl)
	}
	if lvl, ok := g[allModules]; ok {
		if r := p.Levels.rank(lvl); r > best {
			best = r
		}
	}
	return best
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
			return a.challenge(c)
		}
		if !p.Can(u, module, level) {
			c.Log().Warn("auth: policy denied",
				"sub", u.Subject, "module", module, "need", level, "has", p.Level(u, module))
			return &trilha.HTTPError{
				Code:    http.StatusForbidden,
				Message: "needs " + level + " on " + module,
			}
		}
		c.Set(ctxKey, u)
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
