package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/internal/uidoc"
)

// patternTool is get_pattern: one of the kit's screen patterns, whole. It is
// part of ContextTools because it is the same kind of answer — what to read
// before writing — and it reads nothing but the table compiled into the
// binary.
func patternTool() *ai.Tool {
	return ai.NewTool(uidoc.PatternToolName, uidoc.PatternToolDescription,
		ai.Schema(uidoc.PatternToolSchema),
		ai.Typed(func(_ context.Context, in struct {
			Name string `json:"name"`
		}) (string, error) {
			p, names, ok := uidoc.FindPattern(in.Name)
			if !ok {
				return "", fmt.Errorf("no pattern named %q; there are: %s", in.Name, strings.Join(names, ", "))
			}
			return uidoc.PatternMarkdown(p), nil
		}),
	)
}
