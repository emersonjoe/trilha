// Package contas is the register this app keeps: who has an account, in
// memory, for as long as the process lives.
package contas

import "sync"

// Conta is one row.
type Conta struct {
	Nome  string
	Email string
}

// Store is the table.
type Store struct {
	mu   sync.Mutex
	rows []Conta
}

// Nova adds a row.
func (s *Store) Nova(c Conta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, c)
}

// Todas returns every row, oldest first.
func (s *Store) Todas() []Conta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Conta(nil), s.rows...)
}
