package auth

import (
	"net/http"

	"github.com/emersonjoe/trilha"
)

// One column is the most common shape of multi-tenant, and forgetting that
// column in one query is the most common bug of multi-tenant: the report that
// shows somebody else's rows, found by a customer.
//
// The framework has no ORM and should not have one. What it does have is the
// session, and the tenant is session data — so it carries the value, puts it
// where an incident is investigated from, refuses a session that has none, and
// points at the query that forgot it. The query is yours.

// Tenant is the organisation of the current session, or "" when there is no
// session or the session has no tenant.
//
//	rows, err := db.QueryContext(c.Context(), `SELECT … FROM documents WHERE tenant_id = $1`, auth.Tenant(c))
//
// Reading it is one call so that the WHERE reads like a WHERE. Nothing here
// writes SQL: a clause this package generated would be a clause nobody could
// read in a review, which is the opposite of what a tenant filter needs.
//
// It answers for the session the request went through — the shared slot, the
// only one a function with no instance can read. With two Auth in the same
// process it is the guard that ran on this route, which is the one whose
// tenant the query below is about.
func Tenant(c *trilha.Ctx) string {
	if u, ok := c.Get(ctxKey).(*User); ok && u != nil {
		return u.Tenant
	}
	return ""
}

// RequireTenant refuses a session with no organisation chosen. It goes after
// Require in the same folder, or on its own where the session is already
// guaranteed.
//
//	func Middleware(c *trilha.Ctx, next trilha.Next) error { return exige(c, next) }
//	var exige = sso.RequireTenant()
//
// Somebody logged in with no tenant is not an error and not an intruder: it is
// an administrator who has not picked an organisation, or a first login. A
// browser goes to Options.ChooseTenantPath; anything else gets 403, because a
// redirect to a screen is not an answer an API can use.
func (a *Auth) RequireTenant() trilha.MiddlewareFunc {
	return func(c *trilha.Ctx, next trilha.Next) error {
		u := a.User(c)
		if u == nil {
			return a.challenge(c)
		}
		if u.Tenant == "" {
			if dest := a.opts.ChooseTenantPath; dest != "" && wantsHTML(c.Request()) {
				return trilha.RedirectCode(dest, http.StatusFound)
			}
			return &trilha.HTTPError{Code: http.StatusForbidden, Message: "no organisation chosen"}
		}
		return next()
	}
}

// SwitchTenant moves the session to another organisation and writes it down.
//
//	if err := sso.SwitchTenant(c, id); err != nil {
//		return err
//	}
//	return c.Redirect("/")
//
// Changing organisation is a change of what the person can see, so it is
// audited with both sides: an investigation that starts with "they saw the
// wrong rows" begins by asking when they changed.
//
// It is the application's job to check that the person may enter that
// organisation. This package does not know what a membership is, and pretending
// to would be a check that looks like a guarantee and is not one.
func (a *Auth) SwitchTenant(c *trilha.Ctx, tenant string) error {
	u := a.User(c)
	if u == nil {
		return a.challenge(c)
	}
	before := u.Tenant
	if before == tenant {
		return nil
	}
	u.Tenant = tenant
	if err := a.write(c, u); err != nil {
		return err
	}
	a.remember(c, u)
	c.Audit("tenant.trocou", tenant, trilha.Fields{"de": before, "para": tenant})
	return nil
}
