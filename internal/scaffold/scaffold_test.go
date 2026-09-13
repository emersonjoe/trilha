package scaffold

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The base is what every project has; the rest is the shape that was asked for.
func TestWriteBlogIsTheDefault(t *testing.T) {
	dir := t.TempDir()
	written, err := Write(dir, Data{Module: "example.com/x", Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"go.mod", ".gitignore", "app/page.go", "app/api/hello/route.go"} {
		if !slices.Contains(written, want) {
			t.Fatalf("%s not written: %v", want, written)
		}
	}
	if slices.Contains(written, "app/login/page.go") {
		t.Fatal("the blog got a login it never asked for")
	}
}

func TestWriteApp(t *testing.T) {
	dir := t.TempDir()
	written, err := Write(dir, Data{Module: "example.com/x", Name: "x", Template: "app"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"go.mod", "app/middleware.go", "app/items/id_/page.go", "internal/store/store.go"} {
		if !slices.Contains(written, want) {
			t.Fatalf("%s not written: %v", want, written)
		}
	}
	// Two shapes, one base: the app does not carry the blog's pages.
	if slices.Contains(written, "app/api/hello/route.go") {
		t.Fatal("the app got the blog's routes")
	}
	// The module of the project, not of the framework, is what the imports say.
	b, err := os.ReadFile(filepath.Join(dir, "app", "middleware.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"example.com/x/internal/sessao"`) {
		t.Fatalf("import not filled in:\n%s", b)
	}
}

// #212: app/layout.go and app/middleware.go of the app template import
// internal/sessao, which only the login recipe writes — so `trilha new` has
// to know it depends on that recipe even when --with does not name it.
func TestNeedsSaysWhatATemplateDependsOn(t *testing.T) {
	if got := Needs("app"); !slices.Equal(got, []string{"login"}) {
		t.Fatalf("app: got %v, want [login]", got)
	}
	if got := Needs("blog"); got != nil {
		t.Fatalf("blog: got %v, want nil", got)
	}
}

// #212's fallout: app/admin/middleware.go used to ship unconditionally, and a
// --with that leaves app/admin without a single recipe under it left an
// import trilha_gen.go never used — a project failing go vet for a reason
// nobody asked about. Admin says whether a recipe is actually going there.
func TestWriteAppAdminMiddlewareOnlyWhenSomethingUsesIt(t *testing.T) {
	sem, err := Write(t.TempDir(), Data{Module: "example.com/x", Name: "x", Template: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(sem, "app/admin/middleware.go") {
		t.Fatal("wrote app/admin/middleware.go with nothing under app/admin to guard")
	}
	com, err := Write(t.TempDir(), Data{Module: "example.com/x", Name: "x", Template: "app", Admin: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(com, "app/admin/middleware.go") {
		t.Fatal("Admin: true did not write app/admin/middleware.go")
	}
}

func TestWriteRefusesAnUnknownTemplate(t *testing.T) {
	if _, err := Write(t.TempDir(), Data{Module: "x", Name: "x", Template: "banana"}); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := Write(t.TempDir(), Data{Module: "x", Name: "x", Lang: "fr"}); err == nil {
		t.Fatal("expected an error")
	}
}

// Every {{.T.key}} the templates read has to exist in both languages, or the
// generated project comes out with an empty label.
func TestEveryLanguageSaysTheSameThings(t *testing.T) {
	en, pt := texts["en"], texts["pt"]
	for k := range en {
		if _, ok := pt[k]; !ok {
			t.Errorf("pt is missing %q", k)
		}
	}
	for k := range pt {
		if _, ok := en[k]; !ok {
			t.Errorf("en is missing %q", k)
		}
	}
	for _, tmpl := range Templates() {
		for _, lang := range []string{"en", "pt"} {
			if _, err := Write(t.TempDir(), Data{Module: "example.com/x", Name: "x", Lang: lang, Template: tmpl}); err != nil {
				t.Fatalf("%s/%s: %v", tmpl, lang, err)
			}
		}
	}
}
