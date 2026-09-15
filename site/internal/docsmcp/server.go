package docsmcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/ai/mcp"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

// New returns the read-only MCP server backed by the same Markdown pages as
// the documentation site.
func New(version string) *mcp.Server {
	return mcp.NewServer("trilha-docs", version,
		ai.NewTool("search_docs", "Search Trilha documentation pages by title, description and body.", ai.Schema(`{"type":"object","properties":{"query":{"type":"string","minLength":1},"locale":{"type":"string","enum":["en","pt"]},"limit":{"type":"integer","minimum":1,"maximum":20}},"required":["query"]}`), ai.Typed(search)),
		ai.NewTool("get_page", "Return one Trilha documentation page as Markdown by its site path.", ai.Schema(`{"type":"object","properties":{"path":{"type":"string","minLength":1}},"required":["path"]}`), ai.Typed(page)),
		ai.NewTool("get_recipe", "Return one Trilha cookbook recipe as Markdown by slug.", ai.Schema(`{"type":"object","properties":{"name":{"type":"string","minLength":1},"locale":{"type":"string","enum":["en","pt"]}},"required":["name"]}`), ai.Typed(recipe)),
	)
}

type searchInput struct {
	Query  string `json:"query"`
	Locale string `json:"locale"`
	Limit  int    `json:"limit"`
}

type searchResult struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Snippet     string `json:"snippet"`
}

func search(_ context.Context, in searchInput) (string, error) {
	query := strings.ToLower(strings.TrimSpace(in.Query))
	if query == "" {
		return "", errors.New("query is required")
	}
	locale := normalizeLocale(in.Locale)
	limit := in.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	var out []searchResult
	for _, p := range docs.Pages(locale) {
		haystack := strings.ToLower(p.Title + "\n" + p.Description + "\n" + p.Body)
		at := strings.Index(haystack, query)
		if at < 0 {
			continue
		}
		out = append(out, searchResult{Path: p.Path(), Title: p.Title, Description: p.Description, Snippet: snippet(p.Body, query)})
		if len(out) == limit {
			break
		}
	}
	b, err := json.MarshalIndent(out, "", "  ")
	return string(b), err
}

type pageInput struct {
	Path string `json:"path"`
}

func page(_ context.Context, in pageInput) (string, error) {
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return "", errors.New("path is required")
	}
	for _, p := range docs.All() {
		if p.Path() == path || strings.TrimPrefix(p.Path(), "/") == strings.TrimPrefix(path, "/") {
			return markdown(p), nil
		}
	}
	return "", fmt.Errorf("documentation page not found: %s", path)
}

type recipeInput struct {
	Name   string `json:"name"`
	Locale string `json:"locale"`
}

func recipe(_ context.Context, in recipeInput) (string, error) {
	name := strings.Trim(strings.TrimSpace(in.Name), "/")
	if name == "" {
		return "", errors.New("name is required")
	}
	locale := normalizeLocale(in.Locale)
	section := "cookbook"
	if locale == "pt" {
		section = "receitas"
	}
	if p, ok := docs.Get(locale, section, name); ok && p.Slug != "" {
		return markdown(p), nil
	}
	return "", fmt.Errorf("recipe not found: %s", name)
}

func normalizeLocale(locale string) string {
	if locale == "pt" {
		return "pt"
	}
	return "en"
}

func markdown(p docs.Page) string {
	return "# " + p.Title + "\n\n" + p.Description + "\n\n" + strings.TrimSpace(p.Body) + "\n"
}

func snippet(body, query string) string {
	plain := strings.Join(strings.Fields(body), " ")
	runes := []rune(plain)
	needle := []rune(strings.ToLower(query))
	at := -1
	for i := 0; i+len(needle) <= len(runes); i++ {
		matched := true
		for j := range needle {
			if unicode.ToLower(runes[i+j]) != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			at = i
			break
		}
	}
	if at < 0 {
		at = 0
	}
	start := at - 80
	if start < 0 {
		start = 0
	}
	end := at + len(needle) + 160
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out += "…"
	}
	return out
}
