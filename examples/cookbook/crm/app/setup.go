// Package app is the embedded app's own root: the folder is the address, and
// the binary that hosts it never learns any of these names.
package app

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/cookbook/crm/internal/contacts"
	"github.com/emersonjoe/trilha/examples/cookbook/host"
)

// Config is where an embedded app says what is not its to answer for. The
// host already wrote the response headers and already published a policy with
// a nonce in it, so the app writes neither: Delegated sends none of the seven,
// and Nonce hands c.Nonce() the value the host's own policy names. The CSRF
// names move out of the way of the host's, because two hidden fields called
// _csrf on one page is a bug nobody sees until a form silently posts the wrong
// token.
func Config(cfg *trilha.Config) {
	cfg.Security.Delegated = true
	cfg.Security.Nonce = func(r *http.Request) string { return host.Nonce(r) }
	cfg.CSRF = trilha.CSRF{Cookie: "crm_csrf", Field: "_crm_csrf", Header: "X-CRM-CSRF"}
}

// Setup provides what the pages need. The store is a value, not a package
// variable: this app is one of several in the process, and Use gives each one
// back its own.
func Setup(a *trilha.App) error {
	trilha.Provide(a, contacts.New())
	return nil
}
