// Package agentdemo is the runnable demo of the AI agent recipe, once per
// locale. The screen is an order, the chat next to it is the real ui.Chat with
// steps on, and the queue below it is the real ui.Inbox. Only the model is
// pretending: agent-demo.js answers in the browser, in ai.Serve's contract —
// tool_call, tool_result, handoff, text — so the static site shows a whole
// run, tools and all, with no key and no network.
package agentdemo

import (
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/md"
	siteui "github.com/emersonjoe/trilha/site/internal/ui"
	"github.com/emersonjoe/trilha/ui"
)

// action is the route the demo script intercepts; nothing on the server
// answers it, and the page says so.
const action = "/_demo/ai-agent"

// source is the tool the whole recipe turns on, as it is in
// examples/cookbook/aiagent.go.
const source = `// The tool that does not do what it says: cancelling is not the
// model's to do. It opens a request and tells the model so.
func AIAgentCancelTool(c *trilha.Ctx) *ai.Tool {
	return ai.NewTool("cancel_order",
		"Ask a person to cancel an order. This does not cancel anything.",
		ai.Schema(` + "`" + `{"type":"object","properties":{"order_id":{"type":"string"},"reason":{"type":"string"}},"required":["order_id","reason"]}` + "`" + `),
		ai.Typed(func(ctx context.Context, in struct {
			OrderID string ` + "`" + `json:"order_id"` + "`" + `
			Reason  string ` + "`" + `json:"reason"` + "`" + `
		}) (string, error) {
			o, ok := AIAgentOrders.Find(AIAgentWho(c), in.OrderID)
			if !ok {
				return "there is no order " + in.OrderID + " on this account", nil
			}
			id, err := trilha.Use[*approval.Approvals](c).Open(c, approval.Request{
				Kind:   AIAgentCancelKind,
				Subject: "Cancel order " + o.ID,
				Target: "/orders/" + o.ID,
				Assign: approval.Role("support"),
				Due:    time.Now().Add(AIAgentCancelDue),
				Data:   map[string]string{"order_id": o.ID, "reason": in.Reason},
			})
			if err != nil {
				return "", err
			}
			return "request " + id + " is waiting for a person to decide; " +
				"the order has not changed", nil
		}))
}`

// words is the copy of one locale.
type words struct {
	title, intro                    string
	code                            string
	order, customer, status, total  string
	customerValue, statusValue      string
	totalValue, placed, placedValue string
	greeting, placeholder, send     string
	label, try                      string
	queueTitle, queueNote           string
	queueSubject, queueKind         string
	noteTitle, noteText, read       string
	recipe, recipePath              string
}

var copy = map[string]words{
	"en": {
		title:         "An agent that acts on the app's data",
		intro:         "The same chat, with the agent's steps on the screen. Every tool call it makes is a line you can read, and the one tool that changes something does not change it: it opens a request in the queue below, where a person decides. Only the model is scripted, in the browser — this page needs no API key.",
		code:          "the tool that asks instead of doing",
		order:         "Order #1043",
		customer:      "Customer",
		status:        "Status",
		total:         "Total",
		placed:        "Placed",
		customerValue: "Ada Lovelace",
		statusValue:   "Shipped",
		totalValue:    "$32.90",
		placedValue:   "September 1, 2026",
		greeting:      "Ask where order 1043 is — or ask to cancel it, and watch what the agent does instead.",
		placeholder:   "Ask about order 1043",
		send:          "Send",
		label:         "Conversation about order 1043",
		try:           "Try: “where is my order?”, “how much was it?”, “cancel order 1043”.",
		queueTitle:    "What is waiting for a person",
		queueNote:     "The write tool opens a request here and answers the model that it asked. Approve it and the order changes — that is the only path from the conversation to the data.",
		queueSubject:  "Cancel order 1043",
		queueKind:     "order.cancel",
		noteTitle:     "What is pretending",
		noteText:      "Only the model. The component, the steps, the queue and the buttons are the real ones; there is no server behind this page, because the documentation site is static. The script answers the same events ai.Serve emits, tool calls included.",
		read:          "Read the recipe",
		recipe:        "An AI agent",
		recipePath:    "/cookbook/ai-agent",
	},
	"pt": {
		title:         "Um agente que age sobre os dados do app",
		intro:         "O mesmo chat, com os passos do agente na tela. Cada ferramenta chamada é uma linha que dá para ler, e a única que muda alguma coisa não muda: ela abre um pedido na fila abaixo, onde uma pessoa decide. Só o modelo é roteirizado, no navegador — esta página não precisa de chave de API.",
		code:          "a ferramenta que pede em vez de fazer",
		order:         "Pedido nº 1043",
		customer:      "Cliente",
		status:        "Situação",
		total:         "Total",
		placed:        "Feito em",
		customerValue: "Ada Lovelace",
		statusValue:   "Enviado",
		totalValue:    "R$ 32,90",
		placedValue:   "1º de setembro de 2026",
		greeting:      "Pergunte onde está o pedido 1043 — ou peça para cancelar, e veja o que o agente faz em vez disso.",
		placeholder:   "Pergunte sobre o pedido 1043",
		send:          "Enviar",
		label:         "Conversa sobre o pedido 1043",
		try:           "Experimente: “onde está meu pedido?”, “quanto custou?”, “cancelar o pedido 1043”.",
		queueTitle:    "O que está esperando uma pessoa",
		queueNote:     "A ferramenta de escrita abre um pedido aqui e responde ao modelo que pediu. Aprove e o pedido muda — é o único caminho da conversa até os dados.",
		queueSubject:  "Cancelar o pedido 1043",
		queueKind:     "order.cancel",
		noteTitle:     "O que está fingindo",
		noteText:      "Só o modelo. O componente, os passos, a fila e os botões são os reais; não há servidor atrás desta página, porque o site da documentação é estático. O script responde os mesmos eventos que o ai.Serve emite, chamadas de ferramenta incluídas.",
		read:          "Ler a receita",
		recipe:        "Um agente de IA",
		recipePath:    "/pt/receitas/agente-de-ia",
	},
}

// Page renders the demo in one locale.
func Page(c *trilha.Ctx, locale string) (h.Node, error) {
	w := copy[locale]
	c.SetTitle(w.title)
	siteui.SetAlternate(c, "en", "/demos/ai-agent")
	siteui.SetAlternate(c, "pt", "/pt/demos/ai-agent")

	chat := ui.ChatOpts{
		ID:          "order-agent",
		Action:      action,
		Greeting:    w.greeting,
		Placeholder: w.placeholder,
		Submit:      w.send,
		Label:       w.label,
		// Steps is what this demo adds to the chat one: the tool calls are on
		// the screen, where the person reading can see what was done for them.
		Steps:   true,
		Context: map[string]string{"order_id": "1043", "route": c.Request().URL.Path},
	}

	// The queue is the real ui.Inbox with the request the cancel tool opens.
	// It arrives hidden and the demo script reveals it, because a queue that
	// is full before anybody asked for anything teaches the wrong thing.
	queue := ui.Inbox(c, []ui.InboxRow{{
		ID: "apr_1043", Kind: w.queueKind, Subject: w.queueSubject, Target: "#",
		State: "pending", Due: time.Time{},
	}}, ui.InboxOpts{Decide: action, CSRF: h.Fragment()})

	return h.Article(h.Class("demo-page"),
		h.Header(h.Class("demo-page-header"), h.H1(h.Text(w.title)), h.P(h.Text(w.intro))),
		h.Div(h.Class("ui-body ui-stack kit chat-demo-screen"),
			ui.Card(
				ui.CardHeader(ui.CardTitle(w.order), ui.CardDescription(w.customerValue)),
				ui.CardContent(
					h.Dl(h.Class("chat-demo-facts"),
						h.Div(h.Dt(h.Text(w.customer)), h.Dd(h.Text(w.customerValue))),
						h.Div(h.Dt(h.Text(w.status)), h.Dd(h.Text(w.statusValue))),
						h.Div(h.Dt(h.Text(w.total)), h.Dd(h.Text(w.totalValue))),
						h.Div(h.Dt(h.Text(w.placed)), h.Dd(h.Text(w.placedValue))),
					),
				),
			),
			ui.Chat(c, chat),
			h.P(h.Class("agent-demo-try"), h.Text(w.try)),
			h.Section(h.ID("agent-demo-queue"), h.Class("agent-demo-queue"), h.Attr("hidden", ""),
				h.H2(h.Text(w.queueTitle)),
				h.P(h.Text(w.queueNote)),
				queue,
			),
		),
		h.Section(h.Class("chat-demo-code"),
			h.Div(h.Class("demo-rotulo"), h.Text(w.code)),
			h.Div(h.Class("codigo"), h.Data("lang", "go"),
				h.Pre(h.Code(h.Class("lang-go"), h.Raw(md.HighlightGo(source)))))),
		h.Section(h.Class("chat-demo-note"),
			h.H2(h.Text(w.noteTitle)), h.P(h.Text(w.noteText)),
			h.P(h.A(h.Href(w.recipePath), h.Text(w.read+": "+w.recipe))),
		),
		// The demo's model first, so the kit's script finds the fetch it
		// intercepts; both are deferred, and deferred scripts run in order.
		h.Script(h.Src(c.Asset("/agent-demo.js")), h.Defer()),
		ui.ChatScript(c),
	), nil
}
