package cookbook

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// AIChatTestOrder is the record the test's chat sits next to.
var aiChatTestOrder = AIChatOrder{
	ID: "1043", Customer: "Ada", Status: "shipped", Total: "$32.90",
	Placed: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
}

// The chat recipe is tested the way the page says it is: a scripted provider,
// no key in the environment and nothing dialled. What runs is the real route,
// the real component and the real context function.
func newAIChatTest(t *testing.T, answer string) (*trilha.TestClient, *AIChatFake) {
	t.Helper()
	t.Setenv("TRILHA_ENV", "dev")
	t.Setenv("TRILHA_SECRET", "a-test-secret-with-more-than-32-bytes!!")

	fake := &AIChatFake{Answer: answer}
	swap(t, &AIChatClient, fake.Client())
	swap(t, &AIChatFindOrder, func(who, id string) (AIChatOrder, bool) {
		return aiChatTestOrder, id == aiChatTestOrder.ID // another visitor's id finds nothing
	})
	swap(t, &AIChatLog, NewAIChatStore())

	a := trilha.New(trilha.ConfigFromEnv())
	a.Register(trilha.Route{Pattern: "/api/chat", Methods: map[string]trilha.HandlerFunc{"POST": AIChatPOST}})
	a.Register(trilha.Route{Pattern: "/orders/{id}", Page: func(c *trilha.Ctx) (h.Node, error) {
		return AIChatScreen(c, aiChatTestOrder), nil
	}})
	return trilha.NewTestClient(t, a), fake
}

// swap puts a value in a package var for one test and puts the old one back.
func swap[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func TestAIChatAnswersWithoutAKeyOrNetwork(t *testing.T) {
	c, fake := newAIChatTest(t, "Order **1043** shipped on Tuesday.")

	// The screen: the form, what the page knows, and the script.
	c.Get("/orders/1043").WantStatus(200).WantContains(
		`data-trilha-chat="/api/chat"`,
		`name="ctx.order_id" value="1043"`,
		`src="/ui.chat.js`,
	)

	// The submit that arrives when JavaScript is not there: the same route
	// answers it whole and sends the visitor back to the page.
	c.PostForm("/api/chat", url.Values{"message": {"where is my order?"}, "ctx.order_id": {"1043"}}).
		WantStatus(303).WantHeader("Location", "/orders/1043")

	// The page's context reached the model, and the instructions came first.
	sent := fake.Sent()
	if len(sent) < 3 || sent[0].Role != "system" {
		t.Fatalf("the agent's instructions must open the conversation: %#v", sent)
	}
	if !strings.Contains(sent[1].Content, "order 1043") || !strings.Contains(sent[1].Content, "shipped") {
		t.Errorf("the open order must reach the model: %#v", sent[1])
	}
	if last := sent[len(sent)-1]; last.Role != "user" || last.Content != "where is my order?" {
		t.Errorf("the question must be the last turn: %#v", last)
	}

	// The turn is in the log, and the page renders the answer as Markdown.
	if got := AIChatLog.Read("192.0.2.1"); len(got) != 2 || got[0].Text != "where is my order?" {
		t.Fatalf("conversation not stored: %#v", got)
	}
	c.Get("/orders/1043").WantContains("where is my order?", "<strong>1043</strong>")
}

func TestAIChatContextIsThePermissionCheck(t *testing.T) {
	c, fake := newAIChatTest(t, "ok")

	// An id the visitor does not own finds nothing, and the model is told
	// nothing about it — the lookup takes the owner, so it is the check.
	c.PostForm("/api/chat", url.Values{"message": {"hi"}, "ctx.order_id": {"9999"}}).WantStatus(303)
	sent := fake.Sent()
	if len(sent) < 2 || !strings.Contains(sent[1].Content, "no order open") {
		t.Fatalf("a foreign id must tell the model nothing: %#v", sent)
	}
	for _, m := range sent {
		if strings.Contains(m.Content, "Ada") {
			t.Fatalf("the record leaked into the conversation: %#v", m)
		}
	}
}

func TestAIChatLogKeepsItsCeiling(t *testing.T) {
	log := NewAIChatStore()
	for i := 0; i < AIChatMaxTurns; i++ {
		log.Append("ada", "q", "a")
	}
	got := log.Read("ada")
	if len(got) != AIChatMaxTurns {
		t.Fatalf("conversation is %d messages, want %d", len(got), AIChatMaxTurns)
	}
	if got[0].Role != "user" || got[len(got)-1].Role != "assistant" {
		t.Fatalf("the trim must keep whole turns: %#v", got[:2])
	}
	if n := len(AIChatPast(got)); n != AIChatMaxTurns {
		t.Fatalf("the model reads %d messages, want %d", n, AIChatMaxTurns)
	}
}
