// Package chat streams agent runs to the browser over Server-Sent Events.
package chat

import (
	"net/http"
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
)

type input struct {
	Message string       `json:"message"`
	History []ai.Message `json:"history"`
}

// POST runs the agent for one user message and streams events:
// text (delta), tool_call, tool_result, handoff, done (with history), error.
func POST(c *trilha.Ctx) error {
	var in input
	if err := c.BindJSON(&in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Message) == "" {
		return trilha.Errorf(http.StatusUnprocessableEntity, "message is required")
	}
	if len(in.History) > 40 {
		in.History = in.History[len(in.History)-40:]
	}
	s := c.Stream()
	_, err := ai.RunStream(c.Context(), ferramentas.Client, ferramentas.Assistente, in.Message, func(ev ai.Event) {
		switch ev.Type {
		case "text":
			_ = s.Send("text", ev.Text)
		case "tool_call", "tool_result", "handoff":
			_ = s.JSON(ev.Type, map[string]any{"agent": ev.Agent, "tool": ev.Step.Tool, "arguments": ev.Step.Arguments, "output": ev.Step.Output, "to": ev.Step.HandoffTo})
		case "done":
			_ = s.JSON("done", map[string]any{"agent": ev.Agent, "output": ev.Result.Output, "history": ev.Result.Messages, "usage": ev.Result.Usage})
		case "error":
			_ = s.JSON("error", map[string]string{"message": ev.Err.Error()})
		}
	}, in.History...)
	if err != nil {
		c.App().Logger().Warn("chat", "err", err)
	}
	return nil
}
