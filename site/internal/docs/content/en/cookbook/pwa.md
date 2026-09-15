---
title: Installable app (PWA)
description: Add a manifest, icons and a progressive install invitation without pretending offline support is automatic.
---

Run the recipe from the application root:

```bash
trilha add pwa
trilha gen
```

It writes `public/manifest.webmanifest`, replaceable 192 px and 512 px PNG icons,
`public/pwa.js`, `internal/pwa/invite.go` and an `/install` help page. Put the invitation where
the product wants it, usually in the shell or settings screen:

```go
func PWAInvite(c *trilha.Ctx) h.Node {
	return ui.InstallApp(c, ui.InstallAppOpts{Help: "/install"})
}
```

The component is progressive. Without JavaScript it keeps a useful browser-menu instruction
and the help link. With `pwa.js`, Chromium receives the native `beforeinstallprompt`, iPhone
and iPad receive the Safari steps, other iOS browsers explain that installation must start in
Safari, and an already installed app sees no invitation. The browser records standalone mode
in a non-sensitive cookie, exposed on the server as `c.Standalone()`; it is a presentation
hint, never authorization or device identity.

Replace both generated icons before shipping. The manifest uses `display: "standalone"` and
the app root as `start_url`; edit those fields if the product lives below another path.

The recipe deliberately does **not** add a service worker. Installation and offline behavior
are separate product decisions: cache only the routes and assets whose staleness, logout and
tenant boundaries you can define.

Use [`ui.InstallApp`](/reference/ui#installapp) directly when the files already exist or the
copy needs to be application-specific.
