package ui

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// OfflineScript loads the outbox: the script that registers the service
// worker of `trilha add pwa-offline`, queues a form marked with
// trilha.OfflineForm when there is no network, and sends what is queued when
// the network comes back.
//
//	ui.OfflineScript(c)
//
// The tag carries what the worker is allowed to keep: the patterns of the
// pages that declared `var Offline = true` (App.OfflineRoutes) and the version
// of the kit's stylesheet, which is the content hash — a deploy changes it,
// the worker drops the old cache, and nobody is left with a copy of last
// week's screen.
func OfflineScript(c *trilha.Ctx) h.Node {
	routes, version := "", ""
	if c != nil {
		routes = strings.Join(c.App().OfflineRoutes(), ",")
		version = assetVersionOf(c.Asset("/ui.css"))
	}
	return h.Script(
		h.Src(assetOf(c, "/ui.offline.js")), h.Defer(),
		h.Data("ui-offline-routes", routes),
		h.Data("ui-offline-version", version),
	)
}

// Outbox draws what is waiting to be sent: how many submissions the browser is
// holding, the last error the server gave back, and the button that tries
// again now. It is hidden while the outbox is empty, and the script shows it.
//
//	ui.Outbox(c)
//
// It needs OfflineScript on the same page; without JavaScript there is no
// outbox and the box never appears.
func Outbox(c *trilha.Ctx) h.Node {
	pt := langOf(c) == "pt-BR"
	return h.Div(
		h.Class("ui-outbox"), h.Data("ui-outbox", ""), h.Hidden(),
		h.Role("status"),
		h.P(h.Class("ui-outbox-count"), h.Data("ui-outbox-count", ""),
			h.Data("ui-outbox-one", word(pt, "1 pending", "1 pendente")),
			h.Data("ui-outbox-many", word(pt, "{n} pending", "{n} pendentes")),
			h.Text("")),
		h.P(h.Class("ui-outbox-error"), h.Data("ui-outbox-error", ""), h.Hidden(), h.Text("")),
		h.Button(h.Type("button"), h.Class("ui-btn"), h.Data("ui-outbox-send", ""),
			h.Text(word(pt, "Send now", "Enviar agora"))),
	)
}

// assetOf is c.Asset when there is a request, and the plain path otherwise, so
// a component renders outside one (a test, a static page) without a nil check
// at every call.
func assetOf(c *trilha.Ctx, p string) string {
	if c == nil {
		return p
	}
	return c.Asset(p)
}

// assetVersionOf is the version Asset appended to a URL ("/ui.css?v=8f3a1c92"
// → "8f3a1c92"). It is the content hash of the kit's stylesheet, which is what
// names the service worker's cache: a file that changed is a cache that is
// dropped.
func assetVersionOf(url string) string {
	_, q, found := strings.Cut(url, "?v=")
	if !found {
		return ""
	}
	v, _, _ := strings.Cut(q, "&")
	return v
}
