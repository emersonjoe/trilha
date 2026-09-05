package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/emersonjoe/trilha"
)

// ctxKey is where the user is parked for the rest of the request.
const ctxKey = "auth.user"

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
			return a.challenge(c)
		}
		if len(roles) > 0 && !anyRole(u, roles) {
			c.Log().Warn("auth: access denied", "sub", u.Subject, "need", strings.Join(roles, ","))
			return &trilha.HTTPError{Code: http.StatusForbidden, Message: "access denied"}
		}
		c.Set(ctxKey, u)
		return next()
	}
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
	if u, ok := c.Get(ctxKey).(*User); ok {
		return u
	}
	u, err := a.Session(c)
	if err != nil {
		return nil
	}
	c.Set(ctxKey, u)
	return u
}

// Optional loads the user when there is a session and lets anonymous
// requests through, for pages that only change a greeting.
func (a *Auth) Optional() trilha.MiddlewareFunc {
	return func(c *trilha.Ctx, next trilha.Next) error {
		if u, err := a.Session(c); err == nil {
			c.Set(ctxKey, u)
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
