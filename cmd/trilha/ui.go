package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

func cmdUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	force := fs.Bool("force", false, "sobrescrever ui.css/ui.js modificados localmente")
	cssOnly := fs.Bool("css-only", false, "só ui.css (e ui.theme.css se faltar)")
	jsOnly := fs.Bool("js-only", false, "só ui.js")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := scaffold.WriteUI(p.Root, *force, *cssOnly, *jsOnly)
	for _, r := range res {
		fmt.Printf("  %-14s %s\n", r.File, r.Action)
	}
	if errors.Is(err, scaffold.ErrUIModified) {
		fmt.Fprintln(os.Stderr, "\n"+err.Error())
		os.Exit(1)
	}
	return err
}
