package cookbook

import (
	"net/http"

	"github.com/emersonjoe/trilha"
	"github.com/emersonjoe/trilha/ui"
)

// PatternsJSON answers the kit's screen patterns as JSON — the same data
// `trilha ui patterns --json` prints and the MCP tool get_pattern answers,
// for an internal tool or an agent that reaches the app and not the CLI.
// Mount it where only the team can reach it: it is documentation, but it is
// yours to publish or not.
func PatternsJSON(c *trilha.Ctx) error {
	return c.JSON(http.StatusOK, ui.Patterns())
}
