package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/emersonjoe/trilha/internal/ctx"
)

// cmdCtx prints the map of the project: what an agent would otherwise learn
// by opening thirty files, once, in the cheapest form that still answers.
// --pack slices it for one job and budgets it; what the budget cuts is named,
// and --strict makes the cut an error instead of a note.
func cmdCtx(args []string) error {
	fs := flag.NewFlagSet("ctx", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, t("flag ctx json"))
	routes := fs.Bool("routes", false, t("flag ctx routes"))
	types := fs.Bool("types", false, t("flag ctx types"))
	all := fs.Bool("all", false, t("flag ctx all"))
	pack := fs.String("pack", "", t("flag ctx pack"))
	budget := fs.Int("budget", 0, t("flag ctx budget"))
	strict := fs.Bool("strict", false, t("flag ctx strict"))
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
	if *pack != "" && view != ctx.Compact {
		return errors.New(t("ctx pack view"))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	installed := ctx.InstalledRecipes(p.Root)
	c, err := ctx.Build(p.Root, p.Module, version)
	if err != nil {
		return err
	}
	c.InstalledRecipes = installed
	if *pack != "" {
		pk, err := c.PackOf(*pack, installed, *budget)
		if err != nil {
			return err
		}
		if *asJSON {
			b, err := pk.JSON()
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(b)
			if err != nil {
				return err
			}
		} else {
			fmt.Print(pk.Markdown())
		}
		if *strict && len(pk.Truncated) > 0 {
			return fmt.Errorf("E_CTX_BUDGET: %s", fmt.Sprintf(t("ctx budget cut"), strings.Join(pk.Truncated, ", ")))
		}
		return nil
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
