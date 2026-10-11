// Package api is the contract trilha client wrote.
package api

import "example.com/prosa/internal/api/tipos_"

// Card is one card of the study queue.
type Card struct {
	ID     string      `json:"id"`
	Frente string      `json:"frente"`
	Tags   []tipos.Tag `json:"tags"`
}

// Nivel is how hard the round is.
type Nivel int
