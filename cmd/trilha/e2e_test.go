package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestE2E runs `trilha new`, `trilha build` and boots the binary.
func TestE2E(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}
	repo, _ := filepath.Abs(filepath.Join("..", ".."))
	tmp := t.TempDir()
	cli := filepath.Join(tmp, "trilha-cli")
	run(t, repo, "go", "build", "-o", cli, "./cmd/trilha")

	proj := filepath.Join(tmp, "meu-app")
	out := run(t, tmp, cli, "new", proj, "--module", "example.com/meu-app", "--trilha-dir", repo)
	if !strings.Contains(out, "projeto criado") {
		t.Fatal(out)
	}
	for _, f := range []string{"go.mod", "trilha_gen.go", "app/page.go", "app/layout.go", "app/api/hello/route.go", "public/style.css", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	out = run(t, proj, cli, "routes")
	if !strings.Contains(out, "/api/hello") || !strings.Contains(out, "app/page.go") {
		t.Fatal(out)
	}
	run(t, proj, cli, "build", "-o", "bin/app")
	run(t, proj, cli, "export", "-o", "out")
	for _, f := range []string{"out/index.html", "out/404.html", "out/style.css", "out/.trilha-export"} {
		if _, err := os.Stat(filepath.Join(proj, f)); err != nil {
			t.Fatalf("export missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(proj, "out", "api")); err == nil {
		t.Fatal("api must not be exported")
	}

	// Boot the binary from a different directory: public/ must be embedded.
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bin := exec.CommandContext(ctx, filepath.Join(proj, "bin", "app"))
	bin.Dir = tmp
	bin.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "TRILHA_ENV=prod")
	bin.Stdout, bin.Stderr = io.Discard, io.Discard
	if err := bin.Start(); err != nil {
		t.Fatal(err)
	}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	body := waitGet(t, base+"/")
	if !strings.Contains(body, "Olá, meu-app!") || strings.Contains(body, "_trilha/events") {
		t.Fatal(body)
	}
	if b := waitGet(t, base+"/style.css"); !strings.Contains(b, "color-scheme") {
		t.Fatal("public not embedded:", b)
	}
	if b := waitGet(t, base+"/api/hello"); !strings.Contains(b, `"hello":"meu-app"`) {
		t.Fatal(b)
	}
	if b := waitGet(t, base+"/nada"); !strings.Contains(b, "404") {
		t.Fatal(b)
	}
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitGet(t *testing.T, url string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return string(b)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", url)
	return ""
}
