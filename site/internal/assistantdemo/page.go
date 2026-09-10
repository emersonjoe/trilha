// Package assistantdemo is the runnable ui.Assistant demo of the site, once per
// locale. The screen is a small invoice, the assistant is the real component
// over it, and the only thing pretending is the model: assistant-demo.js
// answers in the browser, in ai.Serve's contract, so the static site stays
// runnable and ui.chat.js remains the real client.
package assistantdemo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	siteui "github.com/emersonjoe/trilha/site/internal/ui"
	"github.com/emersonjoe/trilha/ui"
)

// action is the route the demo script intercepts; nothing on the server
// answers it, and the page says so.
const action = "/_demo/assistant"

// words is the copy of one locale.
type words struct {
	title, intro                          string
	label, dialogTitle, hint              string
	greeting, placeholder, send           string
	invoice, customer, status, total, due string
	customerValue, statusValue            string
	totalValue, dueValue                  string
	fallbackTitle, fallbackText           string
}

var copy = map[string]words{
	"en": {
		title:         "Runnable contextual assistant",
		intro:         "Ask about invoice 42. The demo uses the real Assistant, Chat, dialog and streaming client; only the model's answer is deterministic and runs in the browser.",
		label:         "Ask about this invoice",
		dialogTitle:   "Invoice assistant",
		hint:          "Asking about invoice 42",
		greeting:      "I know which invoice is open. Ask about its total, status, or due date.",
		placeholder:   "Ask about invoice 42",
		send:          "Send",
		invoice:       "Invoice #42",
		customer:      "Customer",
		status:        "Status",
		total:         "Total",
		due:           "Due date",
		customerValue: "Northwind Ltd.",
		statusValue:   "Open",
		totalValue:    "$1,284.50",
		dueValue:      "September 30, 2026",
		fallbackTitle: "Without JavaScript",
		fallbackText:  "The launcher is a link before it is a button: with the script off it comes here, to the same conversation as a page. In an application, point AssistantOpts.Page at the server-rendered conversation.",
	},
	"pt": {
		title:         "Assistente contextual executável",
		intro:         "Pergunte sobre a fatura 42. A demo usa Assistant, Chat, dialog e cliente de streaming reais; só a resposta do modelo é determinística e roda no navegador.",
		label:         "Perguntar sobre esta fatura",
		dialogTitle:   "Assistente da fatura",
		hint:          "Perguntando sobre a fatura 42",
		greeting:      "Eu sei qual fatura está aberta. Pergunte sobre o total, a situação ou o vencimento.",
		placeholder:   "Pergunte sobre a fatura 42",
		send:          "Enviar",
		invoice:       "Fatura nº 42",
		customer:      "Cliente",
		status:        "Situação",
		total:         "Total",
		due:           "Vencimento",
		customerValue: "Northwind Ltda.",
		statusValue:   "Em aberto",
		totalValue:    "R$ 1.284,50",
		dueValue:      "30 de setembro de 2026",
		fallbackTitle: "Sem JavaScript",
		fallbackText:  "O launcher é um link antes de ser um botão: com o script desligado ele vem para cá, a mesma conversa como página. Numa aplicação, aponte AssistantOpts.Page para a conversa renderizada no servidor.",
	},
}

// Page renders the demo in one locale.
func Page(c *trilha.Ctx, locale string) (h.Node, error) {
	w := copy[locale]
	c.SetTitle(w.title)
	siteui.SetAlternate(c, "en", "/demos/assistant")
	siteui.SetAlternate(c, "pt", "/pt/demos/assistant")

	// The context is what the page knows and the model does not: it rides
	// with every message, as JSON with the script and as form fields without.
	context := map[string]string{"invoice_id": "42", "route": c.Request().URL.Path}
	chat := ui.ChatOpts{Greeting: w.greeting, Placeholder: w.placeholder, Submit: w.send, Context: context}

	return h.Article(h.Class("demo-page"),
		h.Header(h.Class("demo-page-header"), h.H1(h.Text(w.title)), h.P(h.Text(w.intro))),
		h.Div(h.Class("ui-body kit assistant-demo-screen"),
			ui.Card(
				ui.CardHeader(ui.CardTitle(w.invoice), ui.CardDescription(w.customerValue)),
				ui.CardContent(
					ui.Row(ui.Badge(h.Text(w.statusValue)), h.Span(h.Text(w.customer+": "+w.customerValue))),
					h.Dl(h.Class("assistant-demo-facts"),
						h.Div(h.Dt(h.Text(w.status)), h.Dd(h.Text(w.statusValue))),
						h.Div(h.Dt(h.Text(w.total)), h.Dd(h.Text(w.totalValue))),
						h.Div(h.Dt(h.Text(w.due)), h.Dd(h.Text(w.dueValue))),
					),
				),
			),
			ui.Assistant(c, ui.AssistantOpts{
				ID:     "invoice-assistant",
				Action: action,
				Page:   "#conversation",
				Label:  w.label,
				Title:  w.dialogTitle,
				Hint:   w.hint,
				Chat:   chat,
			}),
		),
		h.Section(h.ID("conversation"), h.Class("assistant-demo-fallback"),
			h.H2(h.Text(w.fallbackTitle)), h.P(h.Text(w.fallbackText)),
			h.Div(h.Class("ui-body kit"), ui.Chat(c, withIDAndAction(chat, "invoice-conversation"))),
		),
		// The demo's answers first, so the kit's script finds the fetch it
		// intercepts; both are deferred, and deferred scripts run in order.
		h.Script(h.Src(c.Asset("/assistant-demo.js")), h.Defer()),
		ui.ChatScript(c),
	), nil
}

// withIDAndAction is the same conversation under another id: the one the
// launcher points at when the dialog cannot open.
func withIDAndAction(o ui.ChatOpts, id string) ui.ChatOpts {
	o.ID, o.Action = id, action
	return o
}
