---
title: Notificações que respeitam a pessoa
description: trilha add notify — uma chamada que segue o canal, o horário silencioso e o digesto diário de cada pessoa, um limite por canal e a fila de onde o admin reenvia. E-mail, webhook e WhatsApp.
---

Avisar as pessoas é fácil de escrever e difícil de conviver. A primeira versão manda um e-mail de
dentro do handler. A segunda acrescenta uma preferência. Na terceira alguém recebeu quarenta
mensagens às três da manhã porque um laço tentou de novo, e ninguém sabe dizer quais saíram.

`trilha add notify` escreve a versão que aguenta: **uma chamada**, e o resto é escolha da pessoa e limite do
app.

## O comando

```bash
trilha add login notify
```

Com o [`trilha add webhooks`](/pt/receitas/webhooks) e o `trilha add channel-whatsapp` no
projeto — antes ou depois, em qualquer ordem —, os canais de webhook e de WhatsApp também ficam
ligados.

## O que instala

| Onde | O quê |
|---|---|
| `internal/notificar/` | o notificador, os canais, as preferências, a fila, o digesto |
| `migrations/0110_notify.sql` | as tabelas `notify_preferences`, `notify_outbox`, `notify_digests`, usadas quando o [`trilha add store`](/pt/receitas/banco-de-dados) está no projeto |
| `app/notificacoes/` | as preferências de cada pessoa (canal, digesto, silêncio, telefone) |
| `app/notificacoes/fila/` | a fila de saída, para o papel `admin`: o que aconteceu, reenviar, o digesto de hoje, os limites |
| `notificacoes_test.go` | os testes das telas, no seu projeto |

A chamada é a única coisa que o seu código escreve:

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

## Preço

`trilha ctx --pack notify` custa **~232 tokens (est.)**, medido num projeto mínimo pelo
`TestRecipeCtxPackCost` — o número que o `trilha add --list` mostra.

## As regras que ela guarda

**Horário silencioso.** De 22 a 7 atravessa a meia-noite; a mesma hora dos dois lados é não ter
silêncio. Uma notificação que chega dentro da janela fica `retida` e sai no próximo digesto
depois que a janela acaba. O reenvio manual respeita a janela também: 3 da manhã continua sendo
3 da manhã para a pessoa.

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

**O digesto.** Quem pediu recebe um e-mail por dia com tudo o que esperou, na hora que os
ajustes dizem, e nunca duas vezes no mesmo dia — um restart nessa hora não manda de novo. O
botão da fila manda o de hoje como [tarefa](/pt/receitas/tarefas), com a data como chave.

**O limite.** Cada canal pode mandar a cada pessoa `LimitePorHora` mensagens por hora (uma seção
de `trilha.Settings` que o admin edita na tela da fila). O que passa disso fica na fila como
`limitada` com o código [`E_NOTIFY_RATE`](/pt/docs/errors/E_NOTIFY_RATE), e o botão de reenviar
manda quando a hora vira.

**O log.** A fila mostra o título e o erro, nunca o corpo numa linha de log: o corpo é o que a
pessoa ouviu, e o log é lido por quem lê log.

## Como estender

- **Outro canal**: um `notificar.Canal` em `notificar.Canais`, no `app/setup.go`. Um provedor
  que só manda texto é `notificar.PorTexto(envia)`, que é como a linha do WhatsApp é escrita.
- **Um banco**: com o [`trilha add store`](/pt/receitas/banco-de-dados) no projeto — antes ou
  depois —, as preferências e a fila vão para as tabelas de `migrations/0110_notify.sql`
  (`internal/notificar/sql.go`), e o digesto é um por dia através de reinícios e réplicas.
  Nenhuma tela muda; o `notificartest.Contrato` segura memória e banco no mesmo comportamento.
  O limite por canal continua por processo.
