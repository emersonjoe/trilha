package api

import "github.com/emersonjoe/trilha"

// This branch really is an API: it answers JSON to a client that sends a token
// in the header, not a form from a page of the site. Saying so out loud is what
// keeps the audit quiet about the writes below — and what makes the same
// declaration in app/legado-/kind.go mean something.
var Kind = trilha.KindAPI
