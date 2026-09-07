package ui

import (
	"strconv"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// ChatMessage is one turn already on the page. The assistant's text is
// Markdown and goes through Markdown; the visitor's is plain text and stays
// plain text — what someone typed is never markup.
type ChatMessage struct {
	// Role is "user" or "assistant". Anything else renders as a note.
	Role string
	Text string
}

// ChatOpts describes the conversation. Action is the only field that has to
// be filled in.
type ChatOpts struct {
	// ID is the element id, and the prefix of the ids inside it. Empty means
	// "chat"; two chats on one page need two ids.
	ID string

	// Action is the route that answers, the one running ai.Serve.
	Action string

	// History is what was said before, oldest first. It is the app's: the
	// framework keeps no session.
	History []ChatMessage

	// Greeting is the Markdown shown when the history is empty.
	Greeting string

	// Placeholder, Submit and Label are the words on the screen. Empty means
	// "Message", "Send" and "Conversation".
	Placeholder string
	Submit      string
	Label       string

	// MaxLength caps the field. Zero means 4000; a negative number lifts it.
	MaxLength int

	// Steps shows what the agent did — the tool it called and what came back.
	// Off by default: it is the app that knows whether the tools are worth
	// showing to whoever is reading.
	Steps bool

	// Markdown is how the answers are rendered. The zero value is the safe
	// one: headings demoted, no images, no raw HTML.
	Markdown MarkdownOpts
}

// Chat renders a conversation with an agent: the messages, the field and the
// button. The route on the other side is ai.Serve.
//
//	ui.Chat(c, ui.ChatOpts{Action: "/api/chat", History: msgs})
//	ui.ChatScript(c)
//
// With the script, the answer arrives word by word and the Markdown is
// rendered when the message ends. Without it the form still submits: the
// route answers the whole thing at once, and the page comes back with the
// answer in the history. Nothing on the screen depends on the script running.
func Chat(c *trilha.Ctx, o ChatOpts) h.Node {
	id := o.ID
	if id == "" {
		id = "chat"
	}
	placeholder := o.Placeholder
	if placeholder == "" {
		placeholder = "Message"
	}
	submit := o.Submit
	if submit == "" {
		submit = "Send"
	}
	label := o.Label
	if label == "" {
		label = "Conversation"
	}

	log := make([]h.Node, 0, len(o.History)+1)
	if len(o.History) == 0 && o.Greeting != "" {
		log = append(log, chatBubble(ChatMessage{Role: "assistant", Text: o.Greeting}, o.Markdown))
	}
	for _, m := range o.History {
		log = append(log, chatBubble(m, o.Markdown))
	}

	field := []h.Node{
		h.Name("message"), h.Placeholder(placeholder), h.Required(),
		h.Aria("label", placeholder), h.Attr("autocomplete", "off"),
	}
	if o.MaxLength >= 0 {
		n := o.MaxLength
		if n == 0 {
			n = 4000
		}
		field = append(field, h.Attr("maxlength", strconv.Itoa(n)))
	}

	attrs := []h.Node{h.Class("ui-chat"), h.ID(id), h.Data("trilha-chat", o.Action)}
	if o.Steps {
		attrs = append(attrs, h.Data("trilha-chat-steps", "1"))
	}

	return h.Div(append(attrs,
		h.Div(h.Class("ui-chat-log"), h.ID(id+"-log"),
			h.Attr("role", "log"), h.Aria("live", "polite"), h.Aria("label", label),
			h.Group(log...),
		),
		h.P(h.Class("ui-chat-state"), h.ID(id+"-state"), h.Aria("live", "polite")),
		h.Form(h.Class("ui-chat-form"), h.ID(id+"-form"),
			h.Method("post"), h.Action(o.Action),
			trilha.CSRFInput(c),
			Input(append(field, h.ID(id+"-input"))...),
			Submit(h.Text(submit)),
		),
	)...)
}

// chatBubble renders one turn. The visitor's text is text; the assistant's is
// Markdown — the same renderer the stream uses when the message ends, so a
// reloaded page looks like the one that streamed.
func chatBubble(m ChatMessage, opt MarkdownOpts) h.Node {
	switch m.Role {
	case "user":
		return h.Div(h.Class("ui-msg ui-msg-user"), h.Text(m.Text))
	case "assistant":
		return h.Div(h.Class("ui-msg ui-msg-assistant"), Markdown(m.Text, opt))
	default:
		return h.Div(h.Class("ui-msg ui-msg-note"), h.Text(m.Text))
	}
}

// ChatScript loads ui.chat.js, the behavior behind Chat: it sends the message,
// reads the events and renders the answer as it arrives. Put it once, in the
// layout of the area with the chat — Head does not load it, so a page without
// a chat does not download it.
func ChatScript(c *trilha.Ctx) h.Node {
	return h.Script(h.Src(c.Asset("/ui.chat.js")), h.Defer())
}

// ChatHTML renders one answer the way Chat renders the history. It is the
// adapter for ai.ServeOpts.HTML:
//
//	ai.ServeOpts{HTML: ui.ChatHTML}.Serve(c, client, agent)
//
// The done event then carries the rendered answer and the bubble stops being
// plain text. The HTML is this package's — escaped by construction — which is
// why handing it to the browser as HTML is safe and handing over a string from
// anywhere else is not.
func ChatHTML(text string) string {
	s, err := h.Render(Markdown(text, MarkdownOpts{}))
	if err != nil {
		s, err = h.Render(h.P(h.Text(text)))
		if err != nil {
			return ""
		}
	}
	return s
}
