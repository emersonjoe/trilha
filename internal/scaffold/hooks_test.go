package scaffold

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Spec 181: the hook is a script of the framework — executable, stamped under
// the shebang, kept, refreshed when untouched, refused when edited.
func TestWriteHooks(t *testing.T) {
	dir := t.TempDir()
	res, err := WriteHooks(dir, false)
	if err != nil || res.File != HookPath || res.Action != UICreated {
		t.Fatal(err, res)
	}
	p := filepath.Join(dir, filepath.FromSlash(HookPath))
	b, _ := os.ReadFile(p)
	if !strings.HasPrefix(string(b), "#!/bin/sh\n# trilha hooks ") || !hookStamp.untouched(b) {
		t.Fatalf("hook does not open with the shebang and its stamp:\n%s", b)
	}
	if !strings.Contains(string(b), "trilha precommit") {
		t.Fatalf("the hook does not run the check:\n%s", b)
	}
	if st, _ := os.Stat(p); st.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable: %v", st.Mode())
	}
	if res, err := WriteHooks(dir, false); err != nil || res.Action != UIKept {
		t.Fatal(err, res)
	}
	os.WriteFile(p, append(b, "\necho mine\n"...), 0o755)
	if res, err := WriteHooks(dir, false); !errors.Is(err, ErrAgentsModified) || res.Action != UIModified {
		t.Fatal(err, res)
	}
	if res, err := WriteHooks(dir, true); err != nil || res.Action != UIUpdated {
		t.Fatal(err, res)
	}
}

// core.hooksPath is pointed at .githooks once, and a project whose hooks live
// elsewhere keeps them.
func TestEnableHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if got := EnableHooks(dir); got != HooksNoRepo {
		t.Fatalf("outside a repository: %s", got)
	}
	exec.Command("git", "-C", dir, "init", "-q").Run()
	if got := EnableHooks(dir); got != HooksEnabled {
		t.Fatalf("first run: %s", got)
	}
	if got := EnableHooks(dir); got != HooksAlready {
		t.Fatalf("second run: %s", got)
	}
	exec.Command("git", "-C", dir, "config", "core.hooksPath", ".husky").Run()
	if got := EnableHooks(dir); got != HooksElsewhere {
		t.Fatalf("hooks of the project's own: %s", got)
	}
}
