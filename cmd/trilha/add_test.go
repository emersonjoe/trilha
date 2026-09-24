package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/internal/recipes"
)

// #253, #254 — every name is a recipe and a flag counts wherever it stands:
// `trilha add login users audit --lang pt` used to apply login, drop the
// other two and never read --lang, because the flag package stops at the
// first name and the second parse stopped at the second.
func TestAddArgsTakeSeveralRecipesAndFlagsAnywhere(t *testing.T) {
	names, o, err := parseAddArgs([]string{"login", "users", "audit", "--lang", "pt"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, " ") != "login users audit" || o.lang != "pt" {
		t.Fatalf("names %v lang %q", names, o.lang)
	}
	names, o, err = parseAddArgs([]string{"--dry-run", "login", "--lang=en", "users"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, " ") != "login users" || !o.dry || o.lang != "en" {
		t.Fatalf("names %v opts %+v", names, o)
	}
	if _, _, err := parseAddArgs([]string{"login", "--lang", "fr"}); err == nil {
		t.Fatal("--lang fr passed")
	}
	// A name nobody answers to refuses the whole call before anything is
	// written, and says which one.
	if _, _, err := parseAddArgs([]string{"login", "typo"}); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("err = %v", err)
	}
	// No name is the listing.
	if names, o, err := parseAddArgs(nil); err != nil || len(names) != 0 || o.list {
		t.Fatalf("names %v opts %+v err %v", names, o, err)
	}
}

// Spec 162: the listing prices every recipe — what `ctx --pack` costs once it
// is installed — in both forms, and labels the number an estimate.
func TestAddListShowsCost(t *testing.T) {
	var human strings.Builder
	if err := listRecipes(&human, false, "en"); err != nil {
		t.Fatal(err)
	}
	for _, r := range recipes.All() {
		want := fmt.Sprintf("~%d tok (est.)", r.CtxPackCost)
		line := ""
		for _, l := range strings.Split(human.String(), "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), r.Name+" ") {
				line = l
			}
		}
		if !strings.Contains(line, want) {
			t.Errorf("%s: line %q does not carry %q", r.Name, line, want)
		}
	}
	var machine strings.Builder
	if err := listRecipes(&machine, true, "en"); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name string `json:"name"`
		Cost int    `json:"ctx_pack_tokens"`
	}
	if err := json.Unmarshal([]byte(machine.String()), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(recipes.All()) {
		t.Fatalf("%d rows for %d recipes", len(rows), len(recipes.All()))
	}
	for _, row := range rows {
		if row.Cost <= 0 {
			t.Errorf("%s: ctx_pack_tokens = %d", row.Name, row.Cost)
		}
	}
}
