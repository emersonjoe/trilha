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
