package trilha

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha/h"
)

// CSRFCookie is the name of the double-submit cookie.
const CSRFCookie = "trilha_csrf"

// CSRFField is the hidden form field name.
const CSRFField = "_csrf"

// CSRFHeader is the header accepted instead of the form field.
const CSRFHeader = "X-CSRF-Token"

// CSRF renames the three places the double-submit token appears. The empty
// value of a field means the constant above, so an app that is alone in its
// process never writes any of this. An app embedded in a host that already
// has a _csrf of its own does: two fields with the same name on one page are
// two tokens nobody reading the HTML can tell apart.
type CSRF struct {
	// Cookie is the double-submit cookie (default CSRFCookie).
	Cookie string
	// Field is the hidden form field (default CSRFField).
	Field string
	// Header is accepted instead of the field (default CSRFHeader).
	Header string
}

// names returns the CSRF names in force, with the defaults filled in. New and
// Handler resolve them once; this is the belt for an App built by hand in a
// test that never reached applyConfig.
func (c CSRF) names() CSRF {
	if c.Cookie == "" {
		c.Cookie = CSRFCookie
	}
	if c.Field == "" {
		c.Field = CSRFField
	}
	if c.Header == "" {
		c.Header = CSRFHeader
	}
	return c
}

// CSRFToken returns the request's CSRF token, creating the cookie on first
// use. Put it in forms with CSRFInput or send it in the X-CSRF-Token header.
func (c *Ctx) CSRFToken() string {
	if v := c.Get("_trilha_csrf"); v != nil {
		return v.(string)
	}
	names := c.app.cfg.CSRF.names()
	if ck, err := c.r.Cookie(names.Cookie); err == nil && len(ck.Value) >= 32 {
		c.Set("_trilha_csrf", ck.Value)
		return ck.Value
	}
	var b [32]byte
	_, _ = rand.Read(b[:])
	tok := base64.RawURLEncoding.EncodeToString(b[:])
	http.SetCookie(c.w, &http.Cookie{
		Name:     names.Cookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	c.Set("_trilha_csrf", tok)
	return tok
}

// CSRFInput renders the hidden input for forms: h.Form(..., trilha.CSRFInput(c), ...).
func CSRFInput(c *Ctx) h.Node {
	return h.Input(h.Type("hidden"), h.Name(c.app.cfg.CSRF.names().Field), h.Value(c.CSRFToken()))
}

// CSRFTokenFrom returns the CSRF token of a request Trilha is serving, for a
// renderer that only receives the *http.Request. The hidden field is named
// CSRFField unless the app renamed it in Config.CSRF:
//
//	<input type="hidden" name="_csrf" value="{{.CSRF}}">
//
// A request from outside Trilha answers "" rather than minting a token nobody
// would be able to check.
func CSRFTokenFrom(r *http.Request) string {
	if c := ctxOf(r); c != nil {
		return c.CSRFToken()
	}
	return ""
}

// checkCSRF validates the double-submit token on state-changing requests.
func (a *App) checkCSRF(c *Ctx) error {
	names := a.cfg.CSRF.names()
	ck, err := c.r.Cookie(names.Cookie)
	if err != nil || ck.Value == "" {
		a.securityEvent(c, "csrf", http.StatusForbidden)
		return &HTTPError{Code: http.StatusForbidden, Message: "missing CSRF cookie"}
	}
	sent := c.r.Header.Get(names.Header)
	if sent == "" {
		ct := c.r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data") {
			if err := c.parseForm(); err != nil {
				return err
			}
			sent = c.r.PostFormValue(names.Field)
		}
	}
	if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(ck.Value)) != 1 {
		a.securityEvent(c, "csrf", http.StatusForbidden)
		return &HTTPError{Code: http.StatusForbidden, Message: "invalid CSRF token"}
	}
	return nil
}
