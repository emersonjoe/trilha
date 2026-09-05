package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/scaffold"
)

func cmdNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	module := fs.String("module", "", "caminho do módulo Go (padrão: nome da pasta)")
	trilhaDir := fs.String("trilha-dir", "", "usar uma cópia local do trilha (adiciona replace no go.mod)")
	noTidy := fs.Bool("no-tidy", false, "não rodar go mod tidy")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("uso: trilha new <dir> [--module caminho]")
	}
	dir := fs.Arg(0)
	// Allow flags after the positional argument: trilha new dir --module x.
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return err
	}
	name := filepath.Base(dir)
	if *module == "" {
		*module = name
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	written, err := scaffold.Write(dir, scaffold.Data{Module: *module, Name: name})
	if err != nil {
		return err
	}
	for _, w := range written {
		fmt.Println("  +", w)
	}
	if *trilhaDir != "" {
		abs, err := filepath.Abs(*trilhaDir)
		if err != nil {
			return err
		}
		if err := runIn(dir, "go", "mod", "edit", "-replace", gen.RuntimeImport+"="+abs); err != nil {
			return err
		}
	}
	if !*noTidy {
		if err := runIn(dir, "go", "mod", "tidy"); err != nil {
			fmt.Fprintln(os.Stderr, "aviso: go mod tidy falhou (sem rede?); rode manualmente:", err)
		}
	}
	// Generate trilha_gen.go so the project builds right away.
	abs, _ := filepath.Abs(dir)
	if _, err := generate(&project{Root: abs, Module: *module}); err != nil {
		return err
	}
	fmt.Printf("\n✓ projeto criado em %s\n\n  cd %s\n  trilha dev\n", dir, dir)
	return nil
}

func runIn(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}
