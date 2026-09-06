package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

// cmdGenerate writes one skeleton in the right place. The argument of a page
// or a route is the URL, not the folder: the convention is what costs to
// remember, and remembering it is the command's job.
func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	force := fs.Bool("force", false, t("flag gen-force"))
	dir := fs.String("dir", "", t("flag gen-dir"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return errors.New(t("generate usage"))
	}
	kind, arg := fs.Arg(0), fs.Arg(1)
	// Allow flags after the positional arguments: trilha generate page /x --force.
	if err := fs.Parse(fs.Args()[2:]); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := scaffold.Generate(p.Root, scaffold.GenOptions{Kind: kind, Arg: arg, Force: *force, Dir: *dir})
	switch {
	case errors.Is(err, scaffold.ErrGenExists):
		return fmt.Errorf("%w — %s", err, t("gen use force"))
	case errors.Is(err, scaffold.ErrGenConflict):
		return fmt.Errorf("%w — %s", err, t("gen conflict"))
	case err != nil:
		return err
	}
	fmt.Println("  +", res.File)
	if res.Pattern == "" {
		return nil
	}
	// The route only exists once trilha_gen.go knows about it.
	if _, err := generate(p); err != nil {
		return err
	}
	fmt.Printf(t("generated route"), res.Pattern)
	return nil
}
