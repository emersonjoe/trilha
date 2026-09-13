package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/recipes"
	"github.com/emersonjoe/trilha/internal/scaffold"
)

func cmdNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	module := fs.String("module", "", t("flag module"))
	langFlag := fs.String("lang", lang, t("flag lang"))
	tmpl := fs.String("template", "blog", t("flag template"))
	trilhaDir := fs.String("trilha-dir", "", t("flag trilha-dir"))
	noTidy := fs.Bool("no-tidy", false, t("flag no-tidy"))
	agents := fs.Bool("agents", false, t("flag agents"))
	with := fs.String("with", "", t("flag with"))
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
	if !slices.Contains(scaffold.Templates(), *tmpl) {
		return errors.New(t("bad template"))
	}
	name := filepath.Base(dir)
	if *module == "" {
		*module = name
	}
	// The recipes of `trilha add`, applied at creation. The app template asks
	// for eight by default because they are what every internal application
	// grows in its first month — and asking for them here rather than copying
	// their screens into templates/ is what keeps one source: what the
	// template ships is exactly what `trilha add` writes.
	//
	// Resolved and checked before a single file is written: a --with that
	// drops what the template depends on, or that names a recipe that does
	// not exist, is a project that does not compile — and the person who
	// typed it deserves to find out before there is anything to clean up.
	receitas, err := resolveRecipes(*with, *tmpl, fs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	admin := false
	for _, nome := range receitas {
		if recipeDir(*tmpl, nome) == "app/admin/" {
			admin = true
			break
		}
	}
	written, err := scaffold.Write(dir, scaffold.Data{
		Module: *module, Name: name, Lang: *langFlag, Template: *tmpl, Admin: admin,
	})
	if err != nil {
		return err
	}
	for _, w := range written {
		fmt.Println("  +", w)
	}
	for _, nome := range receitas {
		r, err := recipes.Get(nome)
		if err != nil {
			return err
		}
		res, err := recipes.Add(dir, r, recipes.Options{
			Module: *module, Lang: *langFlag, At: recipeDir(*tmpl, nome),
		})
		if err != nil {
			return err
		}
		for _, f := range res.Written {
			fmt.Println("  +", f)
		}
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

// recipeList decides which recipes a new project starts with: what --with
// says, or the template's own answer when the flag was not given at all.
//
// An empty --with is a choice and not an absence — somebody who typed
// `--with ""` asked for the skeleton — so the flag being present is what
// decides, not the value being empty.
func recipeList(with, tmpl string, fs *flag.FlagSet) []string {
	dado := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "with" {
			dado = true
		}
	})
	if !dado {
		if tmpl == "app" {
			// login first: the others are screens behind it, and the order is
			// what the person reads in the output. users before permissions
			// and profile before tenant only for the reading; each pair ties
			// itself together whichever comes second.
			return []string{"login", "audit", "api-keys", "settings", "users", "permissions", "profile", "tenant"}
		}
		return nil
	}
	var out []string
	for _, s := range strings.Split(with, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// resolveRecipes is recipeList plus the two checks #212 asked for: the
// template's own dependencies (scaffold.Needs) are added even when --with
// left them out, with a warning naming what was added and why; and a name
// --with gives that no recipe answers to is refused with the list of the
// ones that do, before anything is written — a typo that runs quietly is a
// worse outcome than the error.
func resolveRecipes(with, tmpl string, fs *flag.FlagSet) ([]string, error) {
	out := recipeList(with, tmpl, fs)
	for _, need := range scaffold.Needs(tmpl) {
		if slices.Contains(out, need) {
			continue
		}
		fmt.Fprintf(os.Stderr, t("with needs"), tmpl, need)
		out = append([]string{need}, out...)
	}
	for _, nome := range out {
		if _, err := recipes.Get(nome); err != nil {
			return nil, fmt.Errorf(t("bad recipe"), nome, strings.Join(recipeNames(), ", "))
		}
	}
	return out, nil
}

// recipeNames is every recipe `trilha add` answers to, for the message that
// tells somebody what --with actually takes.
func recipeNames() []string {
	all := recipes.All()
	out := make([]string, len(all))
	for i, r := range all {
		out[i] = r.Name
	}
	return out
}

// recipeDir is where a template wants each recipe. The app template puts the
// administration screens behind a role — they name people, issue credentials
// and change how the application behaves for everybody, which is not the same
// door as a listing of items — and leaves at the root what everybody signed
// in reaches: the login, because a login behind /admin is a login most of the
// application cannot reach; the account screen and the organisation picker,
// because they are about the person asking and not about administering.
func recipeDir(tmpl, recipe string) string {
	if tmpl != "app" {
		return "app/"
	}
	switch recipe {
	case "login", "profile", "tenant":
		return "app/"
	}
	return "app/admin/"
}
