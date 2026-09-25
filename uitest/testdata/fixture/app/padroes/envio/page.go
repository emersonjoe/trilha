// Package envio serves the upload-progress pattern as it is.
package envio

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/examples/patterns/uploadprogress"
	"github.com/emersonjoe/trilha/h"
)

func Page(c *trilha.Ctx) (h.Node, error) { return uploadprogress.Page(c) }

func POST(c *trilha.Ctx) error { return uploadprogress.POST(c) }
