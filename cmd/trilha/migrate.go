package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersonjoe/trilha/internal/migrate"
)

// cmdMigrate is the front door of `trilha migrate <what>`. Today there is one
// what — next — and the shape is there so a second one does not change the
// spelling of the first.
func cmdMigrate(args []string) error {
	if len(args) == 0 || args[0] != "next" {
		return errors.New(t("migrate what"))
	}
	return cmdMigrateNext(args[1:])
}

// cmdMigrateNext writes the skeleton of app/ and the report. It reads a Next
// project and writes a Trilha one; what it will not do is translate a screen,
// which is the part that is not mechanical.
func cmdMigrateNext(args []string) error {
	fs := flag.NewFlagSet("migrate next", flag.ContinueOnError)
	out := fs.String("out", "app", t("flag migrate out"))
	report := fs.String("report", "", t("flag migrate report"))
	dryRun := fs.Bool("dry-run", false, t("flag migrate dry-run"))
	force := fs.Bool("force", false, t("flag migrate force"))
	lang := fs.String("lang", lang, t("flag lang"))
	// The directory is pulled out before parsing so the flags may come on
	// either side of it.
	var dir string
	rest := make([]string, 0, len(args))
	for _, a := range args {
		if dir == "" && !strings.HasPrefix(a, "-") {
			dir = a
			continue
		}
		rest = append(rest, a)
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if dir == "" {
		return errors.New(t("migrate needs a dir"))
	}
	if *lang != "en" && *lang != "pt" {
		return fmt.Errorf(t("bad lang"), *lang)
	}
	// The report says "the tree beside this file is the skeleton", so beside is
	// where it goes: next to --out, and not next to whoever ran the command.
	// With the default --out app that is ./MIGRATION.md, exactly as before.
	if *report == "" {
		*report = filepath.Join(filepath.Dir(filepath.Clean(*out)), migrate.ReportName)
	}
	p, err := migrate.Scan(dir)
	if err != nil {
		return err
	}
	md := migrate.Report(p, *lang)
	if *dryRun {
		fmt.Print(md)
		fmt.Printf("\n"+t("migrate dry")+"\n", len(p.Pages), *out, *report)
		return nil
	}
	res, err := migrate.Write(*out, p, *force)
	if err != nil {
		return err
	}
	created := 0
	for _, r := range res {
		if r.Action == migrate.Created {
			created++
		}
		// A route reads as a route on every system: the rest of the CLI already
		// prints app/page.go, and only this line was printing app\page.go.
		fmt.Printf("  %-40s %s\n", filepath.ToSlash(filepath.Join(*out, r.Path)), t("ui "+r.Action))
	}
	if err := os.WriteFile(*report, []byte(md), 0o644); err != nil {
		return err
	}
	fmt.Printf("  %-40s %s\n", *report, t("ui created"))
	fmt.Printf("\n"+t("migrate done")+"\n", created, len(res)-created, len(p.Notes))
	return nil
}

// absOf is the path as somebody would paste it into another command.
func absOf(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(abs)
}
