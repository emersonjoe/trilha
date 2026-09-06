// kind.go says what a whole branch is, the way layout.go and middleware.go do:
// this site is pages, so every route.go below it enforces CSRF.
package app

import "github.com/emersonjoe/trilha"

var Kind = trilha.KindPage
