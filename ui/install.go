package ui

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// InstallAppOpts configures the progressive PWA install invitation.
type InstallAppOpts struct {
	Script          string
	Manifest        string
	Help            string
	Button          string
	Fallback        string
	IOS             string
	IOSOtherBrowser string
}

// InstallApp renders an install invitation that the recipe's pwa.js adapts to
// Chromium, iOS Safari and already-installed mode. Without JavaScript the
// fallback and Help link remain useful.
//
//	ui.InstallApp(c, ui.InstallAppOpts{Help: "/install"})
func InstallApp(c *trilha.Ctx, opts ...InstallAppOpts) h.Node {
	if c != nil && c.Standalone() {
		return nil
	}
	var o InstallAppOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	pt := langOf(c) == "pt-BR"
	if o.Script == "" {
		o.Script = "/pwa.js"
	}
	if o.Manifest == "" {
		o.Manifest = "/manifest.webmanifest"
	}
	if o.Button == "" {
		o.Button = word(pt, "Install app", "Instalar app")
	}
	if o.Fallback == "" {
		o.Fallback = word(pt, "Use your browser menu to install this app.", "Use o menu do navegador para instalar este app.")
	}
	if o.IOS == "" {
		o.IOS = word(pt, "In Safari, tap Share, then Add to Home Screen.", "No Safari, toque em Compartilhar e depois em Adicionar à Tela de Início.")
	}
	if o.IOSOtherBrowser == "" {
		o.IOSOtherBrowser = word(pt, "On iPhone or iPad, open this page in Safari to install it.", "No iPhone ou iPad, abra esta página no Safari para instalar.")
	}
	manifest := withBase(c, o.Manifest)
	script := o.Script
	if c != nil {
		script = c.Asset(o.Script)
	}
	children := []h.Node{
		h.Class("ui-install-app"), h.Data("ui-install-app", ""), h.Data("ui-install-manifest", manifest),
		h.Button(h.Type("button"), h.Class("ui-btn"), h.Data("ui-install-button", ""), h.Hidden(), h.Text(o.Button)),
		h.P(h.Data("ui-install-fallback", ""), h.Text(o.Fallback)),
		h.P(h.Data("ui-install-ios", ""), h.Hidden(), h.Text(o.IOS)),
		h.P(h.Data("ui-install-ios-other", ""), h.Hidden(), h.Text(o.IOSOtherBrowser)),
	}
	if o.Help != "" {
		children = append(children, h.A(h.Href(withBase(c, o.Help)), h.Text(word(pt, "Installation help", "Ajuda para instalar"))))
	}
	children = append(children, h.Script(h.Src(script), h.Defer()))
	return h.Div(children...)
}

func withBase(c *trilha.Ctx, path string) string {
	if c == nil || path == "" || !strings.HasPrefix(path, "/") {
		return path
	}
	return strings.TrimSuffix(c.Base(), "/") + path
}
