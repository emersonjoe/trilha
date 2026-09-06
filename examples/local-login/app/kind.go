package app

import "github.com/emersonjoe/trilha"

// The whole app is browser pages: every write below here needs the CSRF token,
// including the ones written as route.go. One line at the root of the tree,
// inherited by every leaf — including the leaf somebody adds next month.
var Kind = trilha.KindPage
