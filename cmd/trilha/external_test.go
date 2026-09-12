package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// An unknown command with a trilha-<name> on the PATH runs that program with
// the rest of the line and answers its exit code; without one, the CLI
// still says the command is unknown.
func TestExternalSubcommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script on the PATH")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "trilha-hello")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"hello $* $TRILHA_PARENT_VERSION\"\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	code, ok := runExternal("hello", []string{"task", "next"})
	if !ok || code != 3 {
		t.Fatalf("code=%d ok=%v", code, ok)
	}
	for _, name := range []string{"nope", "", "../hello", "a.b", `x\y`} {
		if _, ok := runExternal(name, nil); ok {
			t.Fatalf("%q was dispatched", name)
		}
	}

	// Through the binary: the output and the exit code are the program's.
	bin := filepath.Join(dir, "trilha")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %s", out)
	}
	out, err := exec.Command(bin, "hello", "task", "next").CombinedOutput()
	var ee *exec.ExitError
	if err == nil || !strings.Contains(string(out), "hello task next 0.") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if ok := errorAs(err, &ee); !ok || ee.ExitCode() != 3 {
		t.Fatalf("exit: %v", err)
	}
	out, err = exec.Command(bin, "nope").CombinedOutput()
	if err == nil || !strings.Contains(string(out), "unknown command") {
		t.Fatalf("unknown: %q %v", out, err)
	}
}
