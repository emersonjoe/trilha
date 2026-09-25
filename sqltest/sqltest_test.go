// Package sqltest proves the SQL stores of the billing and notify recipes on a
// real database. It is a module of its own, like otel/ and uitest/: the SQLite
// driver it needs never reaches the framework's go.mod.
//
// It builds the CLI from this repository, makes projects with `trilha new`,
// adds the store recipe and the two recipes that use it — in both orders,
// since the tie between them is a line each carries — gives the project the
// driver, and runs the tests the recipes wrote, each against a database file
// of its own. Then it opens the files and looks at the tables: a test that
// passed on memory would pass here too, and the rows are the proof it did not.
package sqltest

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

// A database the generated project is tested on: the driver it gets, and a
// fresh database per test, with a way to count what landed in it.
type banco struct {
	nome   string
	driver string // the import of internal/store/driver.go
	get    string // what go get fetches
	// novo gives the DATABASE_URL of a fresh database for one test.
	novo func(t *testing.T, teste string) string
	// linhas counts the rows of table in the database of that URL.
	linhas func(t *testing.T, url, table string) int
}

// sqlite is a file per test, read back with the same driver this module pins.
var sqlite = banco{
	nome:   "SQLite",
	driver: "modernc.org/sqlite",
	get:    "modernc.org/sqlite@v1.34.5",
	novo: func(t *testing.T, teste string) string {
		return "file:" + filepath.Join(t.TempDir(), teste+".db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	},
	linhas: func(t *testing.T, url, table string) int {
		db, err := sql.Open("sqlite", strings.SplitN(url, "?", 2)[0])
		must(t, err)
		defer db.Close()
		var n int
		must(t, db.QueryRow("SELECT count(*) FROM "+table).Scan(&n))
		return n
	},
}

var build struct {
	once sync.Once
	tmp  string
	cli  string
	err  error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if build.tmp != "" {
		os.RemoveAll(build.tmp)
	}
	os.Exit(code)
}

func cli(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	build.once.Do(func() {
		repo, err := filepath.Abs("..")
		if err != nil {
			build.err = err
			return
		}
		if build.tmp, err = os.MkdirTemp("", "sqltest-"); err != nil {
			build.err = err
			return
		}
		build.cli = filepath.Join(build.tmp, "trilha")
		build.err = sh(repo, nil, "go", "build", "-o", build.cli, "./cmd/trilha")
	})
	if build.err != nil {
		t.Fatal(build.err)
	}
	return build.cli
}

// project makes a project with the recipes added in the order given, and the
// driver of b.
func project(t *testing.T, b banco, name string, recipes ...string) string {
	t.Helper()
	bin := cli(t)
	repo, _ := filepath.Abs("..")
	proj := filepath.Join(build.tmp, name)
	must(t, sh(build.tmp, nil, bin, "new", proj, "--module", "example.com/"+name, "--trilha-dir", repo))
	must(t, sh(proj, nil, append([]string{bin, "add"}, recipes...)...))
	must(t, os.WriteFile(filepath.Join(proj, "internal", "store", "driver.go"),
		[]byte("package store\n\nimport _ \""+b.driver+"\" // the driver of this project\n"), 0o644))
	must(t, sh(proj, nil, "go", "get", b.get))
	must(t, sh(proj, nil, "go", "mod", "tidy"))
	must(t, sh(proj, nil, "go", "build", "./..."))
	return proj
}

// runEach runs every test of pkg in a process of its own, each with a fresh
// database, and returns the DATABASE_URL of each by test name.
func runEach(t *testing.T, b banco, proj, pkg string) map[string]string {
	t.Helper()
	out, err := output(proj, nil, "go", "test", pkg, "-list", ".")
	must(t, err)
	dbs := map[string]string{}
	for _, name := range strings.Fields(out) {
		if !strings.HasPrefix(name, "Test") {
			continue
		}
		url := b.novo(t, name)
		if out, err := output(proj, []string{"DATABASE_URL=" + url}, "go", "test", pkg, "-run", "^"+name+"$", "-count=1"); err != nil {
			t.Errorf("%s %s on %s:\n%s", pkg, name, b.nome, out)
			continue
		}
		dbs[name] = url
	}
	return dbs
}

// The store first, the recipes after: the link lines go in when billing and
// notify arrive. Every test the recipes wrote passes on the database, and the
// contracts leave their rows in the tables.
func verificar(t *testing.T, b banco, name string, recipes ...string) {
	proj := project(t, b, name, recipes...)
	setup, err := os.ReadFile(filepath.Join(proj, "app", "setup.go"))
	must(t, err)
	for _, want := range []string{"// trilha:link billing-store", "// trilha:link notify-store"} {
		if strings.Count(string(setup), want) != 1 {
			t.Fatalf("setup.go has %q %d times:\n%s", want, strings.Count(string(setup), want), setup)
		}
	}
	main := runEach(t, b, proj, ".")
	if len(main) < 10 {
		t.Fatalf("only %d tests of the project ran on %s: %v", len(main), b.nome, main)
	}
	for test, tables := range map[string][]string{
		"TestBillingStoreContract":        {"billing_subscriptions", "billing_invoices", "billing_events"},
		"TestBillingWebhookValid":         {"billing_subscriptions", "billing_invoices", "billing_events"},
		"TestNotifyStoreContractOnTheApp": {"notify_preferences", "notify_outbox", "notify_digests"},
		"TestNotifyPreferences":           {"notify_preferences"},
	} {
		url, ok := main[test]
		if !ok {
			t.Errorf("%s did not run on %s", test, b.nome)
			continue
		}
		for _, table := range tables {
			if b.linhas(t, url, table) == 0 {
				t.Errorf("%s left %s empty on %s: the app did not use the database", test, table, b.nome)
			}
		}
	}
	for _, pkg := range []string{"./internal/cobranca/", "./internal/notificar/"} {
		runEach(t, b, proj, pkg)
	}
}

func TestBillingAndNotifyOnSQLite(t *testing.T) {
	verificar(t, sqlite, "lojaemsql", "store", "login", "connections", "webhooks", "billing", "notify")
}

// The other order: billing and notify first, the store last. The link lines
// are the store recipe's now.
func TestStoreAddedLast(t *testing.T) {
	verificar(t, sqlite, "lojadepois", "login", "connections", "webhooks", "billing", "notify", "store")
}

// Postgres is the other dialect: numbered placeholders, real booleans, a
// strict TIMESTAMP. The test starts a cluster of its own in a temporary
// directory, on a free local port, and stops it at the end; without
// PostgreSQL's programs it skips, unless SQLTEST_POSTGRES=1.
func TestBillingAndNotifyOnPostgres(t *testing.T) {
	verificar(t, postgres(t), "lojaempg", "store", "login", "connections", "webhooks", "billing", "notify")
}

func sh(dir string, env []string, args ...string) error {
	out, err := output(dir, env, args...)
	if err != nil {
		return fmt.Errorf("%s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}

func output(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
