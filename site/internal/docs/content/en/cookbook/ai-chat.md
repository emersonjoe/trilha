---
title: AI chat
description: A chat with a model inside your app, from zero to the screen — where the key lives, what the page tells the model, who keeps the history, what happens when the provider says no, and how the test runs without a key.
---

[Learn](/learn/ai-and-agents) teaches the pieces: a call, a tool, an agent, `ai.Serve` and
`ui.Chat`. This is the whole thing assembled, in the order an application needs it, with the
decisions nobody writes down: where the key lives, what the page tells the model, who keeps
the history, what the visitor reads when the provider is having a bad day, and how the test
runs with no key and no network.

The screen it ends at is [the demo](/demos/ai-chat): a support chat next to the order it is
about.

## 1. The client

The key comes from the environment, like the database URL. It is never in the code, never in
a commit, and never in a log — `ai.Client` keeps it out of both.

```go
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
```

The timeout is a decision and not a default:

```go
// AIChatTimeout is how long one answer may take. The default client waits two
// minutes, which is longer than anybody sits in front of a support chat: a
// provider having a bad day should give up before the visitor does.
const AIChatTimeout = 60 * time.Second
```

:::note
`ai.Client` speaks the chat-completions shape. OpenAI, Anthropic (at its OpenAI-compatible
base URL), Groq, OpenRouter, Ollama and vLLM all answer it, so switching provider is a base
URL and a key, not a rewrite.
:::

What an administrator changes without a deploy is not the key — it is the model, how much it
invents, and who the assistant is told to be. That is a [settings
section](/reference/app#settings): a struct, its tags and one screen the framework builds
from it.

```go
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
```

The agent is built per request, from a copy of the settings, so a change on the
administration screen takes effect on the next message:

```go
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
```

:::note
Putting the provider key in the section as well is a real option — `trilha.Secret` keeps it
encrypted at rest and never sends it back to the screen. Do it when someone other than the
deploy pipeline has to rotate it.
:::

## 2. The route

The route is `ai.Serve` and what only the app knows around it: who is asking, how often they
may ask, and what the page they are on means.

```go
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
```

`ai.Serve` reads the message, runs the agent and answers twice over: a stream of named events
when the client asked for `text/event-stream`, the whole answer at once when it did not. The
second one is what arrives when JavaScript is off — the same route, the same agent, the same
audit line.

```go
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
```

A redirect and not a render, for the reason every form has one: reloading the page must not
ask the model a second time.

## 3. The screen

`ui.Chat` is the conversation, the field and the button; `ui.ChatScript` is the client that
makes the answer arrive word by word. Neither of them is required for the page to work.

```go
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
```

`Context` is the part that makes an assistant useful instead of impressive: what the page
knows and the model does not. Each entry becomes a hidden `ctx.<key>` field, sent with the
message by the script and by the plain form alike, and read on the other side by
`ai.ServeOpts.Context`:

```go
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
```

The record it reads is the app's, and it is the only thing the model is told about:

```go
// AIChatFindOrder is the app's query, with the owner in the WHERE clause. It
// is a var because this file has no database; in an application it is the
// same function the order page calls. The default finds nothing, which is the
// safe way to notice it was never wired.
var AIChatFindOrder = func(who, id string) (AIChatOrder, bool) { return AIChatOrder{}, false }
```

:::note
The `ctx.*` fields come from the request, so they are what the visitor sent — not what the
server knows. The lookup takes the owner as well as the id, which is what makes it the
permission check instead of a lookup. Treat an id there exactly like an id in the URL.
:::

Model text is Markdown, and `ui.ChatHTML` is the adapter that renders it the same way the
history is rendered: `<script>` in an answer is text on the page, not a tag in the document.

## 4. The history

The history belongs to the app. The framework keeps no chat session, and that is deliberate:
a conversation is a record, and where records live is not a framework's decision.

With the script, the conversation travels with each message and `ai.ServeOpts.MaxHistory`
caps how much of it goes back to the model — the browser sends it, so the ceiling is on the
server. That is enough for a chat that lives while the page is open.

When the transcript has to survive the page, the app keeps it. This store is a map behind a
mutex, which is the right size for one process and the wrong size for two:

```go
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
```

```go
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
```

The ceiling is the bill, not tidiness: every turn is sent again with the next question.

And the route that reads and writes it is `ai.Serve` plus the two lines `ai.Serve` leaves no
room for — the past comes from the store instead of from the browser, and the turn that just
happened goes back into it:

```go
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
```

```go
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
```

:::note
In an application the store is a table: `(owner, order_id, role, text, at)` with the same two
methods, written with the [database recipe](/cookbook/database). Keeping it in memory is fine
for one process and wrong for two — the second instance answers with a conversation the first
one never saw.
:::

## 5. Limits and failures

A chat is the one screen where a single person can spend real money in a loop. The ceiling
belongs next to the route that spends it:

```go
// AIChatLimit is one bucket per visitor. The limit is not about abuse only: a
// chat is the one screen in an application where a single person can spend
// real money in a loop, and the ceiling belongs next to the route that spends
// it.
var AIChatLimit = trilha.NewLimiter(trilha.RateLimit{RPS: 0.2, Burst: 5})
```

Over the limit, the route answers `429` with `Retry-After`. `ui.chat.js` puts the question
back in the field and shows the status as a note, so the visitor loses nothing but time.

When the provider itself says no, what the visitor reads is the app's sentence, in the app's
words, saying what to do next — never the provider's:

```go
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
```

The provider's message goes to the log, where it is useful and where it leaks nothing. And
the question is audited, which is the line an application needs long before it needs
analytics:

```go
	c.Audit("ai.chat.asked", who)
```

## 6. Testing without a key

The test replaces one thing: the `http.Client` the framework was always going to call. No
key, no bill, no network and no flake — the route, the screen and the context function run
exactly as they do in production.

```go
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
```

The test then asserts what actually matters: that the page's context reached the model, that
the answer came back rendered, and that the screen still works with JavaScript off.

```go
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
```

```go
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
```

:::note
`make test` runs this file on every push. A recipe whose test needs a key is a recipe that
stops being run, and then stops being true.
:::

## 7. The demo

[The chat above, running](/demos/ai-chat). The component, the streaming client and the
context the page sends are the real ones; only the model is scripted, in the browser, because
this site is static and has no server to answer.

## 8. The same chat in the corner

`ui.Assistant` is this chat inside a dialog with a launcher, for the app that wants it on
every page instead of on one:

```go
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
```

The route on the other side does not change. The launcher is a link before it is a button:
with JavaScript off it goes to `Page`, the same conversation rendered as a page of its own —
which is [the assistant demo](/demos/assistant).

## Where to go next

- [AI and agents](/learn/ai-and-agents) — tools, agents, streaming, MCP.
- [`ai` reference](/reference/ai) — every symbol, including the event contract.
- [`ui.Chat`](/reference/ui#chat) — every field of `ChatOpts`.
- [`examples/assistente`](https://github.com/emersonjoe/trilha/tree/main/examples/assistente) —
  the same pieces in a complete application, with tools and an administration screen.
