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
	force := fs.Bool("force", false, t("flag force"))
	cssOnly := fs.Bool("css-only", false, t("flag css-only"))
	jsOnly := fs.Bool("js-only", false, t("flag js-only"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := scaffold.WriteUI(p.Root, *force, *cssOnly, *jsOnly)
	for _, r := range res {
		fmt.Printf("  %-14s %s\n", r.File, uiAction(r.Action))
	}
	if errors.Is(err, scaffold.ErrUIModified) {
		fmt.Fprintln(os.Stderr, "\n"+t("ui modified"))
		os.Exit(1)
	}
	return err
}

// uiAction translates a scaffold.UIResult action for display.
func uiAction(a string) string {
	switch a {
	case scaffold.UICreated:
		return t("ui created")
	case scaffold.UIUpdated:
		return t("ui updated")
	case scaffold.UIKept:
		return t("ui kept")
	case scaffold.UIKeptTheme:
		return t("ui kept theme")
	case scaffold.UIKeptOwn:
		return t("ui kept own")
	case scaffold.UIModified:
		return t("ui local")
	}
	return a
}
