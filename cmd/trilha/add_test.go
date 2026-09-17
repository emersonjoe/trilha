package main

import (
	"strings"
	"testing"
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
