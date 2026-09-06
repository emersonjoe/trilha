// Package llmsfulltxt serves /llms-full.txt — the whole documentation as one Markdown file.
// The folder has a dot in its name (spec 008), so the URL is a fixed path and
// the package name differs from the folder.
package llmsfulltxt

import (
	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// GET answers in plain text, for agents and other programs that read the
// documentation in bulk instead of one HTML page at a time.
func GET(c *trilha.Ctx) error {
	return c.Text(200, docs.LLMsFull("en", c.Base()))
}
