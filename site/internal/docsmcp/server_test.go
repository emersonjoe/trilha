package docsmcp

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/ai/mcp"
	"github.com/emersonjoe/trilha/site/internal/docs"
)

func TestDocumentationServerOverHTTP(t *testing.T) {
	host := httptest.NewServer(New("test").Handler())
	defer host.Close()
	client, err := mcp.Dial(context.Background(), mcp.HTTP(host.URL, nil))
	if err != nil {
		t.Fatal(err)
	}
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{tools[0].Name, tools[1].Name, tools[2].Name}; strings.Join(got, ",") != "search_docs,get_page,get_recipe" {
		t.Fatalf("tools = %v", got)
	}
}

func TestGetRecipeReturnsTheCanonicalMarkdown(t *testing.T) {
	got, err := recipe(context.Background(), recipeInput{Name: "pagination", Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	page, ok := docs.Get("en", "cookbook", "pagination")
	if !ok {
		t.Fatal("pagination recipe is not registered")
	}
	want := markdown(page)
	if got != want {
		t.Fatal("get_recipe drifted from the embedded cookbook Markdown")
	}
	for _, block := range []string{"func ArticlesPage", "func ArticlesAfter", "func Pages"} {
		if !strings.Contains(got, block) {
			t.Errorf("recipe misses canonical example block %q", block)
		}
	}
}

func TestSearchAndGetPage(t *testing.T) {
	got, err := search(context.Background(), searchInput{Query: "streamable HTTP", Locale: "en", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	var results []searchResult
	if err := json.Unmarshal([]byte(got), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 || results[0].Path == "" {
		t.Fatalf("results = %s", got)
	}
	body, err := page(context.Background(), pageInput{Path: "/reference/mcp"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "# mcp") || !strings.Contains(body, "Streamable HTTP") {
		t.Fatalf("page = %q", body)
	}
}

// mcpHost spins the docs server and hands back a client to it.
func mcpHost(t *testing.T) *mcp.Client {
	t.Helper()
	host := httptest.NewServer(New("test").Handler())
	t.Cleanup(host.Close)
	client, err := mcp.Dial(context.Background(), mcp.HTTP(host.URL, nil))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// TestGetContextPricesAPack asks for a recipe's pack and checks the shape:
// the recipe's own page first, the pages it links to beside it, every line
// priced, and the total at the foot.
func TestGetContextPricesAPack(t *testing.T) {
	client := mcpHost(t)
	ctx := context.Background()
	raw, err := client.CallTool(ctx, "get_context", json.RawMessage(`{"pack":"database","locale":"en"}`))
	if err != nil {
		t.Fatal(err)
	}
	out := raw.Text()
	if !strings.Contains(out, "/cookbook/database") {
		t.Fatalf("the pack lacks the recipe's own page:\n%s", out)
	}
	if !strings.Contains(out, "tokens est. in all") {
		t.Fatalf("the pack has no total:\n%s", out)
	}
	if !strings.Contains(out, "· ") {
		t.Fatalf("a page without its price:\n%s", out)
	}
	// A pack that names nothing says what a pack is — as the tool's answer,
	// which is how the protocol carries a tool's own error.
	bad, err := client.CallTool(ctx, "get_context", json.RawMessage(`{"pack":"faturamento"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !bad.IsError {
		t.Fatalf("an unknown pack must be an error answer:\n%s", bad.Text())
	}
}

// TestSearchCodeWindows is the window contract: path:line, one line after,
// a bounded number of matches, never a whole file.
func TestSearchCodeWindows(t *testing.T) {
	client := mcpHost(t)
	ctx := context.Background()
	raw, err := client.CallTool(ctx, "search_code", json.RawMessage(`{"query":"DataTable"}`))
	if err != nil {
		t.Fatal(err)
	}
	out := raw.Text()
	if !strings.Contains(out, ".go:") {
		t.Fatalf("no path:line window in the answer:\n%s", out)
	}
	if !strings.Contains(out, "tokens est.") {
		t.Fatalf("the answer prices itself:\n%s", out)
	}
	if strings.Count(out, "\n") > 25 {
		t.Fatalf("the windows are too wide:\n%s", out)
	}
}
