package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/ctx"
)

// TestInstalledRecipes reads the record the recipes themselves write: the
// markers in setup.go. The recipe's files are resolved where they actually
// landed — a marker for a recipe whose files are gone is not installed.
func TestInstalledRecipes(t *testing.T) {
	p := appProject(t, "recipes", "example.com/recipes")
	got := installedRecipes(p)
	if len(got) != 1 || got[0].Name != "login" {
		t.Fatalf("installed = %+v, want only login", got)
	}
	if got[0].Doc != "/reference/auth" {
		t.Fatalf("doc = %q, want the recipe's own", got[0].Doc)
	}
	found := strings.Join(got[0].Files, ",")
	if !strings.Contains(found, "app/entrar/page.go") {
		t.Fatalf("files = %q, want the screen the recipe brought", found)
	}

	// A recipe slice of the map: only the routes the recipe answers.
	c, err := ctx.Build(p.Root, p.Module, version)
	if err != nil {
		t.Fatal(err)
	}
	pk, err := c.PackOf("login", got, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pk.Routes) != 1 || pk.Routes[0].Pattern != "/entrar" {
		t.Fatalf("slice routes = %+v, want /entrar", pk.Routes)
	}
	if len(pk.Truncated) != 0 {
		t.Fatalf("no budget, nothing cut: %v", pk.Truncated)
	}

	// Strict is the caller's decision, but the cut is the pack's: a tight
	// budget cuts the recipes before the routes — and an empty section is
	// absence, not a cut.
	tight, err := c.PackOf("app", got, 35)
	if err != nil {
		t.Fatal(err)
	}
	if len(tight.Truncated) == 0 {
		t.Fatal("a 35-token budget must cut something on this map")
	}
	if strings.Join(tight.Truncated, ",") != "recipes" {
		t.Fatalf("truncated = %v", tight.Truncated)
	}
	if len(tight.Routes) != 2 {
		t.Fatalf("routes were cut: %+v", tight.Routes)
	}
}

// TestBudgetCutMessage is the sentence an agent reads when --strict trips:
// what was cut, and the two ways out.
func TestBudgetCutMessage(tt *testing.T) {
	msg := fmt.Sprintf(t("ctx budget cut"), "contracts, recipes")
	if !strings.Contains(msg, "contracts") || !strings.Contains(msg, "--budget") {
		tt.Fatalf("message = %q, want the cut list and the way out", msg)
	}
}
