package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/emersonjoe/trilha/internal/ctx"
)

// cmdCtx prints the map of the project: what an agent would otherwise learn
// by opening thirty files, once, in the cheapest form that still answers.
func cmdCtx(args []string) error {
	fs := flag.NewFlagSet("ctx", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, t("flag ctx json"))
	routes := fs.Bool("routes", false, t("flag ctx routes"))
	types := fs.Bool("types", false, t("flag ctx types"))
	all := fs.Bool("all", false, t("flag ctx all"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	view := ctx.Compact
	switch {
	case *routes && *types, *routes && *all, *types && *all:
		return errors.New(t("ctx one view"))
	case *routes:
		view = ctx.OnlyRoutes
	case *types:
		view = ctx.OnlyTypes
	case *all:
		view = ctx.All
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	c, err := ctx.Build(p.Root, p.Module, version)
	if err != nil {
		return err
	}
	if *asJSON {
		b, err := c.JSON()
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(b)
		return err
	}
	fmt.Print(c.Markdown(view))
	return nil
}
