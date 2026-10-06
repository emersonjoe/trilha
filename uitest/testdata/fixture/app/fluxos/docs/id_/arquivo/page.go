// Package arquivo serves the document's file for the frame, counted: a frame
// that reloads asks again.
package arquivo

import (
	"strings"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/h"

	"example.com/uitest/internal/conta"
)

func Page(c *trilha.Ctx) (h.Node, error) {
	conta.Hit(c)
	return nil, c.Inline("doc.txt", strings.NewReader("the text of document "+c.Param("id")), "text/plain")
}
