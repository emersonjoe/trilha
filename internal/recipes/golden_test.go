package recipes

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files of the recipes")

// instalarGolden installs the recipe — and what it needs, first — into a
// fresh project and answers the root and what the recipe itself wrote.
func instalarGolden(t *testing.T, r Recipe) (string, []string) {
	t.Helper()
	raiz := projeto(t)
	var precisa func(r Recipe)
	precisa = func(r Recipe) {
		for _, n := range r.Needs {
			if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash(n.File))); err == nil {
				continue
			}
			dep := mustGet(t, n.Recipe)
			precisa(dep)
			if _, err := Add(raiz, dep, Options{Module: "example.com/golden", Lang: "en"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	precisa(r)
	res, err := Add(raiz, r, Options{Module: "example.com/golden", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	return raiz, res.Written
}

// conferirGolden is the install test of a platform recipe (spec 162): the
// files it writes are the ones in testdata/<name>/, byte for byte, and two
// installs write the same bytes. A change to the recipe shows up as a diff
// of generated code in review — which is what the person who runs
// `trilha add` receives — and `make golden` rewrites it on purpose.
func conferirGolden(t *testing.T, nome string) {
	t.Helper()
	r := mustGet(t, nome)
	raiz, escritos := instalarGolden(t, r)
	outra, _ := instalarGolden(t, r)
	dir := filepath.Join("testdata", nome)
	if *update {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
	}
	quer := map[string]bool{}
	for _, rel := range escritos {
		quer[rel+".golden"] = true
		got, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		deNovo, err := os.ReadFile(filepath.Join(outra, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, deNovo) {
			t.Errorf("%s: two installs wrote different bytes", rel)
		}
		golden := filepath.Join(dir, filepath.FromSlash(rel)+".golden")
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%s: no golden (run make golden): %v", rel, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from %s (run make golden if the change is on purpose)", rel, golden)
		}
	}
	// A golden nobody writes any more is a file the recipe stopped shipping,
	// and the review should see it go.
	var sobra []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if !quer[filepath.ToSlash(rel)] {
			sobra = append(sobra, rel)
		}
		return nil
	})
	sort.Strings(sobra)
	if len(sobra) > 0 {
		t.Errorf("goldens the recipe no longer writes: %s", strings.Join(sobra, ", "))
	}
}

func TestBillingInstall(t *testing.T) {
	conferirGolden(t, "billing")
	// And what it installs is what the plan names: the four tables, the
	// signed endpoint at a fixed address, the screens behind the policy.
	raiz, escritos := instalarGolden(t, mustGet(t, "billing"))
	lista := strings.Join(escritos, " ")
	for _, quero := range []string{"migrations/0100_billing.sql", "app/webhooks/billing/route.go",
		"app/billing/middleware.go", "app/billing/faturas/csv/middleware.go", "billing_test.go"} {
		if !strings.Contains(lista, quero) {
			t.Errorf("did not write %s: %s", quero, lista)
		}
	}
	sql := ler(t, raiz, "migrations/0100_billing.sql")
	for _, tabela := range []string{"billing_plans", "billing_subscriptions", "billing_invoices", "billing_events"} {
		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS "+tabela) {
			t.Errorf("the migration has no %s", tabela)
		}
	}
}

func TestNotifyInstall(t *testing.T) {
	conferirGolden(t, "notify")
	// Alone, it wires e-mail and nothing else: the links to the webhook and
	// WhatsApp channels only land where those recipes are.
	raiz, _ := instalarGolden(t, mustGet(t, "notify"))
	setup := ler(t, raiz, "app/setup.go")
	if !strings.Contains(setup, "notificar.Setup(a)") {
		t.Fatalf("notify did not wire itself:\n%s", setup)
	}
	for _, link := range []string{"trilha:link notify-webhooks", "trilha:link notify-whatsapp"} {
		if strings.Contains(setup, link) {
			t.Errorf("%s landed without the other recipe:\n%s", link, setup)
		}
	}
}

// The two channels that depend on other recipes are tied in either order,
// once, and above the Setup that reads the closed list of events.
func TestNotifyLinksInAnyOrder(t *testing.T) {
	for _, ordem := range [][]string{
		{"login", "connections", "notify", "webhooks", "channel-whatsapp"},
		{"login", "connections", "webhooks", "channel-whatsapp", "notify"},
	} {
		raiz := projeto(t)
		for _, nome := range ordem {
			if _, err := Add(raiz, mustGet(t, nome), Options{Module: "example.com/x", Lang: "en"}); err != nil {
				t.Fatal(nome, err)
			}
		}
		setup := ler(t, raiz, "app/setup.go")
		for _, link := range []string{"trilha:link notify-webhooks", "trilha:link notify-whatsapp"} {
			if n := strings.Count(setup, link); n != 1 {
				t.Errorf("%v: %s appears %d times", ordem, link, n)
			}
		}
		if strings.Index(setup, "avisos.Eventos = append") > strings.Index(setup, "avisos.Setup(a)") {
			t.Errorf("%v: the event joins the list after avisos.Setup read it:\n%s", ordem, setup)
		}
	}
}
