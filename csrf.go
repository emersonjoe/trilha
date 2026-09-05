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

// CSRFToken returns the request's CSRF token, creating the cookie on first
// use. Put it in forms with CSRFInput or send it in the X-CSRF-Token header.
func (c *Ctx) CSRFToken() string {
	if v := c.Get("_trilha_csrf"); v != nil {
		return v.(string)
	}
	if ck, err := c.r.Cookie(CSRFCookie); err == nil && len(ck.Value) >= 32 {
		c.Set("_trilha_csrf", ck.Value)
		return ck.Value
	}
	var b [32]byte
	_, _ = rand.Read(b[:])
	tok := base64.RawURLEncoding.EncodeToString(b[:])
	http.SetCookie(c.w, &http.Cookie{
		Name:     CSRFCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	c.Set("_trilha_csrf", tok)
	return tok
}

func (c *Ctx) isSecure() bool {
	return c.r.TLS != nil || strings.EqualFold(c.r.Header.Get("X-Forwarded-Proto"), "https")
}

// CSRFInput renders the hidden input for forms: h.Form(..., trilha.CSRFInput(c), ...).
func CSRFInput(c *Ctx) h.Node {
	return h.Input(h.Type("hidden"), h.Name(CSRFField), h.Value(c.CSRFToken()))
}

// checkCSRF validates the double-submit token on state-changing requests.
func (a *App) checkCSRF(c *Ctx) error {
	ck, err := c.r.Cookie(CSRFCookie)
	if err != nil || ck.Value == "" {
		return &HTTPError{Code: http.StatusForbidden, Message: "missing CSRF cookie"}
	}
	sent := c.r.Header.Get(CSRFHeader)
	if sent == "" {
		ct := c.r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") || strings.HasPrefix(ct, "multipart/form-data") {
			if err := c.parseForm(); err != nil {
				return err
			}
			sent = c.r.PostFormValue(CSRFField)
		}
	}
	if sent == "" || subtle.ConstantTimeCompare([]byte(sent), []byte(ck.Value)) != 1 {
		return &HTTPError{Code: http.StatusForbidden, Message: "invalid CSRF token"}
	}
	return nil
}
