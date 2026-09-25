// Package semnonce is the negative control of the security scenarios: an
// inline script written without trilha.NonceAttr, which the policy refuses.
// A check that never sees a violation here is a check that sees nothing.
package semnonce

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	return h.Div(h.H1(h.Text("No nonce")), h.Script(h.Raw(`document.title = "ran"`))), nil
}
