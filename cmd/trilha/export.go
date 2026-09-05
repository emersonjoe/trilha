package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	out := fs.String("o", "out", t("flag out"))
	base := fs.String("base", "", t("flag base"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	if _, err := generate(p); err != nil {
		return err
	}
	bin := filepath.Join(p.Root, ".trilha", "export-app")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	start := time.Now()
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = p.Root
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf(t("build failed"), err)
	}
	abs, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	run := exec.Command(bin)
	run.Dir = p.Root
	run.Env = append(os.Environ(), "TRILHA_ENV=prod", "TRILHA_EXPORT="+abs, "TRILHA_BASE_PATH="+*base)
	run.Stdout, run.Stderr = os.Stdout, os.Stderr
	if err := run.Run(); err != nil {
		return fmt.Errorf(t("export failed"), err)
	}
	fmt.Printf("✓ %s (%s)\n", *out, time.Since(start).Round(time.Millisecond))
	return nil
}
