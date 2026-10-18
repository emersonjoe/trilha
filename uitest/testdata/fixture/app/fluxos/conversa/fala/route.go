// Package fala answers the chat of /fluxos/conversa with 1500 tokens, flushed
// in batches the way a model's stream arrives.
package fala

import "github.com/emersonjoe/trilha"

// Tokens is how many text events the answer has.
const Tokens = 1500

// POST streams the answer.
func POST(c *trilha.Ctx) error {
	s := c.Stream()
	for i := 0; i < Tokens; i++ {
		if err := s.Send("text", "w "); err != nil {
			return err
		}
	}
	return s.Send("done", "{}")
}
