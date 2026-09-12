// Package chatdemo is the runnable demo of the AI chat recipe, once per
// locale. The screen is an order, the chat next to it is the real ui.Chat with
// the real ui.chat.js reading the stream, and the only thing pretending is the
// model: chat-demo.js answers in the browser, in ai.Serve's contract, so the
// static site stays runnable with no key and no network.
package chatdemo

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/site/internal/md"
	siteui "github.com/emersonjoe/trilha/site/internal/ui"
	"github.com/emersonjoe/trilha/ui"
)

// action is the route the demo script intercepts; nothing on the server
// answers it, and the page says so.
const action = "/_demo/ai-chat"

// source is the route of the recipe, as it is in examples/cookbook/aichat.go.
const source = `// app/api/chat/route.go
func POST(c *trilha.Ctx) error {
	who := AIChatWho(c)
	if ok, after := AIChatLimit.Allow(who); !ok {
		c.Header("Retry-After", strconv.Itoa(after))
		return trilha.Errorf(http.StatusTooManyRequests, "too many questions")
	}
	c.Audit("ai.chat.asked", who)
	return ai.ServeOpts{
		HTML:    ui.ChatHTML,
		Context: AIChatContext,
		Page:    AIChatAnswerPage,
	}.Serve(c, AIChatClient, AIChatAgent())
}

// the page
ui.Chat(c, ui.ChatOpts{
	Action:  "/api/chat",
	History: AIChatLog.Read(AIChatWho(c)),
	Context: map[string]string{"order_id": o.ID},
})
ui.ChatScript(c)`

// words is the copy of one locale.
type words struct {
	title, intro                     string
	code                             string
	order, customer, status, total   string
	customerValue, statusValue       string
	totalValue, placed, placedValue  string
	greeting, placeholder, send      string
	label, noteTitle, noteText, read string
	recipe, recipePath               string
}

var copy = map[string]words{
	"en": {
		title:         "An AI chat inside the app",
		intro:         "The chat of the recipe, over the record it is about. The component, the streaming client and the context the page sends are the real ones; only the model's answer is deterministic and runs in the browser, which is why this page needs no API key.",
		code:          "app/api/chat/route.go",
		order:         "Order #1043",
		customer:      "Customer",
		status:        "Status",
		total:         "Total",
		placed:        "Placed",
		customerValue: "Ada Lovelace",
		statusValue:   "Shipped",
		totalValue:    "$32.90",
		placedValue:   "September 1, 2026",
		greeting:      "Ask anything about this order — where it is, when it arrives, what it cost.",
		placeholder:   "Ask about order 1043",
		send:          "Send",
		label:         "Conversation about order 1043",
		noteTitle:     "What is pretending",
		noteText:      "Only the answer. There is no server behind this page: the documentation site is static, and the script answers the same events ai.Serve emits. In an application the form also works with JavaScript off — the same route answers the whole thing at once and the page comes back with the answer in it.",
		read:          "Read the recipe",
		recipe:        "AI chat",
		recipePath:    "/cookbook/ai-chat",
	},
	"pt": {
		title:         "Um chat de IA dentro do app",
		intro:         "O chat da receita, sobre o registro de que ele fala. O componente, o cliente de streaming e o contexto que a página envia são os reais; só a resposta do modelo é determinística e roda no navegador — por isso esta página não precisa de chave de API.",
		code:          "app/api/chat/route.go",
		order:         "Pedido nº 1043",
		customer:      "Cliente",
		status:        "Situação",
		total:         "Total",
		placed:        "Feito em",
		customerValue: "Ada Lovelace",
		statusValue:   "Enviado",
		totalValue:    "R$ 32,90",
		placedValue:   "1º de setembro de 2026",
		greeting:      "Pergunte o que quiser sobre este pedido — onde está, quando chega, quanto custou.",
		placeholder:   "Pergunte sobre o pedido 1043",
		send:          "Enviar",
		label:         "Conversa sobre o pedido 1043",
		noteTitle:     "O que está fingindo",
		noteText:      "Só a resposta. Não há servidor atrás desta página: o site da documentação é estático, e o script responde os mesmos eventos que o ai.Serve emite. Num aplicativo o formulário também funciona com o JavaScript desligado — a mesma rota responde tudo de uma vez e a página volta com a resposta dentro.",
		read:          "Ler a receita",
		recipe:        "Chat de IA",
		recipePath:    "/pt/receitas/chat-de-ia",
	},
}

// Page renders the demo in one locale.
func Page(c *trilha.Ctx, locale string) (h.Node, error) {
	w := copy[locale]
	c.SetTitle(w.title)
	siteui.SetAlternate(c, "en", "/demos/ai-chat")
	siteui.SetAlternate(c, "pt", "/pt/demos/ai-chat")

	// The context is what the page knows and the model does not: it rides
	// with every message, as JSON with the script and as form fields without.
	chat := ui.ChatOpts{
		ID:          "order-chat",
		Action:      action,
		Greeting:    w.greeting,
		Placeholder: w.placeholder,
		Submit:      w.send,
		Label:       w.label,
		Context:     map[string]string{"order_id": "1043", "route": c.Request().URL.Path},
	}

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
		),
		h.Section(h.Class("chat-demo-code"),
			h.Div(h.Class("demo-rotulo"), h.Text(w.code)),
			h.Div(h.Class("codigo"), h.Data("lang", "go"),
				h.Pre(h.Code(h.Class("lang-go"), h.Raw(md.HighlightGo(source))))),
		),
		h.Section(h.Class("chat-demo-note"),
			h.H2(h.Text(w.noteTitle)), h.P(h.Text(w.noteText)),
			h.P(h.A(h.Href(w.recipePath), h.Text(w.read+": "+w.recipe))),
		),
		// The demo's answers first, so the kit's script finds the fetch it
		// intercepts; both are deferred, and deferred scripts run in order.
		h.Script(h.Src(c.Asset("/chat-demo.js")), h.Defer()),
		ui.ChatScript(c),
	), nil
}
