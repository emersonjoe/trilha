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
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	list := fs.Bool("list", false, t("flag add list"))
	asJSON := fs.Bool("json", false, t("flag add json"))
	dry := fs.Bool("dry-run", false, t("flag add dry"))
	langFlag := fs.String("lang", lang, t("flag lang"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *langFlag != "en" && *langFlag != "pt" {
		return errors.New(t("bad lang"))
	}
	// No name is the listing: somebody who types `trilha add` is asking what
	// there is, and answering with a usage error would be answering a
	// different question.
	if fs.NArg() == 0 || *list {
		return listRecipes(*asJSON, *langFlag)
	}
	nome := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return err
	}
	r, err := recipes.Get(nome)
	if err != nil {
		return fmt.Errorf("%w — %s", err, t("add see list"))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := recipes.Add(p.Root, r, recipes.Options{
		Module: p.Module, Lang: *langFlag, DryRun: *dry,
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
	if *dry {
		fmt.Println("\n" + t("add dry"))
		return nil
	}
	if len(res.Written) > 0 {
		if _, err := generate(p); err != nil {
			return err
		}
	}
	if res.Next != "" {
		fmt.Println("\n" + res.Next)
	}
	if res.Doc != "" {
		fmt.Println(t("add doc"), "https://trilha.dev"+res.Doc)
	}
	return nil
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
