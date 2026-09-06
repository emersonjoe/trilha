package ctx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// View decides how much of the map gets printed. The compact view is the
// default because the map is read every turn: what it elides is what is the
// same in every project (the shape of an error) or what is rarely the
// question (which middleware guards which single method).
type View int

const (
	Compact View = iota
	OnlyRoutes
	OnlyTypes
	All
)

// problemType is the error body the runtime answers with, the same in every
// project; the compact view says it once instead of once per status.
const problemType = "Problem"

// Markdown renders the map for a reader that pays by the token.
func (c *Context) Markdown(v View) string {
	var sb strings.Builder
	if v == Compact || v == All {
		fmt.Fprintf(&sb, "# %s\n\n", c.Module)
		fmt.Fprintf(&sb, "- trilha %s · %s\n", c.Trilha, count(c.Routes))
		fmt.Fprintf(&sb, "- %s: %s\n", c.Generated.File, c.Generated.Status)
		if c.Setup != nil {
			fmt.Fprintf(&sb, "- %s: %s\n", c.Setup.File, strings.Join(c.Setup.Funcs, ", "))
			for _, val := range c.Setup.Values {
				fmt.Fprintf(&sb, "  - provides %s\n", value(val))
			}
		}
		sb.WriteString("\n")
	}
	if v != OnlyTypes {
		c.routes(&sb, v)
	}
	if v == Compact || v == All {
		c.api(&sb, v)
	}
	if v != OnlyRoutes {
		c.types(&sb, v)
	}
	return sb.String()
}

func value(v Value) string {
	if v.Type == "" {
		return "`" + v.From + "`"
	}
	return "`" + v.Type + "` (`" + v.From + "`)"
}

func count(rs []Route) string {
	pages, apis := 0, 0
	for _, r := range rs {
		if r.Kind == "page" {
			pages++
		} else {
			apis++
		}
	}
	return fmt.Sprintf("%s (%s, %s)", plural(len(rs), "route"), plural(pages, "page"), plural(apis, "API"))
}

func plural(n int, what string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, what)
	}
	return fmt.Sprintf("%d %ss", n, what)
}

func (c *Context) routes(sb *strings.Builder, v View) {
	sb.WriteString("## Routes\n\n")
	for _, r := range c.Routes {
		fmt.Fprintf(sb, "- `%s %s` — %s", strings.Join(r.Methods, " "), r.Pattern, r.File)
		if len(r.Layouts) > 0 {
			fmt.Fprintf(sb, " · layouts: %s", strings.Join(r.Layouts, ", "))
		}
		if len(r.Middlewares) > 0 {
			fmt.Fprintf(sb, " · middleware: %s", strings.Join(r.Middlewares, ", "))
		}
		if v == All && len(r.MiddlewaresByMethod) > 0 {
			var ms []string
			for m := range r.MiddlewaresByMethod {
				ms = append(ms, m)
			}
			sort.Strings(ms)
			for _, m := range ms {
				fmt.Fprintf(sb, " · %s: %s", m, strings.Join(r.MiddlewaresByMethod[m], ", "))
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
}

func (c *Context) api(sb *strings.Builder, v View) {
	var any bool
	for _, r := range c.Routes {
		if len(r.API) > 0 {
			any = true
		}
	}
	if !any {
		return
	}
	sb.WriteString("## API\n\n")
	for _, r := range c.Routes {
		for _, op := range r.API {
			fmt.Fprintf(sb, "### %s %s\n", op.Method, r.Pattern)
			if op.Summary != "" {
				fmt.Fprintf(sb, "%s\n", op.Summary)
			}
			if len(op.Query) > 0 {
				fmt.Fprintf(sb, "- query: %s\n", strings.Join(op.Query, ", "))
			}
			if op.Request != "" {
				fmt.Fprintf(sb, "- body `%s`\n", op.Request)
			}
			var errs []string
			for _, resp := range op.Responses {
				if v != All && resp.Status >= 400 && resp.Type == problemType {
					errs = append(errs, strconv.Itoa(resp.Status))
					continue
				}
				line := fmt.Sprintf("- %d", resp.Status)
				if resp.Type != "" {
					line += " `" + resp.Type + "`"
				}
				if resp.Media != "" {
					line += " (" + resp.Media + ")"
				}
				fmt.Fprintf(sb, "%s\n", line)
			}
			if len(errs) > 0 {
				fmt.Fprintf(sb, "- errors: %s (`%s`)\n", strings.Join(errs, ", "), problemType)
			}
			sb.WriteString("\n")
		}
	}
}

func (c *Context) types(sb *strings.Builder, v View) {
	var shown []Type
	for _, t := range c.Types {
		if v != All && t.Name == problemType {
			continue
		}
		shown = append(shown, t)
	}
	if len(shown) == 0 {
		return
	}
	sb.WriteString("## Types\n\n")
	for _, t := range shown {
		fmt.Fprintf(sb, "### %s\n", t.Name)
		for _, f := range t.Fields {
			line := fmt.Sprintf("- `%s` %s", f.Name, f.Type)
			if f.Required {
				line += ", required"
			}
			if f.Rules != "" {
				line += ", " + f.Rules
			}
			fmt.Fprintf(sb, "%s\n", line)
		}
		sb.WriteString("\n")
	}
	if v != All {
		sb.WriteString("Errors answer `application/problem+json` (RFC 9457): `type`, `title`, `status`, `detail`, `errors`.\n")
	}
}
