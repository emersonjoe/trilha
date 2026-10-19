package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/emersonjoe/trilha/internal/precommit"
	"github.com/emersonjoe/trilha/internal/scaffold"
)

// errPrecommit is the exit of a check that found files over the limits; the
// report is already on stderr.
var errPrecommit = errors.New("precommit: files over the size limits")

// cmdPrecommit is what .githooks/pre-commit runs: the staged files against
// the limits of AGENTS.md, with the way to split them when one is over.
func cmdPrecommit(args []string) error {
	fs := flag.NewFlagSet("precommit", flag.ContinueOnError)
	all := fs.Bool("all", false, t("flag precommit all"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := gitRoot()
	if err != nil {
		return err
	}
	var vs []precommit.Violation
	if *all {
		vs, err = precommit.Tracked(root)
	} else {
		vs, err = precommit.Staged(root)
	}
	if err != nil {
		return err
	}
	if len(vs) == 0 {
		fmt.Print(t("precommit ok"))
		return nil
	}
	reportPrecommit(vs)
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
	return errPrecommit
}

func reportPrecommit(vs []precommit.Violation) {
	files := map[string]bool{}
	for _, v := range vs {
		files[v.Path] = true
	}
	fmt.Fprintf(os.Stderr, t("precommit head"), len(files))
	for _, v := range vs {
		key := "precommit tokens"
		if v.Kind == precommit.Lines {
			key = "precommit lines"
		}
		fmt.Fprintf(os.Stderr, t(key), v.Path, v.Count, v.Limit)
	}
	fmt.Fprint(os.Stderr, t("precommit how"))
}

// cmdHooks writes .githooks/pre-commit and points git at it.
func cmdHooks(args []string) error {
	fs := flag.NewFlagSet("hooks", flag.ContinueOnError)
	force := fs.Bool("force", false, t("flag force hooks"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	return installHooks(p.Root, *force)
}

// installHooks is shared by trilha hooks and trilha agents.
func installHooks(root string, force bool) error {
	res, err := scaffold.WriteHooks(root, force)
	printAgents([]scaffold.UIResult{res})
	if errors.Is(err, scaffold.ErrAgentsModified) {
		fmt.Fprintln(os.Stderr, "\n"+t("agents modified"))
		os.Exit(1)
	}
	if err != nil {
		return err
	}
	switch scaffold.EnableHooks(root) {
	case scaffold.HooksEnabled:
		fmt.Println("  " + t("hooks enabled"))
		fmt.Println("  " + t("hooks per clone"))
	case scaffold.HooksAlready:
		fmt.Println("  " + t("hooks already"))
	case scaffold.HooksElsewhere:
		fmt.Println("  " + t("hooks elsewhere"))
	default:
		fmt.Println("  " + t("hooks no repo"))
	}
	return nil
}

func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", errors.New("trilha precommit: not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}
