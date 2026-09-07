package ui

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// chatPage renders node inside a real request, which is what Chat needs for
// the CSRF field and ChatScript for the asset path.
func chatPage(t *testing.T, fn func(c *trilha.Ctx) h.Node) string {
	t.Helper()
	a := trilha.New(trilha.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	var out string
	a.Register(trilha.Route{Pattern: "/", Page: func(c *trilha.Ctx) (h.Node, error) {
		out = render(t, fn(c))
		return h.Div(), nil
	}})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	return out
}

// TestChatRendersTheConversation is SC-011: the history, the form and the
// announcement, all there before any script runs.
func TestChatRendersTheConversation(t *testing.T) {
	got := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Chat(c, ChatOpts{
			Action: "/api/chat",
			History: []ChatMessage{
				{Role: "user", Text: "o que **isto** faz?"},
				{Role: "assistant", Text: "Ele **responde**."},
			},
		})
	})
	for _, want := range []string{
		`class="ui-chat"`, `id="chat"`, `data-trilha-chat="/api/chat"`,
		`role="log"`, `aria-live="polite"`,
		`<form class="ui-chat-form" id="chat-form" method="post" action="/api/chat"`,
		`name="_csrf"`, `name="message"`, `maxlength="4000"`, `>Send</button>`,
		`class="ui-msg ui-msg-assistant"`, `<strong>responde</strong>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	// What the visitor typed is text, even when it looks like Markdown.
	if !strings.Contains(got, `<div class="ui-msg ui-msg-user">o que **isto** faz?</div>`) {
		t.Fatalf("the visitor's message was interpreted: %s", got)
	}
}

// TestChatGreetingAndSteps: the greeting only shows on an empty conversation,
// and the tool trail is opt-in.
func TestChatGreetingAndSteps(t *testing.T) {
	empty := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Chat(c, ChatOpts{Action: "/a", Greeting: "Ask me *anything*.", Steps: true, ID: "ajuda"})
	})
	if !strings.Contains(empty, "<em>anything</em>") || !strings.Contains(empty, `data-trilha-chat-steps="1"`) {
		t.Fatalf("greeting or steps missing: %s", empty)
	}
	if !strings.Contains(empty, `id="ajuda-log"`) || !strings.Contains(empty, `id="ajuda-input"`) {
		t.Fatalf("the id did not reach the parts: %s", empty)
	}

	full := chatPage(t, func(c *trilha.Ctx) h.Node {
		return Chat(c, ChatOpts{Action: "/a", Greeting: "Ask me anything.",
			History: []ChatMessage{{Role: "user", Text: "oi"}}})
	})
	if strings.Contains(full, "Ask me anything.") {
		t.Fatalf("the greeting stayed on a conversation that already started: %s", full)
	}
	if strings.Contains(full, "data-trilha-chat-steps") {
		t.Fatalf("the tool trail is not opt-in: %s", full)
	}
}

// TestChatScriptIsOptIn is the other half of SC-012: Head does not carry it.
func TestChatScriptIsOptIn(t *testing.T) {
	got := chatPage(t, ChatScript)
	if !strings.Contains(got, `src="/ui.chat.js" defer`) {
		t.Fatalf("ChatScript renders %q", got)
	}
	head := chatPage(t, Head)
	if strings.Contains(head, "ui.chat.js") {
		t.Fatalf("Head loads the chat script: %q", head)
	}
	if len(Asset("ui.chat.js")) > 8<<10 {
		t.Fatalf("ui.chat.js is %d bytes", len(Asset("ui.chat.js")))
	}
	found := false
	for _, f := range Files {
		if f == "ui.chat.js" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ui.chat.js is not in Files: %v", Files)
	}
}

// TestChatHTMLIsTheSameRenderer: what the stream hands the bubble at the end
// is what a reloaded page would have rendered.
func TestChatHTMLIsTheSameRenderer(t *testing.T) {
	got := ChatHTML("Veja o **relatório** e <script>alert(1)</script>.")
	if !strings.Contains(got, "<strong>relatório</strong>") {
		t.Fatalf("no markdown: %q", got)
	}
	if strings.Contains(got, "<script") {
		t.Fatalf("raw HTML got through: %q", got)
	}
}
