package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/emersonjoe/trilha/internal/scaffold"
	"github.com/emersonjoe/trilha/internal/uidoc"
)

func cmdUI(args []string) error {
	// `describe` reads the catalogue that ships with the CLI: it answers
	// anywhere, with or without a project around.
	if len(args) > 0 && args[0] == "describe" {
		return cmdUIDescribe(args[1:])
	}
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	force := fs.Bool("force", false, t("flag force"))
	cssOnly := fs.Bool("css-only", false, t("flag css-only"))
	jsOnly := fs.Bool("js-only", false, t("flag js-only"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := scaffold.WriteUI(p.Root, *force, *cssOnly, *jsOnly)
	for _, r := range res {
		fmt.Printf("  %-14s %s\n", r.File, uiAction(r.Action))
	}
	if errors.Is(err, scaffold.ErrUIModified) {
		fmt.Fprintln(os.Stderr, "\n"+t("ui modified"))
		os.Exit(1)
	}
	return err
}

// uiAction translates a scaffold.UIResult action for display.
func uiAction(a string) string {
	switch a {
	case scaffold.UICreated:
		return t("ui created")
	case scaffold.UIUpdated:
		return t("ui updated")
	case scaffold.UIKept:
		return t("ui kept")
	case scaffold.UIKeptTheme:
		return t("ui kept theme")
	case scaffold.UIKeptOwn:
		return t("ui kept own")
	case scaffold.UIModified:
		return t("ui local")
	}
	return a
}

// cmdUIDescribe prints the ui catalogue: the whole list, or one component with
// its signature, what it is for and how it is called. It is the answer to the
// question an agent asks before writing a screen — what exists and how is it
// spelled — without opening the source or guessing.
func cmdUIDescribe(args []string) error {
	// The name is pulled out before parsing so `describe Field --json` works
	// as well as `describe --json Field`: whoever is typing should not have to
	// know the order the flag package wants.
	var name string
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if name == "" && !strings.HasPrefix(a, "-") {
			name = a
			continue
		}
		rest = append(rest, a)
	}
	fs := flag.NewFlagSet("ui describe", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, t("flag describe json"))
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		if *asJSON {
			os.Stdout.Write(uidoc.JSON())
			fmt.Println()
			return nil
		}
		listComponents()
		return nil
	}
	c, ok := uidoc.Lookup(name)
	if !ok {
		msg := fmt.Sprintf(t("no such component"), name)
		if near := uidoc.Similar(name); len(near) > 0 {
			msg += "\n  → " + fmt.Sprintf(t("did you mean"), strings.Join(near, ", "))
		}
		return errors.New(msg)
	}
	if *asJSON {
		b, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	describeComponent(c)
	return nil
}

// listComponents prints every component grouped as the source groups them,
// one line each: the name and the first sentence.
func listComponents() {
	group := ""
	for i, c := range uidoc.Components() {
		if g := shortGroup(c.Group); g != group {
			group = g
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(group)
		}
		fmt.Printf("  %-18s %s\n", c.Name, clip(c.Summary, 78))
	}
	fmt.Printf("\n"+t("describe hint")+"\n", len(uidoc.Components()))
}

// describeComponent prints one component: signature, doc, fields, example and
// the symbols the doc cites.
func describeComponent(c uidoc.Component) {
	fmt.Println(qualify(c))
	if c.Doc != "" {
		fmt.Println()
		for _, line := range strings.Split(c.Doc, "\n") {
			fmt.Println("  " + line)
		}
	}
	if len(c.Fields) > 0 {
		fmt.Println("\n  " + t("fields:"))
		w := 0
		for _, f := range c.Fields {
			if len(f.Name) > w {
				w = len(f.Name)
			}
		}
		for _, f := range c.Fields {
			line := fmt.Sprintf("    %-*s %s", w, f.Name, f.Type)
			if f.Doc != "" {
				line += "  — " + clip(firstLine(f.Doc), 60)
			}
			fmt.Println(line)
		}
	}
	if c.Example != "" {
		fmt.Println("\n  " + t("example:"))
		for _, line := range strings.Split(c.Example, "\n") {
			fmt.Println("    " + line)
		}
	}
	if len(c.See) > 0 {
		fmt.Println("\n  " + t("see:") + " " + strings.Join(c.See, ", "))
	}
}

// qualify writes the signature as it is called from an app: ui.Field(...) for
// a function, type ui.ShellOpts struct for a type.
func qualify(c uidoc.Component) string {
	if c.Kind == "func" {
		return "ui." + c.Signature
	}
	return strings.Replace(c.Signature, "type "+c.Name, "type ui."+c.Name, 1)
}

// shortGroup drops the parenthetical of a group banner: the heading is a
// heading, not a sentence.
func shortGroup(g string) string {
	if i := strings.Index(g, " ("); i > 0 {
		return g[:i]
	}
	return g
}

// firstLine is the text up to the first line break.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// clip shortens s to n runes, with an ellipsis when it had to cut.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}
