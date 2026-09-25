package uidoc

import (
	"fmt"
	"strings"

	"github.com/emersonjoe/trilha/ui"
)

// FindPattern looks a pattern up by name, ignoring case and surrounding
// space. When there is none it answers the names that exist, so the caller
// can say them instead of saying nothing.
func FindPattern(name string) (ui.Pattern, []string, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	var names []string
	for _, p := range ui.Patterns() {
		if p.Name == want {
			return p, nil, true
		}
		names = append(names, p.Name)
	}
	return ui.Pattern{}, names, false
}

// PatternMarkdown is one pattern as the MCP tools answer it: everything a
// model needs to write the screen, the snippet last and whole.
func PatternMarkdown(p ui.Pattern) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n%s\n\n## Components\n\n", p.Name, p.Summary)
	for _, c := range p.Components {
		fmt.Fprintf(&sb, "- ui.%s\n", c)
	}
	fmt.Fprintf(&sb, "\n## Data\n\n%s\n\n## Accessibility\n\n", p.Data)
	for _, n := range p.A11y {
		fmt.Fprintf(&sb, "- %s\n", n)
	}
	fmt.Fprintf(&sb, "\n## page.go\n\n```go\n%s```\n", p.Snippet)
	return sb.String()
}

// PatternTool is the description and schema the get_pattern tool shares
// between the project's MCP server and the site's.
const (
	PatternToolName        = "get_pattern"
	PatternToolDescription = "Return one UI screen pattern of the Trilha kit (list-with-filter, async-form, approval-inbox, dashboard-chart, master-detail, upload-progress): the components, the data contract, the accessibility notes and a complete page.go of at most 60 lines that compiles. Ask for this before building a screen by hand."
	PatternToolSchema      = `{"type":"object","properties":{"name":{"type":"string","minLength":1}},"required":["name"]}`
)
