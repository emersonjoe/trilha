---
title: An AI agent
description: An agent that acts on your app's data — tools that read what the visitor may read, a write that waits for a person, two agents, the steps on the screen, the audit trail, a scripted test and the same tools over MCP.
---

The [AI chat](/cookbook/ai-chat) recipe puts a conversation in your app. This one is the
question of the second day: the assistant stops answering and starts *doing* — looking an
order up, reading the API, asking for a cancellation. Four things have to be true before that
is a good idea, and no page says them together:

- it reads **what the person asking may read**, and nothing else;
- what it may **do on its own** is separated from what **waits for a person**;
- what it did is **written down**, tool by tool;
- and you can **test all of it** without spending a call.

The domain is small and the same as the chat recipe's: orders and customers of an online
store. Every block below is a declaration in
[`examples/cookbook/aiagent.go`](https://github.com/emersonjoe/trilha/blob/main/examples/cookbook/aiagent.go),
which compiles and is tested with the rest of the repository. It ends at
[the demo](/demos/ai-agent), where a whole run — the handoff, the tools, the queue — happens
in front of you.

## 1. Tools that read

A tool is a function with a name, a description and a JSON schema. What makes it *your* tool
is the closure around it: the request is inside, so the visitor is inside.

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

The index is the app's own, the one the search box already uses. The scoping is the index's
and not the tool's:

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

Read that closure once more: **the caller is not an argument**. There is no id the model can
invent that widens what it sees, because the only thing it can send is words to search for.
That is the whole permission model of a read tool — [`search`](/reference/search) does the
rest, and a denied kind leaves no trace in the count.

### The same API you already have

The second tool reads one order in full, and there is already a route that does that. It is
not declared a second time: it is *called*, as a request, with the caller's own credential.

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

Two lines carry the recipe. `App.Probe` runs the route's middleware chain — the session, the
key, the policy — **without running the handler**, and answers whether this caller would get
through; a caller who would not gets a tool that says no, instead of a handler that ran.
`trilha.WithVia(req, "agent")` marks how the call arrived, so the audit record the handler
writes says `agent` and not `session`: six months later, "what did the assistant do" is a
filter and not an investigation.

The recorder is the whole bridge:

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
This is [your API as agent tools](/cookbook/api-as-tools) seen from the other side. There,
`mcp.FromRoutes` turns the routes into tools for somebody else's agent, and `Probe` decides
what each key may see. Here the agent is yours and the tool is hand-written, because it is
one route and a sentence the model reads — but the primitive is the same one.
:::

## 2. A tool that writes, with a person in the middle

"Cancel the order" is where every agent demo quietly becomes a bad idea. The recipe's answer
is that the tool does not cancel anything: it opens a request, and says so.

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

What the model reads back is the truth — *waiting*, *not changed* — which is what keeps it
from telling the customer the order is cancelled. And the sentence in the tool's description
is not decoration either: the description is the only place the model learns that this tool
asks rather than does.

The decision is the only path from the conversation to the data:

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

The queue, what a decision means, and the check that says the tool's route is there:

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

The screen a person decides in is [`ui.Inbox`](/reference/ui), a table and two forms with no
JavaScript:

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

Everything else about the queue — who may decide, what expires, what `On` may and may not
undo — is in [`approval`](/reference/approval). What matters here is the shape: **the model
can open a request and can never close one.**

## 3. Two agents

Triage answers what it can and hands over what it cannot. The handoff is a tool the framework
writes (`transfer_to_billing`), so it is the model that decides to use it.

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

After a handoff the instructions change and the history stays, so billing reads what the
customer already said. Notice the two agents do not share a tool list: triage cannot cancel
an order, because a tool an agent does not have is a tool it cannot be talked into calling.

**When a handoff is the wrong tool.** A handoff is right when *the conversation* decides where
to go. When *the app* knows the order of the steps, say so in Go:

- `ai.Chain(ctx, cli, input, a, b)` — b reads a's output. Extract, then summarise.
- `ai.Parallel(ctx, cli, input, a, b)` — both at once, results in the agents' order.
- `researcher.AsTool(cli, "...")` — the second agent as a function of the first, which keeps
  the conversation with the caller.

A step the app already knows should not depend on the model remembering to delegate it. Most
"multi-agent" systems are a `Chain` with extra steps; see
[composition](/reference/ai#composition).

## 4. The run on the screen

The route is the same one the chat recipe ends at, with the agent in place of the assistant:

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

`ai.Serve` runs `RunStream` under the hood and emits the events as they happen — `text`,
`tool_call`, `tool_result`, `handoff`, `done`. `ui.Chat` shows them when you ask it to:

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

`Steps: true` is a product decision, not a debug flag. An agent that says "I asked support to
cancel your order" is believable when the line above it says `cancel_order → request apr_… is
waiting`; without it, the customer has to trust a sentence. Without JavaScript the same form
posts to the same route and the page comes back with the finished answer — the steps are the
part that needs the script, not the answer.

## 5. The trail

The trail is not written inside each tool. It is written once, around all of them:

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

A tool added next month is audited because it went through `AIAgentTools`, and not because
whoever wrote it remembered. What the run itself cost is one more line, on the way out:

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

A streamed answer has nowhere for that line to go — `ai.Serve` has already finished writing
when the run ends. When the numbers matter, the route is the loop written by hand, with the
same event names:

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

Both end up in the same place, next to everything people did:

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

[`ui.AuditTable`](/reference/ui) is the screen; [`c.Audit`](/reference/observability) is the
sink. `Actor.Via` is what tells an agent's call apart from a person's, which is why the read
tool marks its request.

## 6. Testing it

The provider is scripted: it answers tool calls in the order they must happen, then the
sentence. No key, no bill, no network — what is replaced is the `http.Client` the framework
was always going to call.

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

The test that matters is the one that proves the write did not happen:

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

The store did not move, the queue has the request with the reason on it, and the model was
told the truth. The other half — a person approves, and *then* the order changes — is
[`TestAIAgentDecisionCancelsTheOrder`](https://github.com/emersonjoe/trilha/blob/main/examples/cookbook/aiagent_test.go)
next to it.

Two more are worth copying. The chain of a route is what refuses, and the handler never runs:

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

And `MaxTurns` is the belt: a model that keeps calling tools instead of answering stops with
`ai.ErrMaxTurns` rather than in the invoice.

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

## 7. The same tools, for somebody else's agent

The tools carry the caller, so exposing them over MCP is one route:

```go
// AIAgentMCPPOST is the same set of tools for an agent that is not this app's:
// Claude, an editor, anything that speaks MCP. The server is built per request
// because the tools carry the caller — the host's key decides what the tools
// can see, exactly as the chat's session does.
func AIAgentMCPPOST(c *trilha.Ctx) error {
	return mcp.NewServer("orders", "1.0", AIAgentTools(c)...).ServeHTTP(c)
}
```

Point Claude, Cursor or an editor at `POST /mcp` and it gets `search_orders`, `get_order` and
`cancel_order` — including the fact that cancelling opens a request. The audit wrapper still
runs, and so does the queue. See [`mcp`](/reference/mcp) for the protocol and the session id,
and [your API as agent tools](/cookbook/api-as-tools) when what you want to expose is the
whole `/api/` and not a hand-picked set.

## 8. The demo

[`/demos/ai-agent`](/demos/ai-agent) is this page running: ask where the order is and watch
the tool call; ask to cancel it and watch the handoff, the request that opens in the queue,
and the answer that says a person has it. It is scripted in the browser — the documentation
site is static — but the component, the steps and the inbox are the real ones.

## Where to go next

- [AI and agents](/learn/ai-and-agents) — the pieces on their own: a call, a tool, streaming, MCP.
- [`ai` reference](/reference/ai) — `Agent`, `Run`, `RunStream`, the event contract, composition.
- [`approval` reference](/reference/approval) — the queue: assignees, deadlines, states.
- [AI chat](/cookbook/ai-chat) — the key, the history, the limits, the failure the visitor reads.
- [`examples/assistente`](https://github.com/emersonjoe/trilha/tree/main/examples/assistente) —
  the same pieces in a complete application.
