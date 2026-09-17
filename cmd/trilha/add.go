package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/emersonjoe/trilha/internal/recipes"
)

// cmdAdd writes a recipe into the project.
//
// The difference from generate is the direction: generate writes from the
// user's code — a struct becomes a screen — and add writes from a recipe of
// the framework's. Both end with gen, and both leave `trilha check` green.
func cmdAdd(args []string) error {
	names, o, err := parseAddArgs(args)
	if err != nil {
		return err
	}
	// No name is the listing: somebody who types `trilha add` is asking what
	// there is, and answering with a usage error would be answering a
	// different question.
	if len(names) == 0 || o.list {
		return listRecipes(o.asJSON, o.lang)
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	var written int
	var tail []string
	for _, nome := range names {
		r, _ := recipes.Get(nome) // parseAddArgs already resolved every name
		if len(names) > 1 {
			fmt.Printf("%s:\n", nome)
		}
		res, err := recipes.Add(p.Root, r, recipes.Options{
			Module: p.Module, Lang: o.lang, DryRun: o.dry,
		})
		if err != nil {
			return err
		}
		for _, f := range res.Written {
			fmt.Println("  +", f)
		}
		// Skipped and not refused: a second run is adding, not starting over, and
		// the file that is already there is one the person owns now.
		for _, f := range res.Skipped {
			fmt.Printf("  · %s (%s)\n", f, t("add skipped"))
		}
		for range res.Setup {
			fmt.Printf("  ~ app/setup.go (%s)\n", t("add wired"))
		}
		written += len(res.Written)
		if res.Next != "" {
			tail = append(tail, res.Next)
		}
		if res.Doc != "" {
			tail = append(tail, t("add doc")+" https://trilha.dev"+res.Doc)
		}
	}
	if o.dry {
		fmt.Println("\n" + t("add dry"))
		return nil
	}
	if written > 0 {
		if _, err := generate(p); err != nil {
			return err
		}
	}
	for _, line := range tail {
		fmt.Println("\n" + line)
	}
	return nil
}

// addOpts is what the flags of `trilha add` said.
type addOpts struct {
	list, asJSON, dry bool
	lang              string
}

// parseAddArgs reads `trilha add [flags] name [name...] [flags]`: every
// positional is a recipe and a flag counts wherever it stands. The flag
// package stops at the first positional, so it is run again from there
// until nothing is left — which is what let `login users audit --lang pt`
// apply one recipe, in the wrong language, with exit 0 (#253, #254). Every
// name is resolved here, before a file is written, so a typo in the third
// name costs nothing.
func parseAddArgs(args []string) ([]string, addOpts, error) {
	var o addOpts
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.BoolVar(&o.list, "list", false, t("flag add list"))
	fs.BoolVar(&o.asJSON, "json", false, t("flag add json"))
	fs.BoolVar(&o.dry, "dry-run", false, t("flag add dry"))
	fs.StringVar(&o.lang, "lang", lang, t("flag lang"))
	var names []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, o, err
		}
		if fs.NArg() == 0 {
			break
		}
		names = append(names, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	if o.lang != "en" && o.lang != "pt" {
		return nil, o, errors.New(t("bad lang"))
	}
	for _, nome := range names {
		if _, err := recipes.Get(nome); err != nil {
			return nil, o, fmt.Errorf("%s: %w — %s", nome, err, t("add see list"))
		}
	}
	return names, o, nil
}

// listRecipes answers what there is, in one line each — or as JSON, which is
// what the MCP server and an editor's agent read.
func listRecipes(asJSON bool, lang string) error {
	type linha struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
		Doc     string `json:"doc"`
	}
	var out []linha
	for _, r := range recipes.All() {
		resumo := r.Summary[lang]
		if resumo == "" {
			resumo = r.Summary["en"]
		}
		out = append(out, linha{Name: r.Name, Summary: resumo, Doc: r.Doc})
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	largura := 0
	for _, l := range out {
		if len(l.Name) > largura {
			largura = len(l.Name)
		}
	}
	for _, l := range out {
		fmt.Printf("  %-*s  %s\n", largura, l.Name, l.Summary)
	}
	return nil
}
