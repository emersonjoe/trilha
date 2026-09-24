package ctx

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/tokbudget"
)

// A pack is the map sliced for one job, under a token budget. The cut order
// is the contract, cheapest information first — and it never cuts routes:
//
//	contracts (API paths)  — what a handler binds and answers
//	recipes                — what is installed and which files it brings
//	conventions            — which conventions the tree actually uses
//	routes                 — the minimum useful; always kept
//
// What did not fit is named in Truncated, in that order, so a strict caller
// knows exactly what to ask for again and with which budget. Every rendered
// form ends with the cost estimate, labeled est., because a budget number
// that lies about being an estimate is worse than no number.
//
// RouteLine is one route of the pack: the methods, the pattern, the file and
// the middlewares around it.
type RouteLine struct {
	Methods     string   `json:"methods"`
	Pattern     string   `json:"pattern"`
	File        string   `json:"file"`
	Middlewares []string `json:"middlewares,omitempty"`
}

// RecipeInfo is one installed recipe: its name, where its documentation
// lives, and the files it brought into the project.
type RecipeInfo struct {
	Name  string   `json:"name"`
	Doc   string   `json:"doc,omitempty"`
	Files []string `json:"files,omitempty"`
}

// Pack is the budgeted slice, and the shape of `trilha ctx --pack … --json`.
type Pack struct {
	Module      string       `json:"module"`
	Routes      []RouteLine  `json:"routes"`
	Recipes     []RecipeInfo `json:"recipes,omitempty"`
	Conventions []string     `json:"conventions,omitempty"`
	APIPaths    []string     `json:"api_paths,omitempty"`
	Budget      int          `json:"budget_tokens,omitempty"`
	Used        int          `json:"used_tokens"`
	Truncated   []string     `json:"truncated,omitempty"`

	rendered string `json:"-"`
}

// PackOf slices the map. name "app" takes the whole map; any other name is a
// recipe's, and only the routes whose files that recipe touches stay.
// installed is what the caller detected in the project — the ctx package
// reads no registry. budget 0 means no budget.
func (c *Context) PackOf(name string, installed []RecipeInfo, budget int) (*Pack, error) {
	p := &Pack{Module: c.Module, Budget: budget}
	if name != "app" {
		var chosen *RecipeInfo
		for i := range installed {
			if installed[i].Name == name {
				chosen = &installed[i]
				break
			}
		}
		if chosen == nil {
			var names []string
			for _, r := range installed {
				names = append(names, r.Name)
			}
			return nil, fmt.Errorf("recipe %q is not installed here (installed: %s)", name, orNone(names))
		}
		p.Recipes = []RecipeInfo{*chosen}
		p.Routes = routesTouched(c, chosen.Files)
	} else {
		p.Routes = allRoutes(c)
		p.Recipes = installed
	}
	p.Conventions = conventions(c, name != "app")
	p.APIPaths = apiPaths(p.Routes)
	p.budget()
	return p, nil
}

// JSON is the same pack, indented, with a trailing newline.
func (p *Pack) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// budget applies the cut order and measures what survived. Routes are
// rendered first and never cut; the rest is appended while it fits.
func (p *Pack) budget() {
	body := p.renderRoutes()
	rest := []struct {
		name   string
		render func(*strings.Builder)
	}{
		{"contracts", func(sb *strings.Builder) { p.renderContracts(sb) }},
		{"recipes", func(sb *strings.Builder) { p.renderRecipes(sb) }},
		{"conventions", func(sb *strings.Builder) { p.renderConventions(sb) }},
	}
	for _, s := range rest {
		var alone strings.Builder
		s.render(&alone)
		if alone.Len() == 0 {
			continue // nothing to say: not a cut, just absence
		}
		candidate := body + alone.String()
		p.Used = tokbudget.Estimate(candidate)
		if p.Budget == 0 || tokbudget.Estimate(candidate+p.footer()) <= p.Budget {
			body = candidate
			continue
		}
		p.Truncated = append(p.Truncated, s.name)
	}
	p.Used = tokbudget.Estimate(body)
	p.Used = tokbudget.Estimate(body + p.footer())
	p.rendered = body
}

// Markdown renders the pack for a reader that pays by the token, footer
// included; Used measures exactly this output.
func (p *Pack) Markdown() string {
	out := p.rendered + p.footer()
	p.Used = tokbudget.Estimate(out)
	return out
}

func (p *Pack) footer() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "---\n\n- %d tokens est. (%d chars/token)", p.Used, tokbudget.CharsPerToken)
	if p.Budget > 0 {
		fmt.Fprintf(&sb, " · budget %d", p.Budget)
	}
	if len(p.Truncated) > 0 {
		fmt.Fprintf(&sb, " · cut: %s", strings.Join(p.Truncated, ", "))
	}
	sb.WriteString("\n")
	return sb.String()
}

func (p *Pack) renderRoutes() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", p.Module)
	for _, r := range p.Routes {
		line := fmt.Sprintf("- `%s %s` — %s", r.Methods, r.Pattern, r.File)
		if len(r.Middlewares) > 0 {
			line += " · middleware: " + strings.Join(r.Middlewares, ", ")
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func (p *Pack) renderContracts(sb *strings.Builder) {
	if len(p.APIPaths) == 0 {
		return
	}
	fmt.Fprintf(sb, "## API paths\n\n")
	for _, a := range p.APIPaths {
		fmt.Fprintf(sb, "- %s\n", a)
	}
	sb.WriteString("\n")
}

func (p *Pack) renderRecipes(sb *strings.Builder) {
	if len(p.Recipes) == 0 {
		return
	}
	fmt.Fprintf(sb, "## Recipes\n\n")
	for _, r := range p.Recipes {
		line := "- `" + r.Name + "`"
		if r.Doc != "" {
			line += " — " + r.Doc
		}
		if len(r.Files) > 0 {
			line += " · " + strings.Join(r.Files, ", ")
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n")
}

func (p *Pack) renderConventions(sb *strings.Builder) {
	if len(p.Conventions) == 0 {
		return
	}
	fmt.Fprintf(sb, "## Conventions\n\n")
	for _, c := range p.Conventions {
		fmt.Fprintf(sb, "- %s\n", c)
	}
	sb.WriteString("\n")
}

// allRoutes converts the full map's routes.
func allRoutes(c *Context) []RouteLine {
	out := make([]RouteLine, 0, len(c.Routes))
	for _, r := range c.Routes {
		out = append(out, RouteLine{
			Methods:     strings.Join(r.Methods, " "),
			Pattern:     r.Pattern,
			File:        r.File,
			Middlewares: r.Middlewares,
		})
	}
	return out
}

// routesTouched keeps the routes whose file is one of the recipe's.
func routesTouched(c *Context, files []string) []RouteLine {
	seen := map[string]bool{}
	for _, f := range files {
		seen[path.Clean(f)] = true
	}
	var out []RouteLine
	for _, r := range allRoutes(c) {
		if seen[path.Clean(r.File)] {
			out = append(out, r)
		}
	}
	return out
}

// apiPaths lists the API patterns of the routes the pack kept: a route.go
// file is an API by convention.
func apiPaths(routes []RouteLine) []string {
	var out []string
	for _, r := range routes {
		if strings.HasSuffix(r.File, "route.go") {
			out = append(out, r.Pattern)
		}
	}
	return out
}

// conventions names what the tree actually uses, which is what stops an
// agent from inventing a second way. whole is false for a recipe slice: a
// slice only claims what its own routes show.
func conventions(c *Context, whole bool) []string {
	var out []string
	var layouts, middlewares, policy, api bool
	for _, r := range c.Routes {
		if len(r.Layouts) > 0 {
			layouts = true
		}
		if len(r.Middlewares) > 0 || len(r.MiddlewaresByMethod) > 0 {
			middlewares = true
		}
		if r.Policy != nil || len(r.PolicyByMethod) > 0 {
			policy = true
		}
		if r.Kind == "api" {
			api = true
		}
	}
	if whole {
		if api {
			out = append(out, "JSON APIs answered by route.go handlers")
		}
		if layouts {
			out = append(out, "nested layouts (layout.go wraps its subtree)")
		}
		if middlewares {
			out = append(out, "folder middleware (middleware.go guards its subtree)")
		}
		if policy {
			out = append(out, "auth policies declared with RequirePolicy")
		}
		if len(c.Enums) > 0 {
			out = append(out, "domain enums listed on this map")
		}
		if c.Setup != nil && len(c.Setup.Values) > 0 {
			out = append(out, "dependencies provided in setup.go (trilha.Use reads them)")
		}
	} else {
		if api {
			out = append(out, "JSON APIs answered by route.go handlers")
		}
		if layouts {
			out = append(out, "nested layouts (layout.go wraps its subtree)")
		}
		if middlewares {
			out = append(out, "folder middleware (middleware.go guards its subtree)")
		}
	}
	return out
}

func orNone(xs []string) string {
	if len(xs) == 0 {
		return "none"
	}
	sort.Strings(xs)
	return strings.Join(xs, ", ")
}
