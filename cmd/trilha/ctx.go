package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/emersonjoe/trilha/internal/ctx"
	"github.com/emersonjoe/trilha/internal/recipes"
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
	installed := installedRecipes(p)
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

// installedRecipes names the recipes the project brought in: the markers the
// recipes themselves wrote into setup.go are the record, and each file a
// recipe brings is looked up where it actually landed. Nothing here trusts
// the registry alone — a file that is not on disk is not installed.
func installedRecipes(p *project) []ctx.RecipeInfo {
	src, err := os.ReadFile(filepath.Join(p.Root, "app", "setup.go"))
	if err != nil {
		return nil
	}
	var marker = regexp.MustCompile(`^// trilha:add ([a-z0-9-]+)$`)
	seen := map[string]bool{}
	var names []string
	for _, line := range strings.Split(string(src), "\n") {
		if m := marker.FindStringSubmatch(strings.TrimSpace(line)); m != nil && !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	sort.Strings(names)
	var out []ctx.RecipeInfo
	for _, name := range names {
		r, err := recipes.Get(name)
		if err != nil {
			continue // a marker nobody answers to is not a recipe
		}
		info := ctx.RecipeInfo{Name: r.Name, Doc: r.Doc}
		for _, f := range r.Files {
			rest := strings.ReplaceAll(f.Rel, "{{.At}}", "")
			for _, cand := range []string{rest, filepath.ToSlash(filepath.Join("app", rest)), filepath.ToSlash(filepath.Join("app/admin", rest))} {
				if _, err := os.Stat(filepath.Join(p.Root, filepath.FromSlash(cand))); err == nil {
					info.Files = append(info.Files, cand)
					break
				}
			}
		}
		out = append(out, info)
	}
	return out
}
