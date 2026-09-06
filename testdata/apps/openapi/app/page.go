// Package app is the home page: a page never enters the OpenAPI document.
package app

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"
)

// Page renders the home page.
func Page(c *trilha.Ctx) (h.Node, error) { return h.P(h.Text("hello")), nil }
