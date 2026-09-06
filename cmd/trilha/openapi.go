package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emersonjoe/trilha/internal/openapi"
	"github.com/emersonjoe/trilha/internal/scan"
)

func cmdOpenAPI(args []string) error {
	fs := flag.NewFlagSet("openapi", flag.ContinueOnError)
	out := fs.String("o", openapi.FileName, t("flag openapi out"))
	title := fs.String("title", "", t("flag openapi title"))
	version := fs.String("version", "", t("flag openapi version"))
	server := fs.String("server", "", t("flag openapi server"))
	check := fs.Bool("check", false, t("flag openapi check"))
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		return err
	}
	doc, err := openapi.Generate(p.Root, res, openapi.Options{
		Title:   *title,
		Version: *version,
		Server:  *server,
	})
	if err != nil {
		return err
	}
	if *check {
		return checkOpenAPI(filepath.Join(p.Root, *out), doc)
	}
	if *out == "-" {
		_, err := os.Stdout.Write(doc)
		return err
	}
	if err := os.WriteFile(filepath.Join(p.Root, *out), doc, 0o644); err != nil {
		return err
	}
	fmt.Printf(t("openapi done"), *out, operationCount(res))
	return nil
}

// checkOpenAPI is the same comparison gen --check makes: the document in the
// repository is either what the code says today or it is a lie in the CI.
func checkOpenAPI(path string, doc []byte) error {
	cur, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if string(cur) == string(doc) {
		fmt.Println("✓", t("openapi fresh"))
		return nil
	}
	return errors.New(t("openapi stale") + "; " + t("openapi stale hint"))
}

func operationCount(res *scan.Result) int {
	n := 0
	for _, r := range res.Routes {
		if r.Kind == "api" {
			n += len(r.Methods)
		}
	}
	return n
}
