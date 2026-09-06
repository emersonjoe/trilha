// Command trilha is the CLI: new, gen, generate, dev, build, routes, export,
// openapi, audit, ui.
// Messages follow TRILHA_LANG / LANG (see i18n.go).
package main

import (
	"go/token"
	"unicode"

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

const version = "0.33.0"

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
	case "generate":
		err = cmdGenerate(os.Args[2:])
	case "dev":
		err = cmdDev(os.Args[2:])
	case "build":
		err = cmdBuild(os.Args[2:])
	case "routes":
		err = cmdRoutes(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "openapi":
		err = cmdOpenAPI(os.Args[2:])
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

// render scans and generates trilha_gen.go in memory. pkg overrides the
// package clause; empty keeps the one the directory already declares.
func render(p *project, pkg string) (*scan.Result, []byte, error) {
	res, err := scan.Scan(p.Root, p.Module)
	if err != nil {
		return nil, nil, err
	}
	if pkg != "" {
		res.Package = pkg
	}
	src, err := gen.Generate(res)
	if err != nil {
		return nil, nil, err
	}
	return res, src, nil
}

// generate scans and writes trilha_gen.go. Returns the scan result.
func generate(p *project) (*scan.Result, error) { return generatePkg(p, "") }

func generatePkg(p *project, pkg string) (*scan.Result, error) {
	res, src, err := render(p, pkg)
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
	check := false
	pkg := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--check":
			check = true
		case args[i] == "--package" && i+1 < len(args):
			i++
			pkg = args[i]
		case strings.HasPrefix(args[i], "--package="):
			pkg = strings.TrimPrefix(args[i], "--package=")
		default:
			return fmt.Errorf(t("unknown flag"), args[i], "trilha gen [--check] [--package <name>]")
		}
	}
	if pkg != "" && !isPackageName(pkg) {
		return fmt.Errorf(t("bad package name"), pkg)
	}
	if check {
		return checkGen(p, pkg)
	}
	res, err := generatePkg(p, pkg)
	if err != nil {
		return err
	}
	fmt.Printf(t("gen done"), gen.FileName, len(res.Routes))
	return nil
}

// checkGen compares trilha_gen.go with a fresh generation without writing
// anything: one line in the CI, and a route added without `trilha gen` stops
// being a 404 nobody explains.
func checkGen(p *project, pkg string) error {
	_, src, err := render(p, pkg)
	if err != nil {
		return err
	}
	cur, err := os.ReadFile(filepath.Join(p.Root, gen.FileName))
	if err != nil {
		return fmt.Errorf("%s: %w", gen.FileName, err)
	}
	if string(cur) == string(src) {
		fmt.Println("✓", t("gen fresh"))
		return nil
	}
	fmt.Fprint(os.Stderr, t("gen diff"), genDiff(string(cur), string(src)))
	return errors.New(t("gen stale") + "; " + t("gen stale hint"))
}

// genDiff lists the lines of one side missing on the other. It is not a full
// diff: the generated file is sorted, so what is added or gone is the answer.
func genDiff(old, new string) string {
	count := map[string]int{}
	for _, l := range strings.Split(old, "\n") {
		count[l]++
	}
	var sb strings.Builder
	for _, l := range strings.Split(new, "\n") {
		if count[l] > 0 {
			count[l]--
			continue
		}
		fmt.Fprintf(&sb, "  + %s\n", strings.TrimSpace(l))
	}
	for _, l := range strings.Split(old, "\n") {
		if count[l] > 0 && strings.TrimSpace(l) != "" {
			count[l]--
			fmt.Fprintf(&sb, "  - %s\n", strings.TrimSpace(l))
		}
	}
	return sb.String()
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

// isPackageName reports whether s can be a Go package clause. A wrong name
// here surfaces as a compile error in the app, far from the flag that caused it.
func isPackageName(s string) bool {
	if s == "" || s == "_" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || unicode.IsLetter(r):
		case unicode.IsDigit(r) && i > 0:
		default:
			return false
		}
	}
	return !token.IsKeyword(s)
}

// embeddedPackage returns the package name when the app is one inside another
// binary — no main to run, so dev and build have nothing to start — and "" when
// the app is the binary itself.
func embeddedPackage(p *project) string {
	if pkg := scan.RootPackage(p.Root); pkg != "main" {
		return pkg
	}
	return ""
}
