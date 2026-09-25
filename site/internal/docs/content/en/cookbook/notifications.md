---
title: Notifications that respect the person
description: trilha add notify — one call that follows each person's channel, quiet hours and daily digest, a per-channel limit, and the outbox an administrator resends from. E-mail, webhook and WhatsApp.
---

Telling people things is easy to write and hard to live with. The first version sends an e-mail
from the handler. The second adds a preference. By the third somebody got forty messages at
three in the morning because a loop retried, and nobody can say which ones left.

`trilha add notify` writes the version that holds: **one call**, and the rest is the person's choice and the
application's limits.

## The command

```bash
trilha add login notify
```

With [`trilha add webhooks`](/cookbook/webhooks) and `trilha add channel-whatsapp` in the
project — before or after, in any order — the webhook and WhatsApp channels are wired too.

## What it installs

| Where | What |
|---|---|
| `internal/notificar/` | the notifier, the channels, the preferences, the outbox, the digest |
| `migrations/0110_notify.sql` | the tables `notify_preferences`, `notify_outbox`, `notify_digests`, used once [`trilha add store`](/cookbook/database) is in the project |
| `app/notificacoes/` | each person's preferences (channel, digest, quiet hours, phone) |
| `app/notificacoes/fila/` | the outbox, for the `admin` role: what happened, resend, today's digest, the limits |
| `notificacoes_test.go` | the tests of the screens, in your project |

The call is the only thing your code writes:

```go
// Notificar is the one call. It decides by the person's preferences and the
// channel's limit, and always leaves the notification in the outbox —
// ErrLimite when the limit held it back, the channel's error when it failed.
func (n *Notificador) Notificar(c *trilha.Ctx, para Pessoa, titulo, corpo string) (Notificacao, error) {
	ctx := ctxDe(c)
	p, err := n.Preferencias(ctx, para.Sujeito)
	if err != nil {
		return Notificacao{}, err
	}
	canal := p.Canal
	if _, ok := Canais[canal]; !ok {
		// A preference for a channel that was unwired since falls back to
		// e-mail rather than to silence.
		canal = CanalEmail
	}
	no, err := n.store.Guardar(ctx, Notificacao{Para: para, Telefone: p.Telefone, Canal: canal,
		Titulo: titulo, Corpo: corpo, Criada: n.Agora()})
	if err != nil {
		return Notificacao{}, err
	}
	switch {
	case p.Digesto:
		return n.marcar(ctx, no, NoDigesto, "")
	case p.Silencio(n.Agora().Hour()):
		return n.marcar(ctx, no, Retida, "")
	}
	return n.entregar(c, no)
}
```

## Price

`trilha ctx --pack notify` costs **~232 tokens (est.)**, measured on a minimal project by
`TestRecipeCtxPackCost` — the number `trilha add --list` shows.

## The rules it keeps

**Quiet hours.** From 22 to 7 wraps midnight; the same hour on both sides is no quiet hours at
all. A notification that arrives inside the window is `retida` — held — and goes out with the
next digest after the window ends. The manual resend respects the window too: 3 a.m. is still
3 a.m. for the person.

```go
// Silencio says whether hour falls in the quiet hours. From 22 to 7 wraps
// midnight; from and to equal is no quiet hours at all.
func (p Preferencias) Silencio(hora int) bool {
	switch {
	case p.SilencioDe == p.SilencioAte:
		return false
	case p.SilencioDe < p.SilencioAte:
		return hora >= p.SilencioDe && hora < p.SilencioAte
	default:
		return hora >= p.SilencioDe || hora < p.SilencioAte
	}
}
```

**The digest.** Whoever asked for it receives one e-mail a day with everything that waited,
at the hour the settings say, and never twice on the same day — a restart at that hour does not
send it again. The outbox button sends today's as a [task](/cookbook/tasks), with the date as
the key.

**The limit.** Each channel may send each person `LimitePorHora` messages an hour (a
`trilha.Settings` section an administrator edits on the outbox screen). What goes over stays in
the outbox as `limitada` with the code
[`E_NOTIFY_RATE`](/docs/errors/E_NOTIFY_RATE), and the resend button sends it once the hour
turns.

**The log.** The outbox shows the title and the error, never the body in a log line: the body
is what the person was told, and a log is read by whoever reads logs.

## Extending it

- **Another channel**: a `notificar.Canal` in `notificar.Canais`, in `app/setup.go`. A provider
  that only sends text is `notificar.PorTexto(send)`, which is how the WhatsApp line is written.
- **A database**: with [`trilha add store`](/cookbook/database) in the project — before or
  after — the preferences and the outbox move to the tables of `migrations/0110_notify.sql`
  (`internal/notificar/sql.go`), and the digest is once a day across restarts and replicas.
  Nothing on the screens changes; `notificartest.Contrato` holds memory and database to the
  same behaviour. The per-channel limit stays per process.
