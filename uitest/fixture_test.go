package uitest_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/emersonjoe/trilha/uitest"
)

// The fixture is what a user of the framework gets: a project made by
// `trilha new`, with the platform recipes added by `trilha add`, plus the
// pages of testdata/fixture — the patterns of spec 163 as they are, and one
// page per browser behavior (island, client navigation, tooltip). Nothing is
// written by hand to imitate a recipe.
var fixture struct {
	once sync.Once
	tmp  string
	dir  string
	err  error
}

// Credentials of the fixture: literals of a test, never used anywhere else.
const (
	adminEmail    = "admin@example.com"
	adminPassword = "a-password-nobody-guesses"
	appSecret     = "a-test-secret-with-more-than-32-bytes!!"
)

var env = []string{
	"TRILHA_ENV=prod",
	"TRILHA_SECRET=" + appSecret,
	"ADMIN_EMAIL=" + adminEmail,
	"ADMIN_PASSWORD=" + adminPassword,
}

func TestMain(m *testing.M) {
	code := m.Run()
	if fixture.tmp != "" {
		os.RemoveAll(fixture.tmp)
	}
	os.Exit(code)
}

// app returns the fixture's directory, building it on first use — after the
// browser check, so a machine without Chrome skips in a second.
func app(t *testing.T) string {
	t.Helper()
	uitest.RequireBrowser(t)
	fixture.once.Do(func() { fixture.dir, fixture.err = buildFixture() })
	if fixture.err != nil {
		t.Fatalf("building the fixture: %v", fixture.err)
	}
	return fixture.dir
}

// run is uitest.RunWith on the fixture, with its environment.
func run(t *testing.T, fn func(s *uitest.Session)) {
	t.Helper()
	uitest.RunWith(t, app(t), uitest.Config{Env: env}, fn)
}

func buildFixture() (string, error) {
	repo, err := filepath.Abs("..")
	if err != nil {
		return "", err
	}
	fixture.tmp, err = os.MkdirTemp("", "uitest-fixture-")
	if err != nil {
		return "", err
	}
	cli := filepath.Join(fixture.tmp, "trilha")
	proj := filepath.Join(fixture.tmp, "app")
	steps := []struct {
		dir  string
		args []string
	}{
		{repo, []string{"go", "build", "-o", cli, "./cmd/trilha"}},
		{fixture.tmp, []string{cli, "new", proj, "--module", "example.com/uitest", "--trilha-dir", repo}},
		{proj, []string{cli, "add", "login", "connections", "webhooks", "billing", "notify", "admin"}},
	}
	for _, s := range steps {
		if err := sh(s.dir, s.args...); err != nil {
			return "", err
		}
	}
	src := filepath.Join("testdata", "fixture")
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(proj, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		return "", err
	}
	if err := sh(proj, cli, "gen"); err != nil {
		return "", err
	}
	return proj, sh(proj, "go", "build", "./...")
}

func sh(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %v\n%s", args, err, out)
	}
	return nil
}
