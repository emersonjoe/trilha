package docsmcp

// The two context tools (spec 160): a pack of documentation — the pages one
// recipe touches, priced in estimated tokens — and code as path:line windows
// into the cookbook's Go sources. Both answer slices, never whole files,
// because the reader pays by the token.

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/ai"
	"github.com/emersonjoe/trilha/internal/checkerr"
	"github.com/emersonjoe/trilha/internal/tokbudget"
	"github.com/emersonjoe/trilha/site/internal/cookbooksrc"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

var linkRe = regexp.MustCompile(`\]\((/[a-z0-9/-]+)\)`)

// contextTools returns get_context, search_code and get_error for the docs
// server: the slices an agent reads before writing, and the catalog entry
// for a code it just saw in a gate's output.
func contextTools() []*ai.Tool {
	tools := []*ai.Tool{
		ai.NewTool("get_context",
			"Return the slice of documentation one recipe touches: the recipe's page and the pages it links to, each priced in estimated tokens. The pack is a cookbook slug (\"database\", \"uploads\", \"login\").",
			ai.Schema(`{"type":"object","properties":{"pack":{"type":"string","minLength":1},"locale":{"type":"string","enum":["en","pt"]}},"required":["pack"]}`),
			ai.Typed(packOfDocs),
		),
		ai.NewTool("search_code",
			"Search the cookbook's Go sources. Answers path:line windows - the matching line plus one after it, a handful of matches at most - never a whole file.",
			ai.Schema(`{"type":"object","properties":{"query":{"type":"string","minLength":1},"limit":{"type":"integer","minimum":1,"maximum":20}},"required":["query"]}`),
			ai.Typed(searchSources),
		),
	}
	return append(tools, getErrorTool())
}

// getErrorTool answers one catalog entry - the cause, the fix, the example -
// for the code a gate printed. Unknown codes say what a code looks like.
func getErrorTool() *ai.Tool {
	return ai.NewTool("get_error",
		"Return the catalog entry for a stable error code (E_DUPLICATE_ROUTE, E_VET_PRINTF...): the cause, the fix and an example. A family code such as E_VULN_GO-2026-0001 answers the family's entry.",
		ai.Schema(`{"type":"object","properties":{"code":{"type":"string","minLength":1}},"required":["code"]}`),
		ai.Typed(func(_ context.Context, in struct {
			Code string `json:"code"`
		}) (string, error) {
			code := strings.TrimSpace(in.Code)
			if code == "" {
				return "", fmt.Errorf("code is required")
			}
			d, ok := checkerr.ByCode(code)
			if !ok {
				return "", fmt.Errorf("no entry for %q; codes are E_ in caps - try the family, like E_VULN_", code)
			}
			var sb strings.Builder
			fmt.Fprintf(&sb, "# %s — %s\n\n%s\n\n## Fix\n\n%s\n", d.Code, d.Title, d.Cause, d.Fix)
			if d.Example != "" {
				fmt.Fprintf(&sb, "\n## Example\n\n%s\n", d.Example)
			}
			fmt.Fprintf(&sb, "\n%s\n", d.Doc)
			return sb.String(), nil
		}))
}

// packOfDocs answers the pack: the page the slug names, the pages that page
// links to, and what reading all of it costs. A pack that names nothing is
// an error that says what a pack is.
func packOfDocs(_ context.Context, in struct {
	Pack   string `json:"pack"`
	Locale string `json:"locale"`
}) (string, error) {
	slug := strings.Trim(strings.TrimSpace(in.Pack), "/")
	if slug == "" {
		return "", fmt.Errorf("pack is required: a cookbook slug (\"database\", \"uploads\")")
	}
	locale := normalizeLocale(in.Locale)
	section := "cookbook"
	if locale == "pt" {
		section = "receitas"
	}
	first, ok := docs.Get(locale, section, slug)
	if !ok || first.Slug == "" {
		return "", fmt.Errorf("no cookbook page for %q; a pack is a cookbook slug", slug)
	}
	byPath := map[string]docs.Page{}
	for _, p := range docs.All() {
		byPath[p.Path()] = p
	}
	picked := []docs.Page{first}
	seen := map[string]bool{first.Path(): true}
	for _, m := range linkRe.FindAllStringSubmatch(first.Body, -1) {
		if len(picked) >= 12 {
			break
		}
		p, ok := byPath[m[1]]
		if !ok || seen[p.Path()] {
			continue
		}
		seen[p.Path()] = true
		picked = append(picked, p)
	}
	var sb strings.Builder
	total := 0
	for _, p := range picked {
		cost := tokbudget.Estimate(p.Body)
		total += cost
		fmt.Fprintf(&sb, "- %s — %s · %d tokens est.\n", p.Path(), p.Title, cost)
	}
	fmt.Fprintf(&sb, "%d page(s), %d tokens est. in all\n", len(picked), total)
	return sb.String(), nil
}

// searchSources answers path:line windows into the embedded cookbook
// sources, at most limit matches, one line of context after each.
func searchSources(_ context.Context, in struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}) (string, error) {
	query := strings.ToLower(strings.TrimSpace(in.Query))
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
	names, err := cookbooksrc.Names()
	if err != nil {
		return "", err
	}
	sort.Strings(names)
	var sb strings.Builder
	matches := 0
	for _, name := range names {
		if matches >= limit {
			break
		}
		b, err := cookbooksrc.Read(name)
		if err != nil {
			continue
		}
		rel := strings.TrimPrefix(name, "sources/")
		lines := strings.Split(string(b), "\n")
		for i, l := range lines {
			if matches >= limit {
				break
			}
			if strings.Contains(strings.ToLower(l), query) {
				fmt.Fprintf(&sb, "%s:%d: %s\n", rel, i+1, strings.TrimSpace(l))
				if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
					fmt.Fprintf(&sb, "%s:%d: %s\n", rel, i+2, strings.TrimSpace(lines[i+1]))
				}
				matches++
			}
		}
	}
	if matches == 0 {
		return "no match for " + in.Query, nil
	}
	fmt.Fprintf(&sb, "%d match(es), %d tokens est.\n", matches, tokbudget.Estimate(sb.String()))
	return sb.String(), nil
}
