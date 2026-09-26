// Package webhooks is the screen an application gives whoever integrates with
// it: register an endpoint, see what was sent, send it again, test it.
//
// One GET and one POST. The POST is webhook.Handle, dispatched by a hidden
// action field, because register, revoke, retry and test are one screen — and
// a form that posts to itself comes back to itself when something is wrong.
package webhooks

import (
	"strconv"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
	"github.com/emersonjoe/trilha/webhook"
)

// Page lists the endpoints and the deliveries at GET /webhooks.
func Page(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Webhooks")
	hooks := trilha.Use[*webhook.Hooks](c)

	subs, err := hooks.Subscriptions(c.Context(), c.Actor().Tenant)
	if err != nil {
		return nil, err
	}
	entregas, err := hooks.Deliveries(c.Context(), webhook.ListParams{
		Tenant: c.Actor().Tenant, Limit: 20})
	if err != nil {
		return nil, err
	}
	return h.Div(
		ui.PageHeader("Webhooks", ui.Back{Href: "/documentos", Label: "Documentos"}),
		ui.Muted(h.Text("O que este blog avisa para fora. A entrega sai fora da requisição, assinada, e tenta de novo quando o outro lado está fora do ar.")),
		ui.WebhooksPanel(c, linhas(subs), entregasDe(entregas), ui.WebhooksOpts{
			Action: "/webhooks",
			CSRF:   trilha.CSRFInput(c),
			Events: hooks.Events(),
			// O segredo de uma assinatura recém-criada, uma vez só. Fora
			// desse instante isto é "".
			Secret: webhook.TakeSecret(c),
			// A data no formato do produto, o mesmo das outras telas, em vez
			// do 11/09/2026 do kit: duas formas de data na mesma página é o
			// que a opção existe para evitar (#288).
			DateFormat: diaCurto,
		}),
	), nil
}

// POST is register, revoke, retry and test, all four.
func POST(c *trilha.Ctx) error {
	return trilha.Use[*webhook.Hooks](c).Handle(c)
}

// linhas mapeia o que o módulo guarda para o que a tela mostra. São cinco
// linhas, e é aqui que "o que a tela mostra" é decidido — o kit desenha dados,
// não os tipos de outro pacote.
//
// O segredo não atravessa: a linha não tem campo para ele, o que é a razão de
// o campo não existir.
func linhas(subs []webhook.Subscription) []ui.WebhookRow {
	out := make([]ui.WebhookRow, 0, len(subs))
	for _, s := range subs {
		out = append(out, ui.WebhookRow{ID: s.ID, Label: s.Label, URL: s.URL,
			Events: s.Events, Created: s.Created, Revoked: s.Revoked})
	}
	return out
}

func entregasDe(lista []webhook.Delivery) []ui.DeliveryRow {
	out := make([]ui.DeliveryRow, 0, len(lista))
	for _, d := range lista {
		out = append(out, ui.DeliveryRow{ID: d.ID, Event: d.Event, State: string(d.State),
			Attempt: d.Attempt, Status: d.Status, Err: d.Err, Response: d.Response,
			When: d.Created, Next: d.NextTry, RetryOf: d.RetryOf})
	}
	return out
}

var meses = [...]string{"jan.", "fev.", "mar.", "abr.", "mai.", "jun.",
	"jul.", "ago.", "set.", "out.", "nov.", "dez."}

// diaCurto escreve "11 de set. de 2026", com o instante inteiro no datetime.
func diaCurto(c *trilha.Ctx, t time.Time) h.Node {
	t = t.In(c.Location())
	texto := strconv.Itoa(t.Day()) + " de " + meses[t.Month()-1] + " de " + strconv.Itoa(t.Year())
	return h.Time(h.Attr("datetime", t.Format(time.RFC3339)), h.Text(texto))
}
