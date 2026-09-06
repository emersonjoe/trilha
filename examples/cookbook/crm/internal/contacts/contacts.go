// Package contacts is the data behind the app, held in a value instead of a
// package variable: the binary that hosts this app runs other things in the
// same process, and two apps sharing one global is a bug waiting for a test.
package contacts

import "sync"

// Contact is one person the CRM answers for.
type Contact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Store holds them. Setup creates one and provides it.
type Store struct {
	mu   sync.RWMutex
	list []Contact
}

// New returns an empty store.
func New() *Store { return &Store{} }

// All returns every contact.
func (s *Store) All() []Contact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Contact(nil), s.list...)
}

// Add stores one.
func (s *Store) Add(c Contact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, c)
}
