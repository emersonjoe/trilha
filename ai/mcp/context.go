package mcp

// The two tools a project's MCP server offers the agent that pays by the
// token (spec 160): the map, sliced and budgeted, and code as path:line
// windows — never a whole file. They read the project the way `trilha ctx`
// reads it, so an agent with no shell gets the same one-read answer the CLI
// gives.

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/internal/ctx"
	"github.com/emersonjoe/trilha/internal/tokbudget"
)

// ContextOpts is where the tools read the project. Module defaults to the
// directory's name; Version is what the map stamps.
type ContextOpts struct {
	Root    string
	Module  string
	Version string
}

// ContextTools returns get_context and search_code over the project at
// opts.Root, and get_pattern — the kit's screen patterns, which need no
// project. Attach them with the server's variadic tools:
//
//	mcp.NewServer(name, version, append(mcp.FromRoutes(a, o).Tools(),
//		mcp.ContextTools(opts)...)...)
func ContextTools(opts ContextOpts) []*ai.Tool {
	return []*ai.Tool{
		ai.NewTool("get_context",
			"Return the project map - routes, installed recipes, conventions and API paths - sliced for one job (pack: \"app\", or an installed recipe's name), priced in estimated tokens. Prefer this to opening files.",
			ai.Schema(`{"type":"object","properties":{"pack":{"type":"string","default":"app"},"budget":{"type":"integer","minimum":0,"description":"token budget; sections cut are named in the answer"}}}`),
			ai.Typed(func(_ context.Context, in struct {
				Pack   string `json:"pack"`
				Budget int    `json:"budget"`
			}) (string, error) {
				name := in.Pack
				if name == "" {
					name = "app"
				}
				root, module := opts.Root, opts.Module
				if module == "" {
					module = moduleOf(root)
				}
				installed := ctx.InstalledRecipes(root)
				c, err := ctx.Build(root, module, opts.Version)
				if err != nil {
					return "", err
				}
				p, err := c.PackOf(name, installed, in.Budget)
				if err != nil {
					return "", err
				}
				return p.Markdown(), nil
			}),
		),
		ai.NewTool("search_code",
			"Search the project's source for a text. Answers path:line windows of the matching line plus one after it - at most a handful of matches - never a whole file.",
			ai.Schema(`{"type":"object","properties":{"query":{"type":"string","minLength":1},"limit":{"type":"integer","minimum":1,"maximum":20}},"required":["query"]}`),
			ai.Typed(func(_ context.Context, in struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}) (string, error) {
				query := strings.TrimSpace(in.Query)
				if query == "" {
					return "", fmt.Errorf("query is required")
				}
				limit := in.Limit
				if limit <= 0 {
					limit = 8
				}
				if limit > 20 {
					limit = 20
				}
				return searchCode(opts.Root, query, limit)
			}),
		),
		patternTool(),
	}
}

// moduleOf reads the module path out of go.mod; a project without one falls
// back to its directory's name.
func moduleOf(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module "))
			}
		}
	}
	return filepath.Base(root)
}

// searchCode walks the project and answers the matches as windows. Skips are
// the same ones a reader would make: caches, binaries, dependencies.
func searchCode(root, query string, limit int) (string, error) {
	needle := strings.ToLower(query)
	type hit struct {
		path  string
		line  int
		text  string
		after string
	}
	var hits []hit
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".trilha", "bin", "node_modules", "vendor", "testdata", "tmp":
				return fs.SkipDir
			}
			return nil
		}
		if len(hits) >= limit {
			return fs.SkipAll
		}
		ext := filepath.Ext(path)
		switch ext {
		case ".go", ".md", ".json", ".css", ".js", ".html", ".yaml", ".yml", ".sql":
		default:
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 200*1024 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(b), "\n")
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		for i, l := range lines {
			if strings.Contains(strings.ToLower(l), needle) {
				after := ""
				if i+1 < len(lines) {
					after = strings.TrimSpace(lines[i+1])
				}
				hits = append(hits, hit{rel, i + 1, strings.TrimSpace(l), after})
				if len(hits) >= limit {
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "no match for " + query, nil
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].path != hits[j].path {
			return hits[i].path < hits[j].path
		}
		return hits[i].line < hits[j].line
	})
	var sb strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&sb, "%s:%d: %s\n", h.path, h.line, h.text)
		if h.after != "" {
			fmt.Fprintf(&sb, "%s:%d: %s\n", h.path, h.line+1, h.after)
		}
	}
	fmt.Fprintf(&sb, "%d match(es), %d tokens est.\n", len(hits), tokbudget.Estimate(sb.String()))
	return sb.String(), nil
}
