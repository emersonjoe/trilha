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
	module := fs.String("module", "", t("flag module"))
	langFlag := fs.String("lang", lang, t("flag lang"))
	trilhaDir := fs.String("trilha-dir", "", t("flag trilha-dir"))
	noTidy := fs.Bool("no-tidy", false, t("flag no-tidy"))
	agents := fs.Bool("agents", false, t("flag agents"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New(t("new usage"))
	}
	dir := fs.Arg(0)
	// Allow flags after the positional argument: trilha new dir --module x.
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return err
	}
	if *langFlag != "en" && *langFlag != "pt" {
		return errors.New(t("bad lang"))
	}
	name := filepath.Base(dir)
	if *module == "" {
		*module = name
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	written, err := scaffold.Write(dir, scaffold.Data{Module: *module, Name: name, Lang: *langFlag})
	if err != nil {
		return err
	}
	for _, w := range written {
		fmt.Println("  +", w)
	}
	// AI support is opt-in: without --agents a new project gets neither file.
	if *agents {
		res, err := scaffold.WriteAgents(dir, scaffold.Data{Name: name, Lang: *langFlag}, false)
		if err != nil {
			return err
		}
		for _, r := range res {
			fmt.Println("  +", r.File)
		}
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
			fmt.Fprintln(os.Stderr, t("tidy failed"), err)
		}
	}
	// Generate trilha_gen.go so the project builds right away.
	abs, _ := filepath.Abs(dir)
	if _, err := generate(&project{Root: abs, Module: *module}); err != nil {
		return err
	}
	fmt.Printf(t("project created"), dir, dir)
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
