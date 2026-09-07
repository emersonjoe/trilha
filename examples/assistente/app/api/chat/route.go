// Package chat answers the assistant. The whole route is ai.Serve: it reads
// the message, runs the agent and streams the events ui.Chat listens for.
package chat

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/examples/assistente/internal/ferramentas"
	"github.com/emersonjoe/trilha/ui"
)

// POST runs the agent for one user message. ui.ChatHTML renders the finished
// answer, so the bubble that arrived word by word ends up as Markdown.
func POST(c *trilha.Ctx) error {
	return ai.ServeOpts{HTML: ui.ChatHTML}.Serve(c, ferramentas.Client, ferramentas.Assistente)
}
