package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emersonjoe/trilha/internal/scaffold"
)

// cmdAgents writes AGENTS.md and CLAUDE.md at the project root, the quality
// skill, and the pre-commit hook that holds the size limits. AI support is
// opt-in: nothing here runs unless it is asked for, by this command or by
// `trilha new --agents`.
func cmdAgents(args []string) error {
	fs := flag.NewFlagSet("agents", flag.ContinueOnError)
	force := fs.Bool("force", false, t("flag force agents"))
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
	res, err := scaffold.WriteAgents(p.Root, scaffold.Data{
		Name: filepath.Base(p.Root),
		Lang: *langFlag,
	}, *force)
	printAgents(res)
	if errors.Is(err, scaffold.ErrAgentsModified) {
		fmt.Fprintln(os.Stderr, "\n"+t("agents modified"))
		os.Exit(1)
	}
	if err != nil {
		return err
	}
	// The limits AGENTS.md states are enforced, not only stated: the hook
	// stops the commit that breaks them.
	return installHooks(p.Root, *force)
}

func printAgents(res []scaffold.UIResult) {
	for _, r := range res {
		fmt.Printf("  %-14s %s\n", r.File, uiAction(r.Action))
	}
}
