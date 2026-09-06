// Package store is the data the API answers with.
package store

import "time"

// Item is one thing on sale.
type Item struct {
	ID string `json:"id"`
	// Name is what the buyer reads.
	Name    string    `json:"name" validate:"required,max=40"`
	Kind    string    `json:"kind" validate:"required,oneof=book tool"`
	Price   float64   `json:"price" validate:"min=0"`
	Tags    []string  `json:"tags" validate:"max=5"`
	Owner   *Owner    `json:"owner,omitempty"`
	Created time.Time `json:"created"`
	Note    string    `json:"-"`
	hidden  string
}

// Owner is whoever answers for an item.
type Owner struct {
	Email string `json:"email" validate:"required,email"`
}

// All returns every item.
func All() []Item { return nil }

// Get returns one item by id.
func Get(id string) (Item, bool) { return Item{}, false }

// Create stores an item and returns it.
func Create(name string) Item { return Item{Name: name} }

// Delete removes an item.
func Delete(id string) bool { return false }
