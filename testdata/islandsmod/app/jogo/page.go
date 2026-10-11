package jogo

import (
	"github.com/google/uuid"

	"example.com/prosa/internal/api"
	contrato "example.com/prosa/internal/api"
)

// Props is what the game island receives.
type Props struct {
	Fila   []api.Card      `json:"fila"`
	Atual  *contrato.Card  `json:"atual,omitempty"`
	Nivel  api.Nivel       `json:"nivel"`
	Sessao uuid.UUID       `json:"sessao"`
}

func Page(c ctx) any { return c.Island("/jogo.js", Props{}, nil) }

type ctx interface{ Island(string, any, any) any }
