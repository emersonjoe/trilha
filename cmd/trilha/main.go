// Command trilha is the CLI: new, gen, dev, build, routes, export, audit, ui.
// Messages follow TRILHA_LANG / LANG (see i18n.go).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/emersonjoe/trilha/internal/gen"
	"github.com/emersonjoe/trilha/internal/scan"
)

const version = "0.11.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, t("usage"))
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "new":
		err = cmdNew(os.Args[2:])
	case "gen":
		err = cmdGen(os.Args[2:])
	case "dev":
		err = cmdDev(os.Args[2:])
	case "build":
		err = cmdBuild(os.Args[2:])
	case "routes":
		err = cmdRoutes(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "ui":
		err = cmdUI(os.Args[2:])
	case "audit":
		err = cmdAudit(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("trilha", version)
	case "help", "-h", "--help":
		fmt.Print(t("usage"))
	default:
		fmt.Fprintf(os.Stderr, t("unknown command"), os.Args[1], t("usage"))
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, t("error:"), err)
		os.Exit(1)
	}
}

// project locates the project root (the nearest go.mod) and its module path.
type project struct {
	Root   string
	Module string
}

func findProject() (*project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if st, err := os.Stat(filepath.Join(cwd, "app")); err != nil || !st.IsDir() {
		return nil, errors.New(t("no app dir"))
	}
	dir := cwd
	for {
		gm := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(gm); err == nil {
			mod, err := modulePath(gm)
			if err != nil {
				return nil, err
			}
			if rel, _ := filepath.Rel(dir, cwd); rel != "." {
				mod = path.Join(mod, filepath.ToSlash(rel))
			}
			return &project{Root: cwd, Module: mod}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf(t("no go.mod"), cwd)
		}
		dir = parent
	}
}

func modulePath(gomod string) (string, error) {
	f, err := os.Open(gomod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "module ")), `"`), nil
		}
	}
	return "", fmt.Errorf(t("no module line"), gomod)
}

// generate scans and writes trilha_gen.go. Returns the scan result.
func generate(p *project) (*scan.Result, error) {
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		return nil, err
	}
	src, err := gen.Generate(res)
	if err != nil {
		return nil, err
	}
	out := filepath.Join(p.Root, gen.FileName)
	if old, err := os.ReadFile(out); err == nil && string(old) == string(src) {
		return res, nil
	}
	return res, os.WriteFile(out, src, 0o644)
}

func cmdGen(args []string) error {
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := generate(p)
	if err != nil {
		return err
	}
	fmt.Printf(t("gen done"), gen.FileName, len(res.Routes))
	return nil
}

func cmdRoutes(args []string) error {
	p, err := findProject()
	if err != nil {
		return err
	}
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		return err
	}
	fmt.Print(routesTable(res))
	return nil
}

func routesTable(res *scan.Result) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-22s %-32s %s\n", t("METHODS"), t("PATTERN"), t("SOURCE"))
	for _, r := range res.Routes {
		ms := r.Methods
		if r.HasPage {
			ms = append([]string{"GET"}, ms...)
		}
		file := "route.go"
		if r.Kind == "page" {
			file = "page.go"
		}
		fmt.Fprintf(&sb, "%-22s %-32s %s\n", strings.Join(ms, ","), r.Pattern, r.Dir+"/"+file)
	}
	return sb.String()
}
