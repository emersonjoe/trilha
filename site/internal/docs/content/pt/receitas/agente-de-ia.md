---
title: Um agente de IA
description: Um agente que age sobre os dados do app — ferramentas que leem o que a pessoa pode ler, uma escrita que espera alguém decidir, dois agentes, os passos na tela, o rastro, um teste roteirizado e o mesmo conjunto no MCP.
---

A receita do [chat de IA](/pt/receitas/chat-de-ia) coloca uma conversa dentro do app. Esta é a
pergunta do segundo dia: o assistente para de responder e começa a *fazer* — procurar um
pedido, ler a API, pedir um cancelamento. Quatro coisas precisam ser verdade antes que isso
seja uma boa ideia, e nenhuma página as diz juntas:

- ele lê **o que a pessoa que perguntou pode ler**, e mais nada;
- o que ele pode **fazer sozinho** está separado do que **espera uma pessoa**;
- o que ele fez fica **escrito**, ferramenta por ferramenta;
- e dá para **testar tudo isso** sem gastar chamada.

O domínio é pequeno e o mesmo da receita do chat: pedidos e clientes de uma loja. Todo bloco
abaixo é uma declaração de
[`examples/cookbook/aiagent.go`](https://github.com/emersonjoe/trilha/blob/main/examples/cookbook/aiagent.go),
que compila e é testado com o resto do repositório. Ela termina em
[uma demo](/pt/demos/ai-agent), onde uma corrida inteira — o repasse, as ferramentas, a fila —
acontece na sua frente.

## 1. Ferramentas que leem

Uma ferramenta é uma função com nome, descrição e um schema JSON. O que a torna *sua* é o
fecho em volta dela: a requisição está dentro, então o visitante está dentro.

```go
// AIAgentWho is who is asking: the signed-in visitor, or the address when
// there is nobody. It is the customer of every query and the tenant of the
// index — one answer, in one place, so no tool can be written without it.
func AIAgentWho(c *trilha.Ctx) string {
	if a := c.Actor(); a.Subject != "" {
		return a.Subject
	}
	return c.ClientIP()
}
```

O índice é o do próprio app, o mesmo da caixa de busca. O recorte é do índice, não da
ferramenta:

```go
// AIAgentSearch is the app's index, the one the search box on the site header
// already uses. The tenant of a document is the customer it belongs to, so a
// query is scoped by the index and not by a filter every tool has to remember
// to write.
var AIAgentSearch = trilha.NewSearch(trilha.SearchOpts{Tenant: AIAgentWho}).
```

```go
// AIAgentSearchTool is the first tool: it reads. The visitor is inside the
// closure and not in the arguments, so there is no id the model can send that
// widens what it sees — the agent reads what the person would read.
func AIAgentSearchTool(c *trilha.Ctx) *ai.Tool {
	return ai.NewTool("search_orders",
		"Find the customer's own orders by words: an order number, a status, a date.",
		ai.Schema(`{"type":"object","properties":{"query":{"type":"string","description":"words to look for"}},"required":["query"]}`),
		ai.Typed(func(ctx context.Context, in struct{ Query string }) (string, error) {
			res, err := AIAgentSearch.Query(c, in.Query, trilha.SearchQuery{Limit: AIAgentHits})
			if err != nil {
				return "", err
			}
			if res.Total == 0 {
				return "nothing found for " + strconv.Quote(in.Query), nil
			}
			var b strings.Builder
			for _, g := range res.Groups {
				for _, hit := range g.Hits {
					fmt.Fprintf(&b, "%s %s: %s (%s)\n", g.Kind, hit.ID, hit.Title, hit.URL)
				}
			}
			return b.String(), nil
		}))
}
```

Leia o fecho outra vez: **quem chamou não é argumento**. Não existe id que o modelo possa
inventar para ver mais, porque a única coisa que ele manda são palavras para procurar. Esse é
o modelo de permissão inteiro de uma ferramenta de leitura — o [`search`](/pt/referencia/search)
faz o resto, e um tipo negado não deixa nem o número na contagem.

### A API que você já tem

A segunda ferramenta lê um pedido inteiro, e já existe uma rota que faz isso. Ela não é
declarada de novo: ela é *chamada*, como requisição, com a credencial de quem perguntou.

```go
// AIAgentOrderGET is app/api/orders/[id]/route.go, the app's own API, written
// for people before it was written for agents. The recipe does not declare it
// a second time.
func AIAgentOrderGET(c *trilha.Ctx) error {
	o, ok := AIAgentOrders.Find(AIAgentWho(c), c.Param("id"))
	if !ok {
		return trilha.ErrNotFound
	}
	c.Audit("order.read", o.ID)
	return c.JSON(http.StatusOK, map[string]string{
		"id": o.ID, "status": o.Status, "total": o.Total,
		"placed": o.Placed.Format("2006-01-02"),
	})
}
```

```go
// AIAgentSignedIn is the chain that route already had. Probe runs it — that is
// the whole point of the tool below.
func AIAgentSignedIn(c *trilha.Ctx, next trilha.Next) error {
	if c.Actor().Subject == "" {
		return trilha.Errorf(http.StatusForbidden, "sign in to see your orders")
	}
	return next()
}
```

```go
// AIAgentAPITool is the second tool, and it is the same API. The route is
// called as a request — with the caller's own credential, marked as arriving
// from the agent — and App.Probe asks the route's chain first, so a caller who
// may not reach the handler gets a tool that says no instead of a handler that
// ran. It is cookbook/api-as-tools seen from the agent's side.
func AIAgentAPITool(c *trilha.Ctx) *ai.Tool {
	return ai.NewTool("get_order",
		"Read one order in full: status, total and the date it was placed.",
		ai.Schema(`{"type":"object","properties":{"id":{"type":"string","description":"the order number"}},"required":["id"]}`),
		ai.Typed(func(ctx context.Context, in struct{ ID string }) (string, error) {
			req, err := http.NewRequestWithContext(ctx, "GET", "/api/orders/"+url.PathEscape(in.ID), nil)
			if err != nil {
				return "", err
			}
			req.Header.Set("Accept", "application/json")
			for _, name := range []string{"Authorization", "Cookie", "Accept-Language", "X-Request-ID"} {
				if v := c.Request().Header.Get(name); v != "" {
					req.Header.Set(name, v)
				}
			}
			req.Host, req.RemoteAddr = c.Request().Host, c.Request().RemoteAddr
			req = trilha.WithVia(req, "agent")
			if !c.App().Probe(req) {
				return "you may not read that order", nil
			}
			rec := &AIAgentRecorder{Status: http.StatusOK}
			c.App().Handler().ServeHTTP(rec, req)
			body := strings.TrimSpace(rec.Body.String())
			if rec.Status >= 400 {
				return "the API answered " + strconv.Itoa(rec.Status) + ": " + body, nil
			}
			return body, nil
		}))
}
```

Duas linhas carregam a receita. O `App.Probe` roda a cadeia de middleware da rota — a sessão,
a chave, a política — **sem rodar o handler**, e responde se aquela pessoa passaria; quem não
passaria recebe uma ferramenta que diz não, em vez de um handler que rodou. O
`trilha.WithVia(req, "agent")` marca como a chamada chegou, então o registro de auditoria que
o handler escreve diz `agent` e não `session`: seis meses depois, "o que o assistente fez" é um
filtro e não uma investigação.

O gravador é a ponte inteira:

```go
// AIAgentRecorder collects what a route answers when the caller is a tool and
// not a socket. Fifteen lines instead of a second handler: whatever the route
// answers a browser is what the model reads.
type AIAgentRecorder struct {
	Status int
	Body   bytes.Buffer

	header http.Header
}
```

:::note
Isto é a [sua API como ferramentas](/pt/receitas/api-como-ferramentas) vista do outro lado. Lá,
o `mcp.FromRoutes` transforma as rotas em ferramentas para o agente de outra pessoa, e o
`Probe` decide o que cada chave enxerga. Aqui o agente é seu e a ferramenta é escrita à mão,
porque é uma rota e uma frase que o modelo lê — mas a primitiva é a mesma.
:::

## 2. Uma ferramenta que escreve, com uma pessoa no meio

"Cancele o pedido" é onde toda demo de agente vira, em silêncio, uma má ideia. A resposta da
receita é que a ferramenta não cancela nada: ela abre um pedido, e diz que abriu.

```go
// AIAgentCancelTool is the third tool, and the one that does not do what it
// says: cancelling an order is not the model's to do. It opens a request in
// the queue, tells the model that a person has it, and changes nothing. What
// the model reads back is the truth, which is what keeps it from telling the
// customer the order is cancelled.
func AIAgentCancelTool(c *trilha.Ctx) *ai.Tool {
	return ai.NewTool("cancel_order",
		"Ask a person to cancel an order. This does not cancel anything: it opens a request somebody from support decides.",
		ai.Schema(`{"type":"object","properties":{"order_id":{"type":"string"},"reason":{"type":"string","description":"what the customer said"}},"required":["order_id","reason"]}`),
		// The tag is not decoration: the argument the model sends is the name
		// in the schema, and ai.Typed hands it to encoding/json.
		ai.Typed(func(ctx context.Context, in struct {
			OrderID string `json:"order_id"`
			Reason  string `json:"reason"`
		}) (string, error) {
			o, ok := AIAgentOrders.Find(AIAgentWho(c), in.OrderID)
			if !ok {
				return "there is no order " + in.OrderID + " on this account", nil
			}
			id, err := trilha.Use[*approval.Approvals](c).Open(c, approval.Request{
				Kind:    AIAgentCancelKind,
				Subject: "Cancel order " + o.ID,
				Target:  "/orders/" + o.ID,
				Assign:  approval.Role("support"),
				Due:     time.Now().Add(AIAgentCancelDue),
				Data:    map[string]string{"order_id": o.ID, "customer": o.Customer, "reason": in.Reason},
			})
			if err != nil {
				return "", err
			}
			return "request " + id + " is waiting for a person to decide; the order has not changed", nil
		}))
}
```

O que o modelo lê de volta é a verdade — *esperando*, *não mudou* —, e é isso que o impede de
dizer ao cliente que o pedido foi cancelado. A frase na descrição da ferramenta também não é
enfeite: a descrição é o único lugar onde o modelo aprende que essa ferramenta pede em vez de
fazer.

A decisão é o único caminho da conversa até os dados:

```go
// AIAgentCancelled is what the decision means, and the only path from the
// conversation to the data. It runs after the decision is written, so a
// failure here does not undo somebody's choice.
func AIAgentCancelled(c *trilha.Ctx, r approval.Record) error {
	if r.State != approval.Approved {
		return nil
	}
	return AIAgentOrders.Cancel(r.Data["customer"], r.Data["order_id"])
}
```

A fila, o que uma decisão significa e a conferência de que a rota da ferramenta existe:

```go
// AIAgentSetup is app/setup.go: the queue, what a decision means, and the
// check that says the tool's route is there. The check is a health check and
// not a panic at startup because Setup runs before the routes are registered —
// an app whose agent points at a route nobody wrote must fail its own probe,
// not fail on the day somebody asks.
func AIAgentSetup(a *trilha.App) error {
	queue := approval.New(approval.Options{
		Logger: a.Logger(),
		Roles:  func(c *trilha.Ctx) []string { return AIAgentRoles(c) },
	})
	queue.On(AIAgentCancelKind, AIAgentCancelled)
	trilha.Provide(a, queue)
	a.Check("agent-tools", func(context.Context) error {
		if r, ok := a.Route(AIAgentOrderRoute); !ok || r.Methods["GET"] == nil {
			return fmt.Errorf("the get_order tool calls GET %s, and no route answers it", AIAgentOrderRoute)
		}
		return nil
	})
	return queue.Setup(a)
}
```

A tela onde alguém decide é o [`ui.Inbox`](/pt/referencia/ui), uma tabela e dois formulários
sem JavaScript:

```go
// AIAgentInboxPage is that screen: what is waiting for whoever is reading, and
// two buttons. Who may decide is the package's answer and not this screen's.
func AIAgentInboxPage(c *trilha.Ctx) (h.Node, error) {
	queue := trilha.Use[*approval.Approvals](c)
	waiting, err := queue.Inbox(c, approval.ListParams{})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rows := make([]ui.InboxRow, 0, len(waiting))
	for _, r := range waiting {
		rows = append(rows, ui.InboxRow{
			ID: r.ID, Kind: r.Kind, Subject: r.Subject, Target: r.Target,
			State: r.State, Due: r.Due, Late: r.Late(now), By: r.By, Reason: r.Reason,
		})
	}
	c.SetTitle("Approvals")
	return ui.Stack(
		ui.PageHeader("Approvals"),
		ui.Inbox(c, rows, ui.InboxOpts{Decide: AIAgentInboxRoute, CSRF: trilha.CSRFInput(c)}),
	), nil
}
```

O resto da fila — quem pode decidir, o que expira, o que o `On` pode e não pode desfazer —
está em [`approval`](/pt/referencia/approval). O que importa aqui é o formato: **o modelo abre
um pedido e nunca fecha um.**

## 3. Dois agentes

A triagem responde o que dá e repassa o que não dá. O repasse é uma ferramenta que o framework
escreve (`transfer_to_billing`), então é o modelo que decide usá-la.

```go
// AIAgentBilling is the specialist: it reads orders and asks for
// cancellations, and it says out loud that it asks rather than does.
func AIAgentBilling(c *trilha.Ctx) *ai.Agent {
	return &ai.Agent{
		Name: "billing",
		Instructions: "You handle orders, payments and refund requests for an online store. " +
			"Use the tools; never state a fact about an order you did not read with one. " +
			"A cancellation or a refund is a request a person decides: say that it was asked for, " +
			"never that it was done.",
		Tools:    AIAgentTools(c),
		MaxTurns: AIAgentMaxTurns,
	}
}
```

```go
// AIAgentTriage is what answers first: it looks things up, answers what it can
// and hands the rest over. The handoff is the model's decision, which is why
// this is a handoff and not a Chain — see the recipe for when it is the other
// way around.
func AIAgentTriage(c *trilha.Ctx) *ai.Agent {
	return &ai.Agent{
		Name: "triage",
		Instructions: "You are the first line of support of an online store. Answer questions about " +
			"delivery and status yourself, with the search tool. Anything about money — refunds, " +
			"charges, cancelling an order — transfer to billing.",
		Tools:    []*ai.Tool{AIAgentAudited(c, AIAgentSearchTool(c))},
		Handoffs: []*ai.Agent{AIAgentBilling(c)},
		MaxTurns: AIAgentMaxTurns,
	}
}
```

Depois de um repasse as instruções mudam e o histórico fica, então o financeiro lê o que o
cliente já disse. Repare que os dois agentes não dividem a lista de ferramentas: a triagem não
consegue cancelar um pedido, porque uma ferramenta que o agente não tem é uma ferramenta que
ninguém convence ele a chamar.

**Quando o repasse é a ferramenta errada.** O repasse serve quando é *a conversa* que decide
para onde ir. Quando é *o app* que sabe a ordem dos passos, escreva a ordem em Go:

- `ai.Chain(ctx, cli, input, a, b)` — o b lê a saída do a. Extrair, depois resumir.
- `ai.Parallel(ctx, cli, input, a, b)` — os dois ao mesmo tempo, na ordem dos agentes.
- `pesquisador.AsTool(cli, "...")` — o segundo agente como função do primeiro, que continua
  dono da conversa.

Um passo que o app já conhece não deveria depender de o modelo lembrar de delegar. A maior
parte dos sistemas "multiagente" é um `Chain` com passos a mais; veja
[composição](/pt/referencia/ai#composicao).

## 4. A corrida na tela

A rota é a mesma em que a receita do chat termina, com o agente no lugar do assistente:

```go
// AIAgentPOST is app/api/chat/route.go. ai.Serve runs the agent and answers:
// the stream of events ui.chat.js reads — text, tool_call, tool_result,
// handoff — or, without JavaScript, the finished answer.
func AIAgentPOST(c *trilha.Ctx) error {
	return ai.ServeOpts{
		HTML: ui.ChatHTML,
		Page: AIAgentAnswerPage,
	}.Serve(c, AIAgentClient, AIAgentTriage(c))
}
```

O `ai.Serve` roda o `RunStream` por baixo e emite os eventos conforme eles acontecem — `text`,
`tool_call`, `tool_result`, `handoff`, `done`. O `ui.Chat` mostra esses passos quando você
pede:

```go
// AIAgentScreen is the page: the conversation next to the order it is about,
// with Steps on, so what the agent did is on the screen instead of in a log
// the customer cannot see.
func AIAgentScreen(c *trilha.Ctx, o AIAgentOrder) h.Node {
	return h.Div(h.Class("ui-stack"),
		ui.Chat(c, ui.ChatOpts{
			Action:   "/api/chat",
			Greeting: "Ask about your orders — where they are, what they cost, or ask to cancel one.",
			Context:  map[string]string{"order_id": o.ID},
			Steps:    true,
		}),
		ui.ChatScript(c),
	)
}
```

`Steps: true` é decisão de produto, não chave de depuração. Um agente que diz "pedi ao
atendimento para cancelar seu pedido" fica acreditável quando a linha acima diz `cancel_order →
request apr_… is waiting`; sem ela, o cliente tem que confiar numa frase. Sem JavaScript o
mesmo formulário posta na mesma rota e a página volta com a resposta pronta — o que precisa do
script são os passos, não a resposta.

## 5. O rastro

O rastro não é escrito dentro de cada ferramenta. Ele é escrito uma vez, em volta de todas:

```go
// AIAgentAudited is the trail, one line per tool call. It wraps a tool instead
// of being written inside each one, so a tool added next month is audited by
// the fact that it went through AIAgentTools — and not by whoever wrote it
// remembering.
func AIAgentAudited(c *trilha.Ctx, t *ai.Tool) *ai.Tool {
	call := t.Func
	return &ai.Tool{
		Name: t.Name, Description: t.Description, Parameters: t.Parameters,
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			out, err := call(ctx, args)
			fields := trilha.Fields{"arguments": string(args)}
			if err != nil {
				fields["error"] = err.Error()
			}
			c.Audit("ai.tool."+t.Name, aiAgentTarget(args), fields)
			return out, err
		},
	}
}
```

```go
// AIAgentTools is the set, built per request because every one of them carries
// the caller. Nothing outside this function hands a tool to an agent, which is
// how the audit line above cannot be forgotten.
func AIAgentTools(c *trilha.Ctx) []*ai.Tool {
	tools := []*ai.Tool{AIAgentSearchTool(c), AIAgentAPITool(c), AIAgentCancelTool(c)}
	for i, t := range tools {
		tools[i] = AIAgentAudited(c, t)
	}
	return tools
}
```

Uma ferramenta acrescentada no mês que vem é auditada porque passou pelo `AIAgentTools`, e não
porque quem a escreveu lembrou. Quanto a corrida custou é mais uma linha, na saída:

```go
// AIAgentAnswerPage answers the submit that did not ask for a stream, and
// writes the line the tool trail does not have: what the whole run cost.
func AIAgentAnswerPage(c *trilha.Ctx, res *ai.Result) error {
	c.Audit("ai.run", res.Agent.Name, trilha.Fields{
		"turns": res.Turns, "steps": len(res.Steps), "tokens": res.Usage.TotalTokens,
	})
	if id := c.Form("ctx.order_id"); id != "" {
		return c.Redirect("/orders/" + url.PathEscape(id))
	}
	return c.Redirect("/")
}
```

Uma resposta em streaming não tem onde pôr essa linha — quando a corrida termina, o `ai.Serve`
já acabou de escrever. Quando os números importam, a rota é o laço escrito à mão, com os mesmos
nomes de evento:

```go
// AIAgentStreamPOST is the same route written by hand, for when the run's own
// numbers have to reach the trail: ai.Serve has nowhere to put the Usage of a
// streamed answer, because the answer is already gone when the run ends. The
// events are the same ones, with the same names, so the screen does not change.
func AIAgentStreamPOST(c *trilha.Ctx) error {
	s := c.Stream()
	agent := AIAgentTriage(c)
	res, err := ai.RunStream(c.Context(), AIAgentClient, agent, c.Form("message"), func(ev ai.Event) {
		switch ev.Type {
		case "text":
			_ = s.Send("text", ev.Text)
		case "tool_call", "tool_result", "handoff":
			_ = s.JSON(ev.Type, map[string]any{
				"agent": ev.Agent, "tool": ev.Step.Tool,
				"arguments": ev.Step.Arguments, "output": ev.Step.Output, "to": ev.Step.HandoffTo,
			})
		}
	})
	if err != nil {
		c.Log().Warn("ai agent", "err", err)
		return s.JSON("error", map[string]string{"message": "The assistant could not finish this one."})
	}
	c.Audit("ai.run", res.Agent.Name, trilha.Fields{
		"turns": res.Turns, "steps": len(res.Steps), "tokens": res.Usage.TotalTokens,
	})
	return s.JSON("done", map[string]any{"output": res.Output, "html": ui.ChatHTML(res.Output)})
}
```

As duas terminam no mesmo lugar, ao lado de tudo o que as pessoas fizeram:

```go
// AIAgentTrailPage is the trail on a screen: every tool the agent called, with
// the arguments it called them with, next to everything else people did.
func AIAgentTrailPage(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Audit")
	return ui.Stack(
		ui.PageHeader("Audit"),
		ui.AuditTable(c, AIAgentTrail.Records()),
	), nil
}
```

O [`ui.AuditTable`](/pt/referencia/ui) é a tela; o [`c.Audit`](/pt/referencia/observabilidade)
é o destino. O `Actor.Via` é o que separa a chamada de um agente da de uma pessoa — por isso a
ferramenta de leitura marca a requisição dela.

## 6. Testando

O provedor é roteirizado: ele responde as chamadas de ferramenta na ordem em que precisam
acontecer, e depois a frase. Sem chave, sem fatura e sem rede — o que é trocado é o
`http.Client` que o framework ia chamar de qualquer jeito.

```go
// AIAgentFake is the provider in a test: it answers the script in order and
// keeps every conversation it was sent. No key, no bill and no network — what
// is replaced is the http.Client the framework was always going to call, so
// the route, the tools and the queue run exactly as they do in production.
//
// When the script runs out it repeats its last answer, which is how the test
// for MaxTurns writes "a model that will not stop" in one line.
type AIAgentFake struct {
	// Script is what the model answers, one entry per model call.
	Script []AIAgentReply

	mu    sync.Mutex
	asked int
	sent  [][]ai.Message
}
```

O teste que importa é o que prova que a escrita não aconteceu:

```go
// The point of the recipe: a tool that changes something does not change it.
// The model asks, the queue keeps the ask, and the order is still open until a
// person says otherwise.
func TestAIAgentCancelWaitsForAPerson(t *testing.T) {
	c, fake, a := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "transfer_to_billing", Arguments: `{"reason":"the customer asked about an order"}`}}},
		AIAgentReply{Calls: []AIAgentCall{{Name: "cancel_order", Arguments: `{"order_id":"1043","reason":"bought the wrong size"}`}}},
		AIAgentReply{Text: "I asked for order 1043 to be cancelled; someone from support will confirm."},
	)
	ask(c, "cancel order 1043").WantStatus(303)

	// The store did not move.
	if o, _ := AIAgentOrders.Find(aiAgentMine.Customer, "1043"); o.Status != "shipped" {
		t.Fatalf("the agent changed the order on its own: %+v", o)
	}
	// The queue did.
	pending, err := trilha.Use[*approval.Approvals](a).List(context.Background(), approval.ListParams{Kind: AIAgentCancelKind})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].State != approval.Pending || pending[0].Data["order_id"] != "1043" {
		t.Fatalf("the cancellation is not waiting in the queue: %+v", pending)
	}
	if !strings.Contains(pending[0].Data["reason"], "wrong size") {
		t.Errorf("the reason the model gave is not on the record: %+v", pending[0].Data)
	}

	// And the model was told the truth: it asked, nothing happened yet.
	if got := aiAgentLastTool(fake); !strings.Contains(got, "waiting") {
		t.Errorf("the tool result must say the cancellation is waiting: %q", got)
	}

	// The trail says which tool ran, for whom.
	if !aiAgentAudited(t, "ai.tool.cancel_order", "1043") {
		t.Errorf("the tool call is not in the audit trail: %+v", AIAgentTrail.Records())
	}
}
```

O store não mudou, a fila tem o pedido com o motivo, e o modelo ouviu a verdade. A outra
metade — uma pessoa aprova e *aí sim* o pedido muda — é o
[`TestAIAgentDecisionCancelsTheOrder`](https://github.com/emersonjoe/trilha/blob/main/examples/cookbook/aiagent_test.go)
logo ao lado.

Outros dois valem a cópia. Quem recusa é a cadeia da rota, e o handler não roda:

```go
func TestAIAgentToolStopsAtTheRouteChain(t *testing.T) {
	c, _, _ := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "transfer_to_billing", Arguments: `{"reason":"the customer asked about an order"}`}}},
		AIAgentReply{Calls: []AIAgentCall{{Name: "get_order", Arguments: `{"id":"1043"}`}}},
		AIAgentReply{Text: "I cannot see that order."},
	)
	// No cookie: a visitor who is not signed in.
	c.PostForm("/api/chat", url.Values{"message": {"where is 1043?"}}).WantStatus(303)
	for _, r := range AIAgentTrail.Records() {
		if r.Action == "order.read" {
			t.Fatalf("the handler ran for a caller the chain refuses: %+v", r)
		}
	}
}
```

E o `MaxTurns` é o cinto: um modelo que insiste em chamar ferramenta em vez de responder para
com `ai.ErrMaxTurns`, e não na fatura.

```go
// The belt: a model that only ever calls tools stops at MaxTurns instead of
// running until the bill notices.
func TestAIAgentStopsAtMaxTurns(t *testing.T) {
	c, fake, _ := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "get_order", Arguments: `{"id":"1043"}`}}},
	)
	ask(c, "and again?").WantStatus(500)
	if fake.Asked() != AIAgentMaxTurns {
		t.Fatalf("the model was asked %d times, want %d", fake.Asked(), AIAgentMaxTurns)
	}
}
```

## 7. O mesmo conjunto, para o agente dos outros

As ferramentas carregam quem chamou, então expor tudo por MCP é uma rota:

```go
// AIAgentMCPPOST is the same set of tools for an agent that is not this app's:
// Claude, an editor, anything that speaks MCP. The server is built per request
// because the tools carry the caller — the host's key decides what the tools
// can see, exactly as the chat's session does.
func AIAgentMCPPOST(c *trilha.Ctx) error {
	return mcp.NewServer("orders", "1.0", AIAgentTools(c)...).ServeHTTP(c)
}
```

Aponte o Claude, o Cursor ou um editor para `POST /mcp` e ele recebe `search_orders`,
`get_order` e `cancel_order` — inclusive o fato de que cancelar abre um pedido. O embrulho da
auditoria continua rodando, e a fila também. Veja [`mcp`](/pt/referencia/mcp) para o protocolo
e o id de sessão, e a [sua API como ferramentas](/pt/receitas/api-como-ferramentas) quando o
que você quer expor é a `/api/` inteira e não um conjunto escolhido a dedo.

## 8. A demo

A [`/demos/ai-agent`](/pt/demos/ai-agent) é esta página rodando: pergunte onde está o pedido e
veja a chamada da ferramenta; peça para cancelar e veja o repasse, o pedido que entra na fila e
a resposta dizendo que uma pessoa está com ele. É roteirizada no navegador — o site da
documentação é estático —, mas o componente, os passos e a caixa de entrada são os reais.

## Para onde ir depois

- [IA e agentes](/pt/aprender/ia-e-agentes) — as peças sozinhas: uma chamada, uma ferramenta, streaming, MCP.
- [Referência do `ai`](/pt/referencia/ai) — `Agent`, `Run`, `RunStream`, o contrato de eventos, composição.
- [Referência do `approval`](/pt/referencia/approval) — a fila: responsáveis, prazos, estados.
- [Chat de IA](/pt/receitas/chat-de-ia) — a chave, o histórico, os limites, a falha que o visitante lê.
- [`examples/assistente`](https://github.com/emersonjoe/trilha/tree/main/examples/assistente) —
  as mesmas peças num aplicativo completo.
