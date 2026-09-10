// Chat over HTTP: the bridge between an agent run and a browser.

package ai

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
)

// DefaultServeHistory is how many past messages travel back into the model
// when ServeOpts says nothing. The ceiling is here because the history comes
// from the browser: without it, one request decides how much the app pays.
const DefaultServeHistory = 40

// DefaultServeInput is the largest request body Serve reads by default.
const DefaultServeInput = 256 << 10

// ServeOpts adjusts what Serve accepts and how it answers a client that did
// not ask for a stream. The zero value is what Serve uses.
type ServeOpts struct {
	// MaxHistory is how many past messages are kept, counting from the end.
	// Zero means DefaultServeHistory; a negative number keeps none.
	MaxHistory int

	// MaxInput is the largest request body accepted, in bytes. Zero means
	// DefaultServeInput.
	MaxInput int64

	// HTML renders the finished answer for the browser — ui.ChatHTML is the
	// one that matches ui.Chat. When it is set, the done event and the JSON
	// answer carry an html field beside the text, so the bubble that arrived
	// word by word ends up rendered like the ones already on the page. When
	// it is nil the answer stays text, which is the safe thing to do with a
	// renderer nobody chose.
	HTML func(text string) string

	// Context receives the opaque fields the page sent with the message — the
	// ctx.* fields of ui.ChatOpts.Context: the id of the record that is open,
	// the route the visitor is on. What it returns goes in front of the
	// history, which is how the page's context reaches the model.
	//
	// It is a function and not a template because only the app knows what a
	// field means: turning "invoice=12" into a sentence, or into nothing, is a
	// decision, and a framework that guessed it would be wrong in every second
	// application.
	//
	// The fields come from the request, so they are what the visitor sent and
	// not what the server knows: check what an id gives access to here, the
	// same way a query parameter is checked.
	Context func(c *trilha.Ctx, fields map[string]string) []Message

	// Page answers a request that did not ask for text/event-stream — the
	// form that arrives when JavaScript is not there. It runs after the whole
	// answer is ready, so the app can render the page with the message in it.
	// When it is nil, Serve replies with the same JSON the done event carries.
	Page func(c *trilha.Ctx, res *Result) error
}

// Serve runs the agent for one message and answers the request. It is the
// route half of ui.Chat:
//
//	func POST(c *trilha.Ctx) error {
//		return ai.Serve(c, client, assistant)
//	}
//
// The request carries {"message": "...", "history": [...]} as JSON, or a
// message field in a form. When the client asks for text/event-stream the
// answer is a stream of named events — text, tool_call, tool_result, handoff,
// done, error — and otherwise the whole answer at once, so the same route
// works with the script and without it.
func Serve(c *trilha.Ctx, cli *Client, agent *Agent) error {
	return ServeOpts{}.Serve(c, cli, agent)
}

// Serve is Serve with the options set.
func (o ServeOpts) Serve(c *trilha.Ctx, cli *Client, agent *Agent) error {
	msg, history, err := o.read(c)
	if err != nil {
		return err
	}
	if !wantsStream(c) {
		res, err := Run(c.Context(), cli, agent, msg, history...)
		if err != nil {
			return err
		}
		if o.Page != nil {
			return o.Page(c, res)
		}
		return c.JSON(http.StatusOK, o.done(res))
	}

	s := c.Stream()
	failed := false
	res, err := RunStream(c.Context(), cli, agent, msg, func(ev Event) {
		switch ev.Type {
		case "text":
			_ = s.Send("text", ev.Text)
		case "tool_call", "tool_result", "handoff":
			_ = s.JSON(ev.Type, step(ev))
		case "done":
			_ = s.JSON("done", o.done(ev.Result))
		case "error":
			failed = true
			_ = s.JSON("error", map[string]string{"message": ev.Err.Error()})
		}
	}, history...)
	if err != nil && !failed {
		// The run ended before it could announce itself. The status line is
		// long gone, so the failure travels as an event like any other.
		_ = s.JSON("error", map[string]string{"message": err.Error()})
	}
	if err != nil {
		c.Log().Warn("ai.Serve", "agent", agent.Name, "err", err)
	} else if res == nil {
		_ = s.JSON("error", map[string]string{"message": "no answer"})
	}
	return nil
}

// serveInput is what the browser sends. Two shapes are read: the pair the
// stream client uses, and a plain messages list, where the last user turn is
// the message and everything before it is the history.
type serveInput struct {
	Message  string            `json:"message"`
	History  []Message         `json:"history"`
	Messages []Message         `json:"messages"`
	Context  map[string]string `json:"context"`
}

func (o ServeOpts) read(c *trilha.Ctx) (string, []Message, error) {
	var in serveInput
	if strings.HasPrefix(c.Request().Header.Get("Content-Type"), "application/json") {
		max := o.MaxInput
		if max == 0 {
			max = DefaultServeInput
		}
		c.AllowBody(max)
		if err := c.BindJSON(&in); err != nil {
			return "", nil, err
		}
	} else {
		in.Message = c.Form("message")
		// Without the script the same fields arrive as what they are on the
		// page: hidden inputs. Reading both here is what keeps the route with
		// one way to see the context.
		for name, values := range c.Request().Form {
			if after, ok := strings.CutPrefix(name, "ctx."); ok && len(values) > 0 {
				if in.Context == nil {
					in.Context = map[string]string{}
				}
				in.Context[after] = values[0]
			}
		}
	}

	msg, history := in.Message, in.History
	if msg == "" && len(in.Messages) > 0 {
		last := in.Messages[len(in.Messages)-1]
		if last.Role == "user" {
			msg, history = last.Content, in.Messages[:len(in.Messages)-1]
		}
	}
	if strings.TrimSpace(msg) == "" {
		return "", nil, trilha.Errorf(http.StatusUnprocessableEntity, "message is required")
	}

	keep := o.MaxHistory
	if keep == 0 {
		keep = DefaultServeHistory
	}
	if keep < 0 {
		keep = 0
	}
	if len(history) > keep {
		history = history[len(history)-keep:]
	}
	// The context goes in front of the history, and after the trim: it is the
	// part of the conversation that is not a turn, and losing it because the
	// conversation got long would be losing the only thing the page knew.
	if o.Context != nil {
		if extra := o.Context(c, in.Context); len(extra) > 0 {
			history = append(append([]Message{}, extra...), history...)
		}
	}
	return msg, history, nil
}

// wantsStream reports whether the client asked for events. The header is read
// whole instead of by content negotiation on purpose: a browser sends */* in
// every form post, and that would turn a plain submit into a stream nobody is
// listening to.
func wantsStream(c *trilha.Ctx) bool {
	return strings.Contains(c.Request().Header.Get("Accept"), "text/event-stream")
}

// step is the payload of tool_call, tool_result and handoff.
func step(ev Event) map[string]any {
	m := map[string]any{"agent": ev.Agent}
	if ev.Step == nil {
		return m
	}
	m["tool"] = ev.Step.Tool
	m["call_id"] = ev.Step.CallID
	m["arguments"] = ev.Step.Arguments
	m["output"] = ev.Step.Output
	if ev.Step.HandoffTo != "" {
		m["to"] = ev.Step.HandoffTo
	}
	if ev.Step.Err != nil {
		m["error"] = ev.Step.Err.Error()
	}
	return m
}

// done is the payload of the last event, and the body of the answer given to
// a client that did not ask for a stream. The history goes back so the next
// request can send it again — the framework keeps no session.
func (o ServeOpts) done(res *Result) map[string]any {
	if res == nil {
		return map[string]any{}
	}
	name := ""
	if res.Agent != nil {
		name = res.Agent.Name
	}
	m := map[string]any{
		"agent":   name,
		"output":  res.Output,
		"history": res.Messages,
		"usage":   res.Usage,
	}
	if o.HTML != nil {
		m["html"] = o.HTML(res.Output)
	}
	return m
}
