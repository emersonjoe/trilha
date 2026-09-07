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
	report := fs.String("report", "MIGRATION.md", t("flag migrate report"))
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
		fmt.Printf("  %-40s %s\n", filepath.Join(*out, r.Path), t("ui "+r.Action))
	}
	if err := os.WriteFile(*report, []byte(md), 0o644); err != nil {
		return err
	}
	fmt.Printf("  %-40s %s\n", *report, t("ui created"))
	fmt.Printf("\n"+t("migrate done")+"\n", created, len(res)-created, len(p.Notes))
	return nil
}
