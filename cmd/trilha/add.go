package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/emersonjoe/trilha/internal/ctx"
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
		return listRecipes(os.Stdout, o.asJSON, o.lang)
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	var written int
	// tail is printed after gen. A cost line names its recipe instead of
	// carrying a number: the number is measured once the project is whole.
	type line struct{ text, costOf string }
	var tail []line
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
			tail = append(tail, line{text: res.Next})
		}
		if res.Doc != "" {
			tail = append(tail, line{text: t("add doc") + " https://trilha.dev" + res.Doc})
		}
		// The price of reading what just arrived, before anybody opens a file.
		tail = append(tail, line{costOf: nome})
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
	measured := packCosts(p)
	for _, l := range tail {
		if l.costOf != "" {
			r, _ := recipes.Get(l.costOf)
			cost, ok := measured[l.costOf]
			if !ok {
				cost = r.CtxPackCost
			}
			l.text = fmt.Sprintf(t("add cost"), l.costOf, cost)
		}
		fmt.Println("\n" + l.text)
	}
	return nil
}

// packCosts measures `trilha ctx --pack <recipe>` in this project, for every
// recipe installed in it: the number `--list` shows is measured on a minimal
// project, and a real one — with layouts, and the conventions it uses — costs
// a little more. What the add prints is what the command will cost here. A
// project that cannot be read yet answers nothing, and the listed price stands.
func packCosts(p *project) map[string]int {
	out := map[string]int{}
	installed := ctx.InstalledRecipes(p.Root)
	c, err := ctx.Build(p.Root, p.Module, version)
	if err != nil {
		return out
	}
	c.InstalledRecipes = installed // what `trilha ctx --pack` does
	for _, info := range installed {
		if pk, err := c.PackOf(info.Name, installed, 0); err == nil {
			out[info.Name] = pk.Used
		}
	}
	return out
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
// what the MCP server and an editor's agent read. Each line carries the
// recipe's price: what `trilha ctx --pack <name>` costs once it is installed,
// in estimated tokens.
func listRecipes(w io.Writer, asJSON bool, lang string) error {
	type linha struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
		Doc     string `json:"doc"`
		Cost    int    `json:"ctx_pack_tokens"`
	}
	var out []linha
	for _, r := range recipes.All() {
		resumo := r.Summary[lang]
		if resumo == "" {
			resumo = r.Summary["en"]
		}
		out = append(out, linha{Name: r.Name, Summary: resumo, Doc: r.Doc, Cost: r.CtxPackCost})
	}
	if asJSON {
		enc := json.NewEncoder(w)
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
		fmt.Fprintf(w, "  %-*s  %-16s  %s\n", largura, l.Name, fmt.Sprintf(t("add list cost"), l.Cost), l.Summary)
	}
	return nil
}
