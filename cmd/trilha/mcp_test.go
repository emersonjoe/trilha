package main

import (
	"strings"
	"testing"
)

// The refusals are the security of this server, so they are tested without a
// server, a project or a process: generateArgs is where a value from the model
// either becomes an argument or stops.
func TestMCPGenerateRefusesWhatItShouldNeverRun(t *testing.T) {
	for _, c := range []struct {
		why     string
		kind    string
		target  string
		methods []string
		bind    string
		form    string
	}{
		{"climbing out of the project", "page", "/../../etc/passwd", nil, "", ""},
		{"climbing, disguised", "route", "/api/../../..", nil, "", ""},
		{"a relative path is not an address", "page", "app/page.go", nil, "", ""},
		{"a shell would find this interesting", "page", "/x; rm -rf /", nil, "", ""},
		{"and this", "page", "/x$(id)", nil, "", ""},
		{"and this too", "page", "/x`id`", nil, "", ""},
		{"a flag is not a path", "page", "--force", nil, "", ""},
		{"a kind that does not exist", "wipe", "/x", nil, "", ""},
		{"a method that does not exist", "route", "/api/x", []string{"DROP"}, "", ""},
		{"a method smuggling a flag", "route", "/api/x", []string{"GET,--force"}, "", ""},
		{"a type that is a command", "route", "/api/x", nil, "Type;id", ""},
		{"a type that is a path", "route", "/api/x", nil, "", "../../secret"},
		{"a component name with a separator", "component", "a/b", nil, "", ""},
	} {
		if args, err := generateArgs(c.kind, c.target, c.methods, c.bind, c.form); err == nil {
			t.Errorf("%s: accepted %q and built %v", c.why, c.target, args)
		}
	}
}

// What it does accept becomes an argument slice — never a string a shell would
// have to take apart.
func TestMCPGenerateBuildsArguments(t *testing.T) {
	args, err := generateArgs("route", "/api/posts/{id}", []string{"GET", "post"}, "Post", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"generate", "route", "/api/posts/{id}", "--methods", "GET,POST", "--bind", "Post"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %v, want %v", args, want)
	}
	if _, err := generateArgs("component", "Card", nil, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := generateArgs("page", "/blog/{slug}", nil, "", ""); err != nil {
		t.Fatal(err)
	}
}

// Least privilege, and it is structural: without --write the writing tool is
// not registered, so it is not in tools/list and a model cannot ask for it.
func TestMCPWritingToolIsNotOfferedByDefault(t *testing.T) {
	r := &mcpRunner{root: t.TempDir(), self: "trilha"}
	read := []string{r.describeProject().Name, r.check().Name, r.routes().Name, r.uiDescribe().Name}
	for _, name := range read {
		if name == "generate" {
			t.Fatalf("a read-only tool is called %q", name)
		}
	}
	if r.generate().Name != "generate" {
		t.Fatal("the writing tool changed name; the --write gate is by name")
	}
	// The tool a model sees describes what it does to the disk, in the first
	// sentence, because that is what a person approving a call reads.
	if !strings.Contains(r.generate().Description, "Writes files") {
		t.Fatal("the writing tool does not say that it writes")
	}
}
