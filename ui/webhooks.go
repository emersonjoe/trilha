package ui

import (
	"strconv"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// WebhookRow is one subscription on the screen. It is a plain struct and not a
// webhook.Subscription because ui does not import webhook — the kit draws, and
// what it draws is data. The application maps one to the other in five lines,
// and that mapping is where "what a screen shows" is decided.
//
// The secret is not here and must not be: only its owner ever saw it, and a
// row carrying it would be a row that ends up in a log.
type WebhookRow struct {
	ID      string
	Label   string
	URL     string
	Events  []string
	Created time.Time
	Revoked bool
}

// DeliveryRow is one attempt on the screen.
type DeliveryRow struct {
	ID    string
	Event string
	// State is "pending", "delivered" or "failed".
	State   string
	Attempt int
	// Status is the last HTTP status; 0 when there was never an answer, and
	// then Err says why.
	Status int
	Err    string
	// Response is what the partner said — the first kilobyte. It is on the
	// screen because "422 field x required" is what turns an afternoon of
	// guessing into a minute of reading.
	Response string
	When     time.Time
	Next     time.Time
	RetryOf  string
}

// WebhooksOpts is what the panel can do besides list.
type WebhooksOpts struct {
	// Action is where the forms post — the panel's own address. Empty draws
	// no form and no button, which is the read-only panel.
	Action string
	// CSRF is the hidden token field, from trilha.CSRFInput(c). A form that
	// creates a subscription has to carry one; it is an option rather than
	// something taken from the Ctx so the panel stays drawable from a test,
	// and so leaving it out is a visible omission.
	CSRF h.Node
	// Events is the closed list this application emits, for the checkboxes.
	Events []string
	// Secret is the plain secret of a subscription just created, shown once.
	// webhook.TakeSecret(c) answers it, and answers "" every other time.
	Secret string
}

// WebhooksPanel is the screen an application gives whoever integrates with it:
// register an endpoint, see what was sent, send it again, test it.
//
//	ui.WebhooksPanel(c, assinaturas, entregas, ui.WebhooksOpts{
//		Action: "/webhooks", CSRF: trilha.CSRFInput(c),
//		Events: app.Hooks.Events(), Secret: webhook.TakeSecret(c),
//	})
//
// It is one form posting to one address, dispatched by a hidden action field,
// because that is what webhook.Handle answers — and because a screen that
// posts to itself comes back to itself when something is wrong, with the
// message beside the field.
func WebhooksPanel(c *trilha.Ctx, subs []WebhookRow, deliveries []DeliveryRow, o WebhooksOpts) h.Node {
	lang := langOf(c)
	w := hookWords[lang]
	n := []h.Node{h.Class("ui-webhooks")}

	// The secret first, and above everything: it is the one thing on this page
	// that cannot be looked up again.
	if o.Secret != "" {
		n = append(n, SecretOnce(c, o.Secret,
			Muted(h.Text(w["secret hint"]))))
	}
	if o.Action != "" {
		n = append(n, hookForm(o, w))
	}
	n = append(n,
		h.H3(h.Class("ui-webhooks-title"), h.Text(w["endpoints"])),
		hookTable(c, subs, o, w),
		h.H3(h.Class("ui-webhooks-title"), h.Text(w["deliveries"])),
		deliveryTable(c, deliveries, o, w),
	)
	return h.Div(n...)
}

func hookForm(o WebhooksOpts, w map[string]string) h.Node {
	campos := []h.Node{
		h.Method("post"), h.Action(o.Action), h.Class("ui-webhooks-form"),
		o.CSRF,
		h.Input(h.Type("hidden"), h.Name("action"), h.Value("subscribe")),
		Field("wh-label", w["label"], Input(h.ID("wh-label"), h.Name("label"),
			h.Attr("placeholder", w["label placeholder"]))),
		Field("wh-url", w["url"],
			Input(h.ID("wh-url"), h.Type("url"), h.Name("url"), h.Required(),
				h.Attr("placeholder", "https://"), h.Attr("inputmode", "url")),
			Help(w["url help"])),
	}
	if len(o.Events) > 0 {
		var caixas []h.Node
		for i, e := range o.Events {
			id := "wh-ev-" + strconv.Itoa(i)
			caixas = append(caixas, CheckRow(
				Checkbox(h.ID(id), h.Name("events"), h.Value(e)), e, id))
		}
		campos = append(campos, h.Fieldset(h.Class("ui-webhooks-events"),
			h.Legend(h.Text(w["events"])),
			Muted(h.Text(w["events help"])),
			h.Div(caixas...)))
	}
	campos = append(campos, Button(h.Type("submit"), h.Text(w["subscribe"])))
	return h.Form(campos...)
}

func hookTable(c *trilha.Ctx, rows []WebhookRow, o WebhooksOpts, w map[string]string) h.Node {
	if len(rows) == 0 {
		return Empty(EmptyOpts{Title: w["no endpoints"]})
	}
	var linhas []h.Node
	for _, r := range rows {
		nome := r.Label
		if nome == "" {
			nome = r.URL
		}
		eventos := w["all events"]
		if len(r.Events) > 0 {
			eventos = joinWords(r.Events)
		}
		linhas = append(linhas, h.Tr(
			h.Td(h.Text(nome), h.Br(), Muted(Code(r.URL))),
			h.Td(h.Text(eventos)),
			h.Td(Date(c, r.Created, Relative())),
			h.Td(hookState(r, w)),
			h.Td(hookButtons(r, o, w)),
		))
	}
	return h.Div(h.Class("ui-table-wrap"), h.Table(h.Class("ui-table"),
		h.Thead(h.Tr(h.Th(h.Text(w["endpoint"])), h.Th(h.Text(w["events"])),
			h.Th(h.Text(w["created"])), h.Th(h.Text(w["state"])), h.Th())),
		h.Tbody(linhas...)))
}

func hookState(r WebhookRow, w map[string]string) h.Node {
	if r.Revoked {
		return Badge(Destructive(), h.Text(w["revoked"]))
	}
	return Badge(Outline(), h.Text(w["active"]))
}

func hookButtons(r WebhookRow, o WebhooksOpts, w map[string]string) h.Node {
	if o.Action == "" || r.Revoked {
		return h.Nil
	}
	return h.Div(h.Class("ui-webhooks-actions"),
		hookAction(o, "ping", r.ID, w["ping"], nil),
		hookAction(o, "revoke", r.ID, w["revoke"], Destructive()),
	)
}

func hookAction(o WebhooksOpts, action, id, label string, variante h.Node) h.Node {
	botao := []h.Node{Ghost(), h.Type("submit"), h.Text(label)}
	if variante != nil {
		botao = []h.Node{variante, Ghost(), h.Type("submit"), h.Text(label)}
	}
	return h.Form(h.Method("post"), h.Action(o.Action), h.Class("ui-inline-form"),
		o.CSRF,
		h.Input(h.Type("hidden"), h.Name("action"), h.Value(action)),
		h.Input(h.Type("hidden"), h.Name("id"), h.Value(id)),
		Button(botao...))
}

func deliveryTable(c *trilha.Ctx, rows []DeliveryRow, o WebhooksOpts, w map[string]string) h.Node {
	if len(rows) == 0 {
		return Empty(EmptyOpts{Title: w["no deliveries"]})
	}
	var linhas []h.Node
	for _, r := range rows {
		linhas = append(linhas, h.Tr(
			h.Td(Code(r.Event), h.Br(), Muted(h.Text(r.ID))),
			h.Td(deliveryState(r, w)),
			h.Td(h.Text(strconv.Itoa(r.Attempt))),
			h.Td(deliveryAnswer(r, w)),
			h.Td(Date(c, r.When, Relative())),
			h.Td(deliveryRetry(r, o, w)),
		))
	}
	return h.Div(h.Class("ui-table-wrap"), h.Table(h.Class("ui-table"),
		h.Thead(h.Tr(h.Th(h.Text(w["event"])), h.Th(h.Text(w["state"])),
			h.Th(h.Text(w["attempts"])), h.Th(h.Text(w["answer"])),
			h.Th(h.Text(w["when"])), h.Th())),
		h.Tbody(linhas...)))
}

func deliveryState(r DeliveryRow, w map[string]string) h.Node {
	switch r.State {
	case "delivered":
		return Badge(Outline(), h.Text(w["delivered"]))
	case "failed":
		return Badge(Destructive(), h.Text(w["failed"]))
	}
	n := []h.Node{Badge(Secondary(), h.Text(w["pending"]))}
	if !r.Next.IsZero() && r.Attempt > 0 {
		// "Next attempt at 14:32" is what stops somebody pressing retry on a
		// delivery that is already going to try again on its own.
		n = append(n, h.Br(), Muted(h.Textf(w["next at"], r.Next.Format("15:04"))))
	}
	return h.Fragment(n...)
}

// deliveryAnswer shows what the other side said. The status alone is not it:
// the body is the sentence that names the field that was missing.
func deliveryAnswer(r DeliveryRow, w map[string]string) h.Node {
	if r.Status == 0 && r.Err != "" {
		return Muted(h.Text(short(r.Err, 160)))
	}
	if r.Status == 0 {
		return Muted(h.Text("—"))
	}
	n := []h.Node{h.Text(strconv.Itoa(r.Status))}
	if r.Response != "" {
		n = append(n, h.Br(), Muted(Code(short(r.Response, 160))))
	}
	return h.Fragment(n...)
}

func deliveryRetry(r DeliveryRow, o WebhooksOpts, w map[string]string) h.Node {
	// Nothing to press while it is still going: a retry on a pending delivery
	// is a second delivery of the same event.
	if o.Action == "" || r.State == "pending" {
		return h.Nil
	}
	return hookAction(o, "retry", r.ID, w["retry"], nil)
}

func short(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func joinWords(list []string) string {
	out := ""
	for i, s := range list {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

// hookWords is the text this kit writes on its own: two languages, and not a
// translation mechanism, which is the same trade the rest of the kit makes.
var hookWords = map[string]map[string]string{
	"en": {
		"endpoints": "Endpoints", "deliveries": "Deliveries",
		"endpoint": "Endpoint", "events": "Events", "created": "Registered",
		"state": "State", "event": "Event", "attempts": "Attempts",
		"answer": "Answer", "when": "When",
		"active": "Active", "revoked": "Revoked",
		"delivered": "Delivered", "failed": "Failed", "pending": "Pending",
		"next at": "next attempt at %s",
		"label":   "Name", "label placeholder": "Partner's billing system",
		"url": "URL", "url help": "https only, and it has to be reachable from the public internet.",
		"events help": "None selected means every event.",
		"all events":  "every event",
		"subscribe":   "Register", "revoke": "Revoke", "retry": "Send again", "ping": "Test",
		"no endpoints": "No endpoint registered yet.", "no deliveries": "Nothing has been sent yet.",
		"secret hint": "It signs every delivery. Store it where the receiving application reads it from; this application cannot show it again.",
	},
	"pt-BR": {
		"endpoints": "Endereços", "deliveries": "Entregas",
		"endpoint": "Endereço", "events": "Eventos", "created": "Cadastrado",
		"state": "Estado", "event": "Evento", "attempts": "Tentativas",
		"answer": "Resposta", "when": "Quando",
		"active": "Ativo", "revoked": "Revogado",
		"delivered": "Entregue", "failed": "Falhou", "pending": "Pendente",
		"next at": "próxima tentativa às %s",
		"label":   "Nome", "label placeholder": "Sistema de cobrança do parceiro",
		"url": "URL", "url help": "Só https, e precisa ser alcançável pela internet pública.",
		"events help": "Nenhum marcado significa todos os eventos.",
		"all events":  "todos os eventos",
		"subscribe":   "Cadastrar", "revoke": "Revogar", "retry": "Enviar de novo", "ping": "Testar",
		"no endpoints": "Nenhum endereço cadastrado ainda.", "no deliveries": "Nada foi enviado ainda.",
		"secret hint": "Ele assina cada entrega. Guarde onde a aplicação que recebe vai ler; esta aqui não consegue mostrar de novo.",
	},
}
