package legado

import "github.com/emersonjoe/trilha"

// The migrated area is browser only: its writes live in route.go files, one
// folder per action, the way the old app had them. Kind is inherited by the
// whole subtree, so this one line is what turns CSRF on for all of them — the
// alternative was the same line pasted into every leaf, and the leaf someone
// forgets is a write open to a form on another site.
var Kind = trilha.KindPage
