package cookbook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/ai/mcp"
	"github.com/emersonjoe/trilha/approval"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// AIAgentClient is the provider, built once at startup from the environment —
// the key is never in the code. It is a package var so a test can put a
// scripted one in its place.
var AIAgentClient = ai.NewFromEnv()

// AIAgentMaxTurns is how many times one question may go to the model. It is
// the belt of the whole recipe: a model that keeps calling tools instead of
// answering stops here with ai.ErrMaxTurns, and the bill stops with it.
const AIAgentMaxTurns = 6

// AIAgentHits is how many rows a search may put in front of the model. A tool
// that answers with a hundred rows spends the context window on rows nobody
// asked about.
const AIAgentHits = 5

// AIAgentOrder is the record the agent works on. It belongs to the app, and
// Customer is the account it belongs to: the string every query below takes,
// so no tool can read a row by id alone.
type AIAgentOrder struct {
	ID       string
	Customer string
	Status   string
	Total    string
	Placed   time.Time
}

// AIAgentStore is the app's data. In an application these three methods are
// three queries; what matters to the recipe is that each of them takes the
// customer, which is what makes the lookup the permission check.
type AIAgentStore struct {
	mu sync.Mutex
	by map[string]AIAgentOrder
}

// NewAIAgentStore builds an empty store.
func NewAIAgentStore() *AIAgentStore { return &AIAgentStore{by: map[string]AIAgentOrder{}} }

// AIAgentOrders is the store the routes and the tools below read.
var AIAgentOrders = NewAIAgentStore()

// Put writes one order.
func (s *AIAgentStore) Put(o AIAgentOrder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.by[o.ID] = o
}

// Find answers with the order when it is that customer's, and with nothing
// when it is somebody else's — the same answer as for an id that does not
// exist, on purpose: "this one is not yours" is already an answer.
func (s *AIAgentStore) Find(customer, id string) (AIAgentOrder, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.by[id]
	if !ok || o.Customer != customer {
		return AIAgentOrder{}, false
	}
	return o, true
}

// All is every order, for the indexing pass.
func (s *AIAgentStore) All() []AIAgentOrder {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AIAgentOrder, 0, len(s.by))
	for _, o := range s.by {
		out = append(out, o)
	}
	return out
}

// Cancel is the only place an order stops. Nothing the model can reach calls
// it: it runs after a person decided.
func (s *AIAgentStore) Cancel(customer, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.by[id]
	if !ok || o.Customer != customer {
		return fmt.Errorf("order %s not found for %s", id, customer)
	}
	o.Status = "cancelled"
	s.by[id] = o
	return nil
}

// AIAgentWho is who is asking: the signed-in visitor, or the address when
// there is nobody. It is the customer of every query and the tenant of the
// index — one answer, in one place, so no tool can be written without it.
func AIAgentWho(c *trilha.Ctx) string {
	if a := c.Actor(); a.Subject != "" {
		return a.Subject
	}
	return c.ClientIP()
}

// AIAgentSearch is the app's index, the one the search box on the site header
// already uses. The tenant of a document is the customer it belongs to, so a
// query is scoped by the index and not by a filter every tool has to remember
// to write.
var AIAgentSearch = trilha.NewSearch(trilha.SearchOpts{Tenant: AIAgentWho}).
	Kind("order", trilha.KindOpts{Label: "Orders"})

// AIAgentIndex writes the projection the search reads — enough to find the
// order, name it and go to it, and not the record itself.
func AIAgentIndex(c *trilha.Ctx, o AIAgentOrder) error {
	return AIAgentSearch.Put(c, trilha.Doc{
		Kind: "order", ID: o.ID, Tenant: o.Customer,
		Title: "Order " + o.ID + " — " + o.Status,
		Body:  o.Customer + " " + o.Total + " " + o.Placed.Format("2006-01-02"),
		URL:   "/orders/" + o.ID,
	})
}

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

// AIAgentOrderRoute is the route the read tool calls. It is a constant because
// two places have to agree: the app that registers it and the health check
// that says so when they stop agreeing.
const AIAgentOrderRoute = "/api/orders/{id}"

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

// AIAgentSignedIn is the chain that route already had. Probe runs it — that is
// the whole point of the tool below.
func AIAgentSignedIn(c *trilha.Ctx, next trilha.Next) error {
	if c.Actor().Subject == "" {
		return trilha.Errorf(http.StatusForbidden, "sign in to see your orders")
	}
	return next()
}

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

// AIAgentRecorder collects what a route answers when the caller is a tool and
// not a socket. Fifteen lines instead of a second handler: whatever the route
// answers a browser is what the model reads.
type AIAgentRecorder struct {
	Status int
	Body   bytes.Buffer

	header http.Header
}

// Header is the response header the route writes into.
func (r *AIAgentRecorder) Header() http.Header {
	if r.header == nil {
		r.header = http.Header{}
	}
	return r.header
}

// Write keeps the body.
func (r *AIAgentRecorder) Write(b []byte) (int, error) { return r.Body.Write(b) }

// WriteHeader keeps the status.
func (r *AIAgentRecorder) WriteHeader(code int) { r.Status = code }

// AIAgentCancelKind is the kind of request the queue groups by, and what the
// On below is registered for.
const AIAgentCancelKind = "order.cancel"

// AIAgentCancelDue is how long a cancellation may wait before it expires. A
// queue with no deadline is a queue that grows.
const AIAgentCancelDue = 24 * time.Hour

// AIAgentRoles is how a request assigned to a role finds its people. It is a
// var because the approval package does not know how this app authenticates,
// and a check it guessed would look like a guarantee without being one.
var AIAgentRoles = func(c *trilha.Ctx) []string { return nil }

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

// AIAgentCancelled is what the decision means, and the only path from the
// conversation to the data. It runs after the decision is written, so a
// failure here does not undo somebody's choice.
func AIAgentCancelled(c *trilha.Ctx, r approval.Record) error {
	if r.State != approval.Approved {
		return nil
	}
	return AIAgentOrders.Cancel(r.Data["customer"], r.Data["order_id"])
}

// AIAgentInboxRoute is where a person decides.
const AIAgentInboxRoute = "/admin/approvals"

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

// AIAgentInboxPOST writes the decision. The decision and the reason travel in
// the same form.
func AIAgentInboxPOST(c *trilha.Ctx) error {
	err := trilha.Use[*approval.Approvals](c).Decide(c, c.Form("id"), c.Form("decision"), c.Form("reason"))
	if err != nil {
		if err == approval.ErrNotYours {
			return trilha.Errorf(http.StatusForbidden, "this one is not yours to decide")
		}
		return err
	}
	return c.Redirect(AIAgentInboxRoute)
}

// AIAgentAuditLog is this app's audit sink: in memory here, a table in an
// application. The interface is one method precisely so the decision left to
// make is "which table" and not "which shape".
type AIAgentAuditLog struct {
	mu   sync.Mutex
	recs []trilha.AuditRecord
}

// Write keeps one record.
func (l *AIAgentAuditLog) Write(r trilha.AuditRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recs = append(l.recs, r)
	return nil
}

// Records is what was written, oldest first.
func (l *AIAgentAuditLog) Records() []trilha.AuditRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]trilha.AuditRecord(nil), l.recs...)
}

// AIAgentTrail is the sink the app hands to trilha.Config.Audit.
var AIAgentTrail = &AIAgentAuditLog{}

// AIAgentTrailPage is the trail on a screen: every tool the agent called, with
// the arguments it called them with, next to everything else people did.
func AIAgentTrailPage(c *trilha.Ctx) (h.Node, error) {
	c.SetTitle("Audit")
	return ui.Stack(
		ui.PageHeader("Audit"),
		ui.AuditTable(c, AIAgentTrail.Records()),
	), nil
}

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

// aiAgentTarget is what the call was about, for the Target column: the id in
// the arguments when there is one, so the trail of an order is one filter
// away.
func aiAgentTarget(args json.RawMessage) string {
	var in struct {
		OrderID string `json:"order_id"`
		ID      string `json:"id"`
	}
	_ = json.Unmarshal(args, &in)
	if in.OrderID != "" {
		return in.OrderID
	}
	return in.ID
}

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

// AIAgentPOST is app/api/chat/route.go. ai.Serve runs the agent and answers:
// the stream of events ui.chat.js reads — text, tool_call, tool_result,
// handoff — or, without JavaScript, the finished answer.
func AIAgentPOST(c *trilha.Ctx) error {
	return ai.ServeOpts{
		HTML: ui.ChatHTML,
		Page: AIAgentAnswerPage,
	}.Serve(c, AIAgentClient, AIAgentTriage(c))
}

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

// AIAgentMCPPOST is the same set of tools for an agent that is not this app's:
// Claude, an editor, anything that speaks MCP. The server is built per request
// because the tools carry the caller — the host's key decides what the tools
// can see, exactly as the chat's session does.
func AIAgentMCPPOST(c *trilha.Ctx) error {
	return mcp.NewServer("orders", "1.0", AIAgentTools(c)...).ServeHTTP(c)
}

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

// AIAgentCall is one tool call in a script: what the model asks for and with
// which arguments.
type AIAgentCall struct{ Name, Arguments string }

// AIAgentReply is one answer of the scripted provider — a sentence, or the
// tool calls that come before it.
type AIAgentReply struct {
	Text  string
	Calls []AIAgentCall
}

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

// Client is the ai.Client to put where the real one goes.
func (f *AIAgentFake) Client() *ai.Client {
	return &ai.Client{
		BaseURL:    "https://example.invalid/v1",
		APIKey:     "test-key",
		Model:      "test-model",
		HTTPClient: &http.Client{Transport: f},
	}
}

// Sent is one entry per model call: the conversation as the model read it,
// with the instructions first and the tool results in it.
func (f *AIAgentFake) Sent() [][]ai.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]ai.Message(nil), f.sent...)
}

// Asked is how many times the model was called.
func (f *AIAgentFake) Asked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

// RoundTrip answers one turn of the script in the shape a chat-completions API
// answers it.
func (f *AIAgentFake) RoundTrip(r *http.Request) (*http.Response, error) {
	var in struct {
		Messages []ai.Message `json:"messages"`
	}
	if r.Body != nil {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
	}
	f.mu.Lock()
	reply := AIAgentReply{Text: "no script"}
	if n := len(f.Script); n > 0 {
		reply = f.Script[min(f.asked, n-1)]
	}
	f.asked++
	f.sent = append(f.sent, in.Messages)
	f.mu.Unlock()

	calls := make([]ai.ToolCall, 0, len(reply.Calls))
	for i, call := range reply.Calls {
		calls = append(calls, ai.ToolCall{
			ID: "call-" + strconv.Itoa(i), Type: "function",
			Function: ai.FunctionCall{Name: call.Name, Arguments: call.Arguments},
		})
	}
	finish := "stop"
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	body, err := json.Marshal(ai.Response{
		ID: "chatcmpl-test", Model: "test-model",
		Choices: []ai.Choice{{Message: ai.Message{Role: "assistant", Content: reply.Text, ToolCalls: calls}, FinishReason: finish}},
		Usage:   ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	})
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    r,
	}, nil
}
