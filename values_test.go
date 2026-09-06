package trilha

import (
	"strings"
	"testing"

	"github.com/emersonjoe/trilha/h"
)

type deps struct{ dsn string }

type store interface{ Name() string }

type memStore struct{ name string }

func (m memStore) Name() string { return m.name }

// Spec 046 (#55): the dependencies of an app arrive with a type. The key is
// the type itself, so there is no string to mistype and no nil travelling to
// the first use of the pool.
func TestProvideAndUse(t *testing.T) {
	a := New(Config{Logger: quiet()})
	Provide(a, &deps{dsn: "postgres://farol"})
	Provide(a, deps{dsn: "by value"})
	Provide[store](a, memStore{name: "mem"})

	var got []string
	a.Register(Route{Pattern: "/", Page: func(c *Ctx) (h.Node, error) {
		got = append(got, Use[*deps](c).dsn, Use[deps](c).dsn, Use[store](c).Name())
		return h.Text("ok"), nil
	}})
	if rec := get(t, a, "GET", "/", "", nil); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if want := []string{"postgres://farol", "by value", "mem"}; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
	// The bag stays the bag: Values keeps working for everything else.
	if len(a.Values()) != 3 {
		t.Fatalf("Values: %v", a.Values())
	}
}

// What is missing has to fail where the cause is, with the type in the text.
func TestUseWithoutProvidePanics(t *testing.T) {
	a := New(Config{Logger: quiet()})
	defer func() {
		r := recover()
		msg, _ := r.(string)
		if !strings.Contains(msg, "*trilha.deps") || !strings.Contains(msg, "Provide") {
			t.Fatalf("panic should name the type and the fix: %v", r)
		}
	}()
	c := &Ctx{app: a}
	_ = Use[*deps](c)
}

// Two apps in one process — the shape of a suite that boots a server per test
// — do not see each other's dependencies. This is what package state cannot do.
func TestProvideIsPerApp(t *testing.T) {
	one, two := New(Config{Logger: quiet()}), New(Config{Logger: quiet()})
	Provide(one, &deps{dsn: "one"})
	Provide(two, &deps{dsn: "two"})
	if Use[*deps](&Ctx{app: one}).dsn != "one" || Use[*deps](&Ctx{app: two}).dsn != "two" {
		t.Fatal("apps share dependencies")
	}
	// The App is a Bag too: Setup and a test reach the same values without a
	// request in hand.
	if Use[*deps](one).dsn != "one" {
		t.Fatal("the app itself should answer Use")
	}
}
