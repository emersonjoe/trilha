package auth

import (
	"strings"

	"github.com/emersonjoe/trilha"
)

// An organisation is not flat. A city hall has secretariats, and a secretariat
// has departments, and a department has sectors — and the questions people
// actually ask are "does this analyst see their own sector's demands", "which
// unit does this task go to", "the secretary sees everything below them".
//
// Tenant answers none of those: it is the organisation, one column, the same
// value for everybody in the building. What follows is the unit inside it,
// carried by the session the same way — a path, so that "below me" is a
// question about a prefix and not a join nobody wrote.

// Unit is the unit of the current session — the first one — or "" when there
// is no session or the session has no unit.
//
//	rows, err := db.QueryContext(c.Context(),
//		`SELECT … FROM demands WHERE tenant_id = $1 AND unit LIKE $2`,
//		auth.Tenant(c), auth.Unit(c)+"%")
//
// It reads the session the request went through, like auth.Tenant, so a
// function with no instance can ask. Somebody who belongs to more than one
// unit is Units; this is the one the screen defaults to.
func Unit(c *trilha.Ctx) string {
	if u := units(c); len(u) > 0 {
		return u[0]
	}
	return ""
}

// Units is every unit of the current session, as paths, or nil.
//
//	for _, u := range auth.Units(c) { … }
//
// A listing filters by these; the route that receives one resource's unit
// checks it with Requirement.In instead, because a filter is not a rule.
func Units(c *trilha.Ctx) []string { return units(c) }

// units reads the shared slot, the only one a function with no instance can
// read — the same slot auth.Tenant reads, for the same reason.
func units(c *trilha.Ctx) []string {
	if u, ok := c.Get(ctxKey).(*User); ok && u != nil {
		return u.Units
	}
	return nil
}

// UnitWithin reports whether unit is ancestor or lives under it.
//
//	auth.UnitWithin("sec-adm/protocolo", "sec-adm") // true
//	auth.UnitWithin("sec-adm/protocolo", "sec")     // false
//
// The comparison is by segment and never by string prefix: "sec" is not an
// ancestor of "sec-adm", and a rule that said otherwise would hand a whole
// secretariat to whoever named a unit carefully. An empty ancestor is the
// organisation itself, which contains everything.
func UnitWithin(unit, ancestor string) bool {
	unit, ancestor = normalizeUnit(unit), normalizeUnit(ancestor)
	if ancestor == "" {
		return true
	}
	if unit == ancestor {
		return true
	}
	return strings.HasPrefix(unit, ancestor+"/")
}

// normalizeUnit is the path as this package compares it: no surrounding
// spaces, no leading or trailing slash. A value that arrived from a form as
// "/sec-adm/" is the same unit as "sec-adm", and treating it as another one
// would be a denial nobody can explain.
func normalizeUnit(p string) string {
	return strings.Trim(strings.TrimSpace(p), "/")
}

// unitIn reports whether one of the user's units is exactly this one.
func unitIn(u *User, unit string) bool {
	unit = normalizeUnit(unit)
	for _, mine := range u.Units {
		if normalizeUnit(mine) == unit {
			return true
		}
	}
	return false
}

// unitUnder reports whether the unit is one of the user's units or below it,
// which is the chief seeing what their department contains.
func unitUnder(u *User, unit string) bool {
	for _, mine := range u.Units {
		if normalizeUnit(mine) != "" && UnitWithin(unit, mine) {
			return true
		}
	}
	return false
}
