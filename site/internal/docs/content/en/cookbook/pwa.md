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

## Offline: the app shell and an outbox of forms

Two products asked for the same thing — field collection and a counter on a bad line — so
there is a second recipe, written on top of this one:

```bash
trilha add pwa-offline
trilha gen
```

It writes `public/sw.js` (the service worker), `internal/offline/offline.go`, a `/coleta`
screen with the whole pattern and the test that proves a resend does not duplicate.

**What is kept.** A page says so itself, the way `var Kind` and `var CORS` do:

```go
var Offline = true
```

`a.OfflineRoutes()` answers the patterns of the pages that declared it, sorted, and
`ui.OfflineScript(c)` writes that list into the page (`data-ui-offline-routes`) with the
content hash of the kit's stylesheet as the cache version. The worker keeps the app shell and
those routes and nothing else: an address nobody declared always goes to the network, which is
how the private area of another `Audience` never lands on the device. A response carrying
`Set-Cookie` or `Cache-Control: no-store` is never stored, and a new version drops the old
cache on activate.

**The outbox.** A form marked with `trilha.OfflineForm(c)` gets an idempotency key minted on
the server (the hidden `_idempotency_key`) and an empty `_queued_at`:

The snippets below come from the example application in `examples/blog`, where the domain
is named in Portuguese:

```go
func formulario(c *trilha.Ctx) h.Node {
	return h.Form(h.Method("post"), h.Action("/coleta"),
		trilha.CSRFInput(c),
		trilha.OfflineForm(c),
		ui.Field("texto", "Anotação", ui.Input(h.ID("texto"), h.Name("texto"), h.Required())),
		ui.Submit(h.Text("Enviar")),
	)
}
```

With `ui.OfflineScript(c)` on the page, a submit with no network goes into IndexedDB
(`trilha-outbox`) with the client's stamp in `_queued_at`, and is sent in order when the
network comes back — `POST`, `credentials: same-origin`, the key in the `Idempotency-Key`
header, the CSRF field already in the body. `ui.Outbox(c)` shows how many are waiting
("3 pending"), the last error the server gave back, and a button that tries again now.

**On the server**, the resend is recognised and answered the same way:

```go
func POST(c *trilha.Ctx) error {
	texto := c.Form("texto")
	if texto == "" {
		return trilha.Errorf(http.StatusBadRequest, "escreva alguma coisa")
	}
	repetida, err := trilha.Idempotent(c, janela)
	if err != nil {
		return err
	}
	if !repetida {
		guardar(c, texto)
	}
	return c.Redirect("/coleta")
}
```

`trilha.Idempotent` reads the key from the `Idempotency-Key` header or from the hidden field
(`trilha.IdempotencyKey(c)` is the same lookup; `trilha.NewIdempotencyKey()` mints one, and
`trilha.OfflineKeyField` and `trilha.OfflineQueuedAtField` are the field names), records it
and writes an `offline.replay` line with `c.Audit` when it has seen it before. The keys live in
`Config.Idempotency` — a `trilha.IdempotencyStore`, and nil keeps them in the process, bounded
and expiring, which is honest about one replica.

`trilha.QueuedAt(c)` is when the person pressed the button, parsed from `_queued_at`. It is a
client clock: use it as the stamp you record beside the value you kept — last write wins, with
the carimbo written down — never as the authority on ordering. That is the whole conflict
story here; a CRDT is not what a field notebook needs.

**What does not work offline**, and says so instead of pretending:

- a large upload: a form carrying a file is never queued, because the bytes do not survive the
  wait;
- an island or a fragment that needs the server to draw itself (`ui.Defer`, `ui.Live`,
  `ui.Poll`, a `Swap` target);
- any screen that did not declare `var Offline = true`, and anything under another audience's
  session;
- a resend is not a second submission: the same key answers the same thing, so a handler that
  must be repeatable is the one thing the application still owes.
