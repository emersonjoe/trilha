---
title: Cobrança sem provedor acoplado
description: trilha add billing — planos, assinaturas numa máquina de estados fechada, webhook assinado e idempotente, dunning por e-mail e as telas atrás de dois papéis. O provedor cobra o cartão; o app decide o que cada evento significa.
---

Todo produto que cobra acaba escrevendo as mesmas cinco coisas: uma tabela de planos, uma
assinatura que passa por estados, o endereço que o provedor de pagamento chama, o e-mail que sai
quando um cartão falha e uma tela para quem cuida do dinheiro. Nada disso é decisão do provedor
— ele cobra o cartão e diz o que aconteceu. O que *em atraso* significa para o seu app, quando
uma assinatura acaba, quem pode exportar as faturas: isso é do app, e é o que o
`trilha add billing` escreve.

## O comando

```bash
trilha add login connections billing
```

`billing` se apoia em duas receitas e recusa sem elas, pelo nome, antes de escrever qualquer
coisa: `login` para a sessão e os papéis, `connections` para o segredo selado com que o provedor
assina.

## O que instala

| Onde | O quê |
|---|---|
| `internal/cobranca/` | planos, assinaturas, faturas, a máquina de estados, o webhook, os lembretes |
| `migrations/0100_billing.sql` | as tabelas `billing_plans`, `billing_subscriptions`, `billing_invoices`, `billing_events`, na convenção do [`trilha add store`](/pt/receitas/banco-de-dados) |
| `app/webhooks/billing/` | `POST /webhooks/billing`, o endereço que o provedor chama — fixo, porque é digitado no painel do provedor |
| `app/billing/` | assinaturas filtradas por estado, `planos/` (criar e apagar), `faturas/` e `faturas/csv/` |
| `billing_test.go` | os testes abaixo, no seu projeto, rodados pelo `trilha check` |

Dois papéis, declarados uma vez como `auth.Policy`: `billing:reader` lê as telas,
`billing:admin` edita planos e exporta faturas. `admin` também tem o segundo nível.

```go
// Politica is who may do what with money. "admin" is here because the person
// who administers the application should not need a second role to look at
// what it charges.
var Politica = auth.Policy{
	Modules: []string{"billing"},
	Levels:  auth.Levels{"ler", "administrar"},
	Roles: map[string]auth.Grants{
		PapelLeitor: {"billing": "ler"},
		PapelAdmin:  {"billing": "administrar"},
		"admin":     {"billing": "administrar"},
	},
}
```

## Preço

`trilha ctx --pack billing` custa **~356 tokens (est.)** — o que um agente lê para saber o que a
receita trouxe, medido num projeto mínimo pelo `TestRecipeCtxPackCost`. O `trilha add --list`
mostra o mesmo número, e o selo desta página dá o preço da documentação para a qual ela aponta.

## A máquina

Nada muda o estado de uma assinatura sem passar por `Mover`, e `Mover` só permite o que esta
tabela diz. Uma assinatura cancelada não volta à vida porque um evento velho chegou atrasado.

```go
// Transicoes is the whole machine. Nothing changes a subscription's state
// without passing through Mover, and Mover only allows what is written here —
// a canceled subscription does not come back to life because an old event
// arrived late.
var Transicoes = map[Estado][]Estado{
	Trial:     {Ativa, Atrasada, Cancelada},
	Ativa:     {Atrasada, Cancelada},
	Atrasada:  {Ativa, Cancelada},
	Cancelada: nil,
}
```

Uma cobrança falha leva a assinatura para `past_due` e conta uma tentativa; uma fatura paga a
devolve para `active` e esquece as falhas; a terceira falha seguida cancela (`MaxTentativas`).
Cada falha enfileira um lembrete como [tarefa](/pt/receitas/tarefas) — o provedor espera a
resposta, e servidor de e-mail não é motivo para fazê-lo esperar —, e o terceiro lembrete diz que
a assinatura acabou.

## O endereço

O provedor assina `timestamp.body` com o segredo da conexão `billing-webhook`, em
`X-Webhook-Timestamp` e `X-Webhook-Signature` — o esquema do
[`webhook.Verify`](/pt/referencia/webhook), que confere em tempo constante e recusa um timestamp
a mais de cinco minutos de agora. Todo jeito de falhar é o mesmo `401` com o mesmo código,
[`E_BILLING_WEBHOOK_UNSIGNED`](/pt/docs/errors/E_BILLING_WEBHOOK_UNSIGNED): dizer qual parte
falhou conta a um estranho o quanto ele chegou perto.

```go
// Receber is the whole endpoint: signature over timestamp.body, the window,
// the event seen once, the change, the trail, the answer.
//
// webhook.Verify checks the signature in constant time and refuses a
// timestamp more than webhook.Tolerance (five minutes) away from now, in
// either direction — a captured request stops working before anybody could
// replay it. Every way of failing is the same 401 with the same code: saying
// which part failed tells a stranger how close they got.
func (b *Billing) Receber(c *trilha.Ctx) error {
	segredo, err := Segredo(c)
	if err != nil {
		c.Log().Warn("cobranca: webhook refused", "code", CodigoNaoAssinado, "why", "no connection")
		return naoAssinado()
	}
	corpo, err := webhook.Verify(c.Request(), segredo)
	if err != nil {
		c.Log().Warn("cobranca: webhook refused", "code", CodigoNaoAssinado)
		return naoAssinado()
	}
	var ev Evento
	if err := json.Unmarshal(corpo, &ev); err != nil || ev.ID == "" {
		return trilha.Errorf(http.StatusBadRequest, "invalid event")
	}
	// Signed and not seen before, and only then: an unsigned request must not
	// be able to burn an id a real event will carry later.
	agora := time.Now().UTC()
	visto, err := b.Store.Visto(c.Context(), ev.ID, agora)
	if err != nil {
		return err // 500: the provider tries again
	}
	if visto {
		return c.JSON(http.StatusOK, map[string]string{"status": "duplicate"})
	}
	if err := b.aplicar(c, ev, agora); err != nil {
		var falha falhaDoStore
		if errors.As(err, &falha) {
			// The store failed and wrote nothing: the id is forgotten and the
			// answer is 500, so the provider's retry is processed.
			if ferr := b.Store.Esquecer(c.Context(), ev.ID); ferr != nil {
				c.Log().Error("cobranca: event id kept after a failure", "event", ev.ID, "err", ferr)
			}
			return falha.err
		}
		// The event was understood and refused by the machine — an old event
		// arriving after a cancel, say. It is logged with the reason, and the
		// answer is still 200: retrying it would not make it valid.
		c.Log().Warn("cobranca: event not applied", "event", ev.ID, "type", ev.Tipo, "err", err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

O id do evento só é registrado depois que a assinatura confere — uma requisição sem assinatura
não pode queimar um id que um evento de verdade vai trazer depois —, e a segunda entrega do mesmo
id responde `200` sem aplicar nada. Toda mudança que mexe com dinheiro vai para a trilha de
auditoria com o id do evento e a transição.

## Como estender

- **Outro provedor**: a struct `Evento` e os quatro nomes de evento em
  `internal/cobranca/webhook.go` são o único código com a forma do provedor. Um provedor que
  assina de outro jeito é `webhook.VerifyHMAC` no lugar de `webhook.Verify` em `Receber`.
- **Um banco**: com o [`trilha add store`](/pt/receitas/banco-de-dados) no projeto — antes ou
  depois —, a linha `trilha:link billing-store` do `app/setup.go` liga o `cobranca.Banco` ao
  pool, e as linhas vão para `billing_*` (`internal/cobranca/sql.go`, SQLite ou PostgreSQL).
  Nenhuma tela muda; o `cobrancatest.Contrato` segura memória e banco no mesmo comportamento, e
  uma falha do banco no webhook responde 500 e esquece o id, para a nova tentativa do provedor
  valer.
- **Produção**: o segredo mora em `connections`, e o `trilha audit` avisa enquanto elas ficam
  em memória — um restart esqueceria o segredo e todo evento seria recusado.
