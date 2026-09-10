package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

// cmdGenerate writes one skeleton in the right place. The argument of a page
// or a route is the URL, not the folder: the convention is what costs to
// remember, and remembering it is the command's job.
func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	force := fs.Bool("force", false, t("flag gen-force"))
	dir := fs.String("dir", "", t("flag gen-dir"))
	methods := fs.String("methods", "", t("flag gen-methods"))
	bind := fs.String("bind", "", t("flag gen-bind"))
	form := fs.String("form", "", t("flag gen-form"))
	layout := fs.String("layout", "", t("flag gen-layout"))
	langFlag := fs.String("lang", lang, t("flag lang"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return errors.New(t("generate usage"))
	}
	kind, arg := fs.Arg(0), fs.Arg(1)
	if kind == "crud" {
		return crud(arg, fs.Args()[2:])
	}
	// Allow flags after the positional arguments: trilha generate page /x --force.
	if err := fs.Parse(fs.Args()[2:]); err != nil {
		return err
	}
	if *langFlag != "en" && *langFlag != "pt" {
		return errors.New(t("bad lang"))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	var list []string
	if *methods != "" {
		list = strings.Split(*methods, ",")
	}
	res, err := scaffold.Generate(p.Root, scaffold.GenOptions{
		Kind: kind, Arg: arg, Force: *force, Dir: *dir,
		Methods: list, Bind: *bind, Form: *form, Layout: *layout,
		Module: p.Module, Lang: *langFlag,
	})
	switch {
	case errors.Is(err, scaffold.ErrGenExists):
		return fmt.Errorf("%w — %s", err, t("gen use force"))
	case errors.Is(err, scaffold.ErrGenConflict):
		return fmt.Errorf("%w — %s", err, t("gen conflict"))
	case err != nil:
		return err
	}
	for _, f := range res.Extra {
		fmt.Println("  +", f)
	}
	fmt.Println("  +", res.File)
	// A test answers no URL of its own, so trilha_gen.go has nothing to learn
	// from it.
	if res.Pattern == "" || kind == "test" {
		return nil
	}
	// The route only exists once trilha_gen.go knows about it.
	if _, err := generate(p); err != nil {
		return err
	}
	fmt.Printf(t("generated route"), res.Pattern)
	return nil
}

// crud writes the screens a struct needs. It is its own function because it
// answers with several files and three routes, and because its flags are not
// the flags of the other kinds — a shared flag set that half the kinds ignore
// is a help text nobody can read.
func crud(arg string, args []string) error {
	fs := flag.NewFlagSet("generate crud", flag.ContinueOnError)
	at := fs.String("at", "", t("flag crud at"))
	langFlag := fs.String("lang", lang, t("flag lang"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *langFlag != "en" && *langFlag != "pt" {
		return errors.New(t("bad lang"))
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	opts := scaffold.CrudOptions{Type: arg, At: *at, Module: p.Module, Lang: *langFlag}
	res, err := scaffold.Crud(p.Root, opts)
	switch {
	case errors.Is(err, scaffold.ErrGenExists):
		// No --force, and no offer of one: the CRUD is what somebody generates
		// after having edited it. What running it again is worth is the answer
		// to why they ran it — the struct grew a field — so that is what gets
		// printed, and it is information and not a failure.
		faltando, ferr := scaffold.CrudMissing(p.Root, opts)
		if ferr != nil {
			return fmt.Errorf("%w — %s", err, t("crud exists"))
		}
		if len(faltando) == 0 {
			fmt.Println(t("crud already") + " " + t("crud complete"))
			return nil
		}
		fmt.Println(t("crud already"))
		fmt.Println()
		for _, m := range faltando {
			where, how := t("crud missing form"), fmt.Sprintf("ui.Field(%q, …)", m.Form)
			if m.Kind == "list" {
				where, how = t("crud missing list"), fmt.Sprintf("{Key: %q, …}", m.Form)
			}
			fmt.Printf("  %s %s\n    %s %s %s:%d\n", m.Field, where, t("crud missing add"), how, m.File, m.Line)
		}
		return nil
	case err != nil:
		return err
	}
	for _, f := range res.Files {
		fmt.Println("  +", f)
	}
	// What could not be drawn is said out loud. The file carries a comment
	// where the field would go; this is so nobody has to open it to find out
	// that a field of theirs is on no screen.
	for _, f := range res.Skipped {
		fmt.Printf("  %s %s\n", t("crud skipped"), f)
	}
	// What is above the destination changes what the generated test can do, so
	// it is said here instead of being discovered when the test runs.
	if res.Guard != "" {
		if res.Helper != "" {
			fmt.Printf("  "+t("crud guard helper")+"\n", res.Guard, res.Helper)
		} else {
			fmt.Printf("  "+t("crud guard skip")+"\n", res.Guard)
		}
	}
	if _, err := generate(p); err != nil {
		return err
	}
	for _, pat := range res.Patterns {
		fmt.Printf(t("generated route"), pat)
	}
	return nil
}
