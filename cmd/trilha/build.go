package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	out := fs.String("o", "", "arquivo de saída (padrão bin/<nome-da-pasta>)")
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
	if *out == "" {
		*out = filepath.Join("bin", filepath.Base(p.Root))
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(p.Root, *out)), 0o755); err != nil {
		return err
	}
	start := time.Now()
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", *out, ".")
	cmd.Dir = p.Root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build falhou: %w", err)
	}
	fmt.Printf("✓ %s (%s)\n", *out, time.Since(start).Round(time.Millisecond))
	return nil
}
