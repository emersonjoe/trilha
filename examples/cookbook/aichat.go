package cookbook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/h"
	"github.com/emersonjoe/trilha/ui"
)

// AIChatTimeout is how long one answer may take. The default client waits two
// minutes, which is longer than anybody sits in front of a support chat: a
// provider having a bad day should give up before the visitor does.
const AIChatTimeout = 60 * time.Second

// AIChatClient is the provider, built once at startup. The key never appears
// in the code and never in a log — it comes from the environment, which is the
// same place the database URL comes from.
var AIChatClient = newAIChatClient()

// newAIChatClient picks the provider from what the environment has. Anthropic
// answers the same chat-completions shape at its OpenAI-compatible base URL,
// so one Client covers both and the app has no provider branch past this
// function. With neither key the client still builds, and only a request
// fails: a missing key must not stop the app from starting.
func newAIChatClient() *ai.Client {
	c := ai.NewFromEnv() // OPENAI_API_KEY, OPENAI_BASE_URL, TRILHA_AI_MODEL
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" && os.Getenv("OPENAI_API_KEY") == "" {
		c.BaseURL, c.APIKey = "https://api.anthropic.com/v1", key
	}
	c.HTTPClient = &http.Client{Timeout: AIChatTimeout}
	return c
}

// AIChatConfig is the part of the assistant an administrator changes without a
// deploy: which model answers, how much it invents, and who it is told to be.
// The tags do three jobs at once — json stores it, form names the input, and
// validate is the rule, the same one on the screen and on anything else that
// saves it.
type AIChatConfig struct {
	Model        string  `json:"model"        form:"model"        validate:"max=60"            label:"Model"        help:"the name the provider knows; empty keeps the one the environment chose"`
	Temperature  float64 `json:"temperature"  form:"temperature"  validate:"min=0,max=2"        label:"Temperature"  help:"0 answers the same way every time; 2 invents"`
	Instructions string  `json:"instructions" form:"instructions" validate:"required,max=2000" label:"Instructions" help:"who the assistant is, and what it must not answer"`
}

// AIChatSettings is the section, with the defaults that answer before anybody
// has saved anything. It is a package var because configuration is not built
// per request — it is read on every one.
var AIChatSettings = trilha.NewSettings("ai-chat", AIChatConfig{
	Temperature: 0.2,
	Instructions: "You are the support assistant of an online store. Answer about the " +
		"order the visitor has open, in the language the question was asked in. " +
		"When the answer is not in what you were told, say so instead of guessing.",
})

// AIChatAgent is the agent of one request. It is built per request and from a
// copy of the settings, so changing the model on the administration screen
// takes effect on the next message — without a restart, and without a request
// already running seeing the agent change underneath it.
func AIChatAgent() *ai.Agent {
	cfg := AIChatSettings.Get()
	return &ai.Agent{
		Name:         "support",
		Instructions: cfg.Instructions,
		Model:        cfg.Model,
		Temperature:  &cfg.Temperature,
	}
}

// AIChatLimit is one bucket per visitor. The limit is not about abuse only: a
// chat is the one screen in an application where a single person can spend
// real money in a loop, and the ceiling belongs next to the route that spends
// it.
var AIChatLimit = trilha.NewLimiter(trilha.RateLimit{RPS: 0.2, Burst: 5})

// AIChatWho is the key of the limit and the owner of the conversation: the
// logged-in visitor when there is one, the address otherwise. It is the same
// answer both times on purpose — a conversation nobody owns is a conversation
// anybody could read.
func AIChatWho(c *trilha.Ctx) string {
	if a := c.Actor(); a.Subject != "" {
		return a.Subject
	}
	return c.ClientIP()
}

// AIChatPOST is app/api/chat/route.go, the whole route. ai.Serve reads the
// message, runs the agent and answers: a stream of events for ui.chat.js, the
// finished answer for a plain form submit. What the app adds around it is what
// only the app knows — who is asking, how often they may ask, and what the
// page they are on means.
func AIChatPOST(c *trilha.Ctx) error {
	who := AIChatWho(c)
	if ok, after := AIChatLimit.Allow(who); !ok {
		c.Header("Retry-After", strconv.Itoa(after))
		return trilha.Errorf(http.StatusTooManyRequests, "too many questions in a row; try again in %ds", after)
	}
	// The question is audited, the answer is not: the trail says who asked
	// what and when, and the conversation itself stays where it belongs.
	c.Audit("ai.chat.asked", who)
	return ai.ServeOpts{
		HTML:    ui.ChatHTML,
		Context: AIChatContext,
		Page:    AIChatAnswerPage,
	}.Serve(c, AIChatClient, AIChatAgent())
}

// AIChatOrder is the record the chat sits next to. It is the app's, and it is
// the only thing the model is told about: an assistant that can read the whole
// database is an assistant that will, eventually, read it out loud.
type AIChatOrder struct {
	ID       string
	Customer string
	Status   string
	Total    string
	Placed   time.Time
}

// AIChatFindOrder is the app's query, with the owner in the WHERE clause. It
// is a var because this file has no database; in an application it is the
// same function the order page calls. The default finds nothing, which is the
// safe way to notice it was never wired.
var AIChatFindOrder = func(who, id string) (AIChatOrder, bool) { return AIChatOrder{}, false }

// AIChatContext turns the ctx.* fields the page sent into the sentence the
// model reads before the conversation. It is what the page knows and the model
// does not.
//
// The fields come from the request, so they are what the visitor sent and not
// what the server knows: the lookup takes the owner as well as the id, which
// makes it the permission check. An id that is not theirs finds nothing, and
// the model is told nothing.
//
// The messages are user turns and not system ones. The history travels through
// the browser, so a run drops every system message it finds in it — that is
// what stops a page from being talked into new instructions, and it applies
// here too. What the app really wants to fix goes in Agent.Instructions, which
// no request can touch.
func AIChatContext(c *trilha.Ctx, fields map[string]string) []ai.Message {
	id := fields["order_id"]
	if id == "" {
		return nil
	}
	o, ok := AIChatFindOrder(AIChatWho(c), id)
	if !ok {
		return []ai.Message{ai.User("I have no order open. Answer only general questions about the store.")}
	}
	return []ai.Message{ai.User(fmt.Sprintf(
		"I am looking at order %s, placed on %s by %s. Status: %s. Total: %s. Answer about this order and no other.",
		o.ID, o.Placed.Format("2006-01-02"), o.Customer, o.Status, o.Total))}
}

// AIChatScreen is the page with the chat in it: the conversation, the field
// and the button, plus the script that makes the answer arrive word by word.
// Without the script the same form posts to the same route and the page comes
// back with the answer in it — nothing on the screen depends on JavaScript.
func AIChatScreen(c *trilha.Ctx, o AIChatOrder) h.Node {
	return h.Div(h.Class("ui-stack"),
		ui.Chat(c, ui.ChatOpts{
			Action:   "/api/chat",
			History:  AIChatLog.Read(AIChatWho(c)),
			Greeting: "Ask anything about this order — where it is, when it arrives, what it cost.",
			// What the page knows travels with every message, as JSON with
			// the script and as hidden fields without it.
			Context: map[string]string{"order_id": o.ID},
		}),
		ui.ChatScript(c),
	)
}

// AIChatAnswerPage answers the request that did not ask for a stream — the
// plain form submit, which is what arrives when JavaScript is not there. It
// runs after the whole answer is ready, so the turn goes into the log and the
// visitor is sent back to the page that has it. A redirect and not a render:
// reloading must not ask the model a second time.
func AIChatAnswerPage(c *trilha.Ctx, res *ai.Result) error {
	id := c.Form("ctx.order_id")
	AIChatLog.Append(AIChatWho(c), c.Form("message"), res.Output)
	if id == "" {
		return c.Redirect("/")
	}
	return c.Redirect("/orders/" + url.PathEscape(id))
}

// AIChatMaxTurns is how many messages one conversation keeps. The ceiling is
// the bill: every turn is sent again with the next question, so a conversation
// that never forgets is a conversation that costs more every time.
const AIChatMaxTurns = 20

// AIChatStore is the conversation, which belongs to the app — the framework
// keeps no chat session. This one is a map behind a mutex, which is the right
// size for one process and the wrong size for two: in an application it is a
// table keyed by (owner, order) with the same two methods, and the recipe for
// that table is the database one.
type AIChatStore struct {
	mu sync.Mutex
	by map[string][]ui.ChatMessage
}

// NewAIChatStore builds an empty store.
func NewAIChatStore() *AIChatStore { return &AIChatStore{by: map[string][]ui.ChatMessage{}} }

// AIChatLog is the store the routes above use.
var AIChatLog = NewAIChatStore()

// Read returns the conversation of one owner, oldest first, as the screen
// wants it. The copy is not politeness: the caller renders it while another
// request may be appending to it.
func (s *AIChatStore) Read(who string) []ui.ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ui.ChatMessage(nil), s.by[who]...)
}

// Append writes the turn that just happened and drops the oldest ones past
// AIChatMaxTurns.
func (s *AIChatStore) Append(who, question, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log := append(s.by[who],
		ui.ChatMessage{Role: "user", Text: question},
		ui.ChatMessage{Role: "assistant", Text: answer})
	if len(log) > AIChatMaxTurns {
		log = log[len(log)-AIChatMaxTurns:]
	}
	s.by[who] = log
}

// AIChatPast is the stored conversation in the shape the model reads.
func AIChatPast(msgs []ui.ChatMessage) []ai.Message {
	out := make([]ai.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "assistant" {
			out = append(out, ai.Assistant(m.Text))
			continue
		}
		out = append(out, ai.User(m.Text))
	}
	return out
}

// AIChatKeepPOST is the route to use when the transcript has to survive the
// page — an audit trail, a conversation two people continue, a chat that
// reloads where it stopped. It is ai.Serve with the two lines ai.Serve leaves
// no room for: the past comes from the app's store instead of from the
// browser, and the turn that just happened goes back into it.
//
// Everything else is the same contract: the same event names, the same answer
// without JavaScript, the same ui.Chat on the other side. A chat that only
// lives while the page is open does not need any of this — AIChatPOST is
// shorter and reads the history the browser already keeps.
func AIChatKeepPOST(c *trilha.Ctx) error {
	who := AIChatWho(c)
	message, err := aiChatMessage(c)
	if err != nil {
		return err
	}
	past := AIChatPast(AIChatLog.Read(who))
	agent := AIChatAgent()
	if !strings.Contains(c.Request().Header.Get("Accept"), "text/event-stream") {
		res, err := ai.Run(c.Context(), AIChatClient, agent, message, past...)
		if err != nil {
			return err
		}
		AIChatLog.Append(who, message, res.Output)
		return AIChatAnswerPage(c, res)
	}
	s := c.Stream()
	res, err := ai.RunStream(c.Context(), AIChatClient, agent, message, func(ev ai.Event) {
		if ev.Type == "text" {
			_ = s.Send("text", ev.Text)
		}
	}, past...)
	if err != nil {
		// The provider's own words never reach the screen: they are for the
		// log, where they say which provider failed and how.
		c.Log().Warn("ai chat", "err", err)
		return s.JSON("error", map[string]string{"message": AIChatFailure(err)})
	}
	AIChatLog.Append(who, message, res.Output)
	return s.JSON("done", map[string]any{"output": res.Output, "html": ui.ChatHTML(res.Output)})
}

// aiChatMessage reads the question from the two shapes the same screen sends:
// JSON when the script is there, a form field when it is not.
func aiChatMessage(c *trilha.Ctx) (string, error) {
	message := c.Form("message")
	if strings.HasPrefix(c.Request().Header.Get("Content-Type"), "application/json") {
		var in struct {
			Message string `json:"message"`
		}
		c.AllowBody(ai.DefaultServeInput)
		if err := c.BindJSON(&in); err != nil {
			return "", err
		}
		message = in.Message
	}
	if strings.TrimSpace(message) == "" {
		return "", trilha.Errorf(http.StatusUnprocessableEntity, "message is required")
	}
	return message, nil
}

// AIChatFailure is what the visitor reads when the provider says no. The
// sentence is the app's, in the app's language, and it says what to do next;
// the status code and the provider's message go to the log, where they are
// useful and where they leak nothing.
func AIChatFailure(err error) string {
	var perr *ai.Error
	switch {
	case errors.As(err, &perr) && perr.Status == http.StatusTooManyRequests:
		return "The assistant is answering a lot of people right now. Try again in a few seconds."
	case errors.Is(err, context.DeadlineExceeded):
		return "The answer took too long. Ask again, or ask something shorter."
	case errors.Is(err, ai.ErrMaxTurns):
		return "I could not finish this one. Try asking it in smaller parts."
	}
	return "The assistant is unavailable right now. Everything else on this page still works."
}

// AIChatFake is the provider in a test: it answers from a script and keeps
// what it was asked. No key, no bill and no network — the route, the screen
// and the context function run exactly as they do in production, because the
// only thing replaced is the http.Client the framework was always going to
// call.
type AIChatFake struct {
	// Answer is what the model says, every time.
	Answer string

	mu   sync.Mutex
	sent []ai.Message
}

// Client is the ai.Client to put where the real one goes.
func (f *AIChatFake) Client() *ai.Client {
	return &ai.Client{
		BaseURL:    "https://example.invalid/v1",
		APIKey:     "test-key",
		Model:      "test-model",
		HTTPClient: &http.Client{Transport: f},
	}
}

// Sent is the conversation of the last request: the instructions, the page's
// context and the turns, in the order the model read them. It is what a test
// asserts on to know the page's context reached the model.
func (f *AIChatFake) Sent() []ai.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ai.Message(nil), f.sent...)
}

// RoundTrip answers one assistant message, in the shape a chat-completions API
// answers it.
func (f *AIChatFake) RoundTrip(r *http.Request) (*http.Response, error) {
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
	f.sent = in.Messages
	f.mu.Unlock()

	body, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{
			"index":   0,
			"message": map[string]string{"role": "assistant", "content": f.Answer},
		}},
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

// AIChatCorner is the same chat in the corner of every page. ui.Assistant is
// ui.Chat inside a dialog with a launcher, and the launcher is a link before
// it is a button: with JavaScript off it goes to Page, the same conversation
// rendered as a page of its own. The route on the other side does not change.
func AIChatCorner(c *trilha.Ctx, o AIChatOrder) h.Node {
	return ui.Assistant(c, ui.AssistantOpts{
		ID:     "order-assistant",
		Action: "/api/chat",
		Page:   "/orders/" + url.PathEscape(o.ID) + "#chat",
		Label:  "Ask about this order",
		Title:  "Order assistant",
		Chat:   ui.ChatOpts{Context: map[string]string{"order_id": o.ID}},
	})
}
