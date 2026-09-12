package auth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/emersonjoe/trilha"
)

// ctxKey is where the user of the request is parked for the rest of it. It is
// the slot of an application with one Auth — the great majority — and the
// shared slot that auth.Tenant and the API keys read, because a function with
// no instance has nowhere else to look.
const ctxKey = "auth.user"

// ctxKey is where this Auth parks its user. An application with two publics in
// the same process — the internal area and the portal, routes of the same tree
// with a middleware each — gets a slot per instance, so that the other one's
// User(c) does not answer with the person this one has just let in. Without
// Options.Audience the slot is the one above and nothing changes.
func (a *Auth) ctxKey() string {
	if a.opts.Audience == "" {
		return ctxKey
	}
	return ctxKey + "." + a.opts.Audience
}

// mine reports whether a user found on the request belongs to this Auth. The
// slot already separates the instances; this separates what got into it by
// another door — the same cookie name, a shared Store, an Auth reused in the
// wrong place — and turns an identity mistake into a nil.
func (a *Auth) mine(u *User) bool { return u != nil && u.Audience == a.opts.Audience }

// Require blocks anonymous requests. A browser navigation is sent to the
// login page carrying next; anything else gets 401, because redirecting an
// API call to an HTML form only produces a confusing parse error.
//
//	// app/admin/middleware.go
//	var Middleware = sso.Require()
func (a *Auth) Require() trilha.MiddlewareFunc {
	return a.guard(nil)
}

// RequireRole blocks anyone without at least one of the roles. An
// authenticated user missing the role gets 403: they are known, just not
// allowed, and sending them back to the login page would loop.
//
//	var Middleware = sso.RequireRole("admin", "editor")
func (a *Auth) RequireRole(roles ...string) trilha.MiddlewareFunc {
	return a.guard(roles)
}

func (a *Auth) guard(roles []string) trilha.MiddlewareFunc {
	return func(c *trilha.Ctx, next trilha.Next) error {
		u, err := a.Session(c)
		if err != nil {
			return a.refuse(c, err)
		}
		if len(roles) > 0 && !anyRole(u, roles) {
			c.Log().Warn("auth: access denied", "sub", u.Subject, "need", strings.Join(roles, ","))
			return &trilha.HTTPError{Code: http.StatusForbidden, Message: "access denied"}
		}
		a.remember(c, u)
		return next()
	}
}

// refuse answers a Session that did not produce a user. Not being logged in is
// the login page; anything else is the store having failed, and sending
// somebody to the login because the database is down turns an incident into a
// login loop that no log explains.
func (a *Auth) refuse(c *trilha.Ctx, err error) error {
	if storeFailed(c, err) {
		return &trilha.HTTPError{Code: http.StatusServiceUnavailable, Message: "session store unavailable"}
	}
	return a.challenge(c)
}

// storeFailed reports whether the error is the session store breaking rather
// than there being no session, and logs it when it is. It is logged here, in
// the one place every path goes through, so that Optional and User — which
// answer "anonymous" either way — do not swallow an outage silently.
func storeFailed(c *trilha.Ctx, err error) bool {
	if err == nil || errors.Is(err, ErrNoSession) {
		return false
	}
	c.Log().Error("auth: session store failed", "err", err.Error())
	return true
}

// challenge sends the browser to the login page and everything else a 401.
func (a *Auth) challenge(c *trilha.Ctx) error {
	if !wantsHTML(c.Request()) {
		return &trilha.HTTPError{Code: http.StatusUnauthorized, Message: "not authenticated"}
	}
	dest := a.opts.LoginPath
	if next := safeNext(c.Request().URL.RequestURI()); next != "" {
		dest += "?" + url.Values{"next": {next}}.Encode()
	}
	return trilha.RedirectCode(dest, http.StatusFound)
}

// User returns the authenticated user, or nil. Handlers under Require can
// rely on it being present; anywhere else, check for nil.
func (a *Auth) User(c *trilha.Ctx) *User {
	if u, ok := c.Get(a.ctxKey()).(*User); ok && a.mine(u) {
		return u
	}
	u, err := a.Session(c)
	if err != nil {
		storeFailed(c, err)
		return nil
	}
	a.remember(c, u)
	return u
}

// Optional loads the user when there is a session and lets anonymous
// requests through, for pages that only change a greeting.
func (a *Auth) Optional() trilha.MiddlewareFunc {
	return func(c *trilha.Ctx, next trilha.Next) error {
		u, err := a.Session(c)
		if err == nil {
			a.remember(c, u)
		} else {
			// Anonymous is the point of Optional, so a broken store still lets
			// the page render — but it says so, instead of the outage looking
			// like everybody having logged out at once.
			storeFailed(c, err)
		}
		return next()
	}
}

func anyRole(u *User, roles []string) bool {
	for _, r := range roles {
		if u.HasRole(r) {
			return true
		}
	}
	return false
}

// wantsHTML reports a browser navigation, the same rule the framework uses
// to decide between an HTML error page and JSON.
func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") &&
		!strings.Contains(accept, "application/json") &&
		!strings.HasPrefix(r.URL.Path, "/api/")
}

// remember puts the session on the request and, with it, who is acting — so
// that c.Audit below this middleware is attributed without the application
// writing a line. It is one function because the same two things were being
// set in five places, and the fifth is where one of them gets forgotten.
//
// It writes two slots: the one of this instance, which is what User reads, and
// the shared one, which is what auth.Tenant reads — the tenant of the session
// the request went through, which is the only question a function with no
// instance can be asking.
func (a *Auth) remember(c *trilha.Ctx, u *User) {
	if k := a.ctxKey(); k != ctxKey {
		c.Set(k, u)
	}
	c.Set(ctxKey, u)
	c.SetActor(trilha.Actor{Subject: u.Subject, Email: u.Email, Name: u.Name, Via: "session", Tenant: u.Tenant})
}
