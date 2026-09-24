package cookbook

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/approval"
)

// The agent recipe is tested the way the page says it is: a provider that
// answers from a script — the tool calls in the order they must happen, then
// the sentence — with no key in the environment and nothing dialled. The
// route, the tools, the queue and the audit trail are the real ones.

// The two orders of the test: one the visitor owns and one they do not.
var (
	aiAgentMine   = AIAgentOrder{ID: "1043", Customer: "ada@example.com", Status: "shipped", Total: "$32.90", Placed: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}
	aiAgentTheirs = AIAgentOrder{ID: "2001", Customer: "grace@example.com", Status: "shipped", Total: "$99.00", Placed: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)}
)

// aiAgentSignIn is the app's session middleware, small enough to fit in a
// test: the cookie says who is asking. It matters that it is a cookie and not
// a test-only header — the tool that calls the app's own API forwards the
// caller's cookie, so what the agent gets is what the visitor would get.
func aiAgentSignIn(c *trilha.Ctx, next trilha.Next) error {
	if ck, err := c.Cookie("who"); err == nil && ck.Value != "" {
		c.SetActor(trilha.Actor{Subject: ck.Value, Name: "Ada"})
	}
	return next()
}

// newAIAgentTest stands up the app of the recipe: the chat route, the API the
// read tool calls, the inbox a person decides in, and the queue.
func newAIAgentTest(t *testing.T, script ...AIAgentReply) (*trilha.TestClient, *AIAgentFake, *trilha.App) {
	t.Helper()
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")
	t.Setenv("OPENAI_API_KEY", "")

	fake := &AIAgentFake{Script: script}
	swap(t, &AIAgentClient, fake.Client())
	swap(t, &AIAgentOrders, NewAIAgentStore())
	swap(t, &AIAgentRoles, func(*trilha.Ctx) []string { return []string{"support"} })
	swap(t, &AIAgentTrail, &AIAgentAuditLog{})
	AIAgentOrders.Put(aiAgentMine)
	AIAgentOrders.Put(aiAgentTheirs)

	cfg := trilha.ConfigFromEnv()
	cfg.Audit = AIAgentTrail
	a := trilha.New(cfg)
	if err := AIAgentSetup(a); err != nil {
		t.Fatal(err)
	}
	session := []trilha.MiddlewareFunc{aiAgentSignIn}
	a.Register(trilha.Route{Pattern: "/api/chat", Middlewares: session,
		Methods: map[string]trilha.HandlerFunc{"POST": AIAgentPOST}})
	a.Register(trilha.Route{Pattern: AIAgentOrderRoute, Middlewares: append(session, AIAgentSignedIn),
		Methods: map[string]trilha.HandlerFunc{"GET": AIAgentOrderGET}})
	a.Register(trilha.Route{Pattern: AIAgentInboxRoute, Middlewares: session,
		Page:    AIAgentInboxPage,
		Methods: map[string]trilha.HandlerFunc{"POST": AIAgentInboxPOST}})
	a.Register(trilha.Route{Pattern: "/index", Middlewares: session,
		Methods: map[string]trilha.HandlerFunc{"GET": func(c *trilha.Ctx) error {
			for _, o := range AIAgentOrders.All() {
				if err := AIAgentIndex(c, o); err != nil {
					return err
				}
			}
			return c.Text(200, "indexed")
		}}})
	return trilha.NewTestClient(t, a), fake, a
}

// ada is the visitor of every test below.
func ada() trilha.TestOption { return trilha.WithCookie("who", aiAgentMine.Customer) }

// ask sends one question the way a browser without JavaScript sends it.
func ask(c *trilha.TestClient, question string) *trilha.TestResponse {
	return c.PostForm("/api/chat", url.Values{"message": {question}}, ada())
}

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

// The other half: the decision is what writes, and it writes once.
func TestAIAgentDecisionCancelsTheOrder(t *testing.T) {
	c, _, a := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "transfer_to_billing", Arguments: `{"reason":"the customer asked about an order"}`}}},
		AIAgentReply{Calls: []AIAgentCall{{Name: "cancel_order", Arguments: `{"order_id":"1043","reason":"bought the wrong size"}`}}},
		AIAgentReply{Text: "asked"},
	)
	ask(c, "cancel order 1043").WantStatus(303)

	// The person sees it in the inbox and decides there.
	c.Get(AIAgentInboxRoute, ada()).WantStatus(200).WantContains("Cancel order 1043", `name="decision" value="approved"`)
	pending, err := trilha.Use[*approval.Approvals](a).List(context.Background(), approval.ListParams{Kind: AIAgentCancelKind})
	if err != nil || len(pending) != 1 {
		t.Fatalf("queue: %+v %v", pending, err)
	}
	c.PostForm(AIAgentInboxRoute, url.Values{
		"id": {pending[0].ID}, "decision": {approval.Approved}, "reason": {"within the window"},
	}, ada()).WantStatus(303)

	if o, _ := AIAgentOrders.Find(aiAgentMine.Customer, "1043"); o.Status != "cancelled" {
		t.Fatalf("the decision did not reach the order: %+v", o)
	}
}

// The read tool goes through the app's own API, and the chain of that route is
// what says yes or no. A visitor who is not signed in gets a tool that answers
// "not allowed" without the handler ever running.
func TestAIAgentReadsThroughItsOwnAPI(t *testing.T) {
	c, _, _ := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "transfer_to_billing", Arguments: `{"reason":"the customer asked about an order"}`}}},
		AIAgentReply{Calls: []AIAgentCall{{Name: "get_order", Arguments: `{"id":"1043"}`}}},
		AIAgentReply{Text: "It shipped on the 1st."},
	)
	ask(c, "where is 1043?").WantStatus(303)
	if !aiAgentAudited(t, "order.read", "1043") {
		t.Fatalf("the API route did not answer the tool: %+v", AIAgentTrail.Records())
	}
	// The record says how the call arrived: the agent, not the person typing
	// into the API by hand.
	for _, r := range AIAgentTrail.Records() {
		if r.Action == "order.read" && r.Actor.Via != "agent" {
			t.Errorf("the trail must say the call came from the agent: %+v", r.Actor)
		}
	}
}

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

// The search tool reads the index of the caller and only that: another
// customer's order is not a hit, because the scoping is the index's and not a
// filter the tool has to remember.
func TestAIAgentSearchIsScopedToTheCaller(t *testing.T) {
	c, fake, _ := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "search_orders", Arguments: `{"query":"order"}`}}},
		AIAgentReply{Text: "You have one order."},
	)
	c.Get("/index", ada()).WantStatus(200)
	c.Get("/index", trilha.WithCookie("who", aiAgentTheirs.Customer)).WantStatus(200)
	ask(c, "what are my orders?").WantStatus(303)

	result := aiAgentLastTool(fake)
	if !strings.Contains(result, "1043") {
		t.Errorf("the visitor's own order is not in the result: %q", result)
	}
	if strings.Contains(result, "2001") {
		t.Errorf("another customer's order reached the model: %q", result)
	}
}

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

// Triage answers what it can and hands the rest over: after the handoff the
// conversation carries the other agent's instructions, and its tools.
func TestAIAgentHandsOffToBilling(t *testing.T) {
	c, fake, _ := newAIAgentTest(t,
		AIAgentReply{Calls: []AIAgentCall{{Name: "transfer_to_billing", Arguments: `{"reason":"refund"}`}}},
		AIAgentReply{Text: "Billing here — I can start the refund."},
	)
	ask(c, "I want my money back").WantStatus(303)

	sent := fake.Sent()
	if len(sent) < 2 {
		t.Fatalf("the handoff did not happen: %d", len(sent))
	}
	if sent[1][0].Role != "system" || !strings.Contains(sent[1][0].Content, "refund") {
		t.Errorf("after the handoff the instructions must be billing's: %#v", sent[1][0])
	}
}

// aiAgentLastTool is the last tool result the model read.
func aiAgentLastTool(fake *AIAgentFake) string {
	sent := fake.Sent()
	for i := len(sent) - 1; i >= 0; i-- {
		for j := len(sent[i]) - 1; j >= 0; j-- {
			if sent[i][j].Role == "tool" {
				return sent[i][j].Content
			}
		}
	}
	return ""
}

// aiAgentAudited reports whether the trail has that action on that target.
func aiAgentAudited(t *testing.T, action, target string) bool {
	t.Helper()
	for _, r := range AIAgentTrail.Records() {
		if r.Action == action && r.Target == target {
			return true
		}
	}
	return false
}
