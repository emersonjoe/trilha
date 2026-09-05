package dev

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestWatchable(t *testing.T) {
	yes := []string{"app/page.go", "go.mod", "public/style.css", "internal/x.go"}
	no := []string{"trilha_gen.go", "README.md", "bin/app", "app/x.txt"}
	for _, p := range yes {
		if !Watchable(p) {
			t.Errorf("%s should be watchable", p)
		}
	}
	for _, p := range no {
		if Watchable(p) {
			t.Errorf("%s should be ignored", p)
		}
	}
}

func TestSnapshotDiff(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "app"), 0o755))
	must(os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	must(os.MkdirAll(filepath.Join(root, "bin"), 0o755))
	must(os.WriteFile(filepath.Join(root, "app", "page.go"), []byte("package app"), 0o644))
	must(os.WriteFile(filepath.Join(root, "trilha_gen.go"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, ".git", "x.go"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "bin", "x.go"), []byte("x"), 0o644))
	a := Take(root)
	if len(a) != 1 {
		t.Fatalf("%v", a)
	}
	time.Sleep(10 * time.Millisecond)
	must(os.WriteFile(filepath.Join(root, "app", "page.go"), []byte("package app // edit"), 0o644))
	must(os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x"), 0o644))
	b := Take(root)
	d := Diff(a, b)
	sort.Strings(d)
	if len(d) != 2 || d[0] != "app/page.go" || d[1] != "go.mod" {
		t.Fatal(d)
	}
	must(os.Remove(filepath.Join(root, "go.mod")))
	if d := Diff(b, Take(root)); len(d) != 1 || d[0] != "go.mod" {
		t.Fatal(d)
	}
}

func TestWatchEmitsDebounced(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("1"), 0o644)
	stop := make(chan struct{})
	defer close(stop)
	ch := Watch(root, 20*time.Millisecond, 50*time.Millisecond, stop)
	time.Sleep(30 * time.Millisecond)
	_ = os.WriteFile(filepath.Join(root, "a.go"), []byte("22"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "b.go"), []byte("3"), 0o644)
	select {
	case changed := <-ch:
		if len(changed) < 1 {
			t.Fatal("expected changes")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event")
	}
}
