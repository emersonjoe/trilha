package recipes

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

// writeSetup puts the recipe's lines into app/setup.go.
//
// This is the one file a recipe edits rather than writes, and the marker is
// what makes that safe: a second run finds its own line and leaves it alone,
// instead of adding a second copy of a store nobody wanted twice.
//
// It answers the markers it actually inserted, so the command can say which
// lines it added and a test can assert that the second run added none.
func writeSetup(root string, r Recipe, linhas []Insert, dados map[string]any, dryRun bool) ([]string, error) {
	if len(linhas) == 0 {
		return nil, nil
	}
	rel := "app/setup.go"
	abs := filepath.Join(root, filepath.FromSlash(rel))
	src, err := os.ReadFile(abs)
	novo := os.IsNotExist(err)
	switch {
	case novo:
		src = []byte(baseSetup())
	case err != nil:
		return nil, err
	}
	texto := string(src)

	i := strings.Index(texto, "func Setup(a *trilha.App) error {")
	if i < 0 {
		// Saying the lines is better than guessing where they go: a Setup
		// somebody wrote by hand is a Setup whose order matters.
		var sb strings.Builder
		for _, in := range linhas {
			fmt.Fprintf(&sb, "\n%s%s", in.Marker, in.Line)
		}
		return nil, fmt.Errorf("%s has no `func Setup(a *trilha.App) error`; add these to yours:\n%s",
			rel, sb.String())
	}
	fim := strings.Index(texto[i:], "{") + i + 2

	var feitas []string
	var bloco strings.Builder
	var imports []string
	for _, in := range linhas {
		if strings.Contains(texto, in.Marker) {
			continue // já está lá: a marca existe para isto
		}
		fmt.Fprintf(&bloco, "\t%s\n%s", in.Marker, in.Line)
		feitas = append(feitas, in.Marker)
		imports = append(imports, in.Imports...)
	}
	if bloco.Len() == 0 {
		return nil, nil
	}
	texto = texto[:fim] + bloco.String() + texto[fim:]
	for _, caminho := range imports {
		texto = addImport(texto, caminho)
	}
	for _, imp := range r.Imports {
		caminho, err := render(imp, dados)
		if err != nil {
			return nil, err
		}
		texto = addImport(texto, caminho)
	}
	if dryRun {
		return feitas, nil
	}
	formatado, err := format.Source([]byte(texto))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, formatado, 0o644); err != nil {
		return nil, err
	}
	return feitas, nil
}

// baseSetup is the file for a project that has none.
func baseSetup() string {
	return `// Package app is where the routes live, and setup.go is what runs before the
// first request.
package app

import (
	"github.com/emersonjoe/trilha"
)

// Setup runs once, before the first request: it puts the dependencies where
// the pages find them with trilha.Use[T](c).
func Setup(a *trilha.App) error {
	return nil
}
`
}

// addImport puts a package in the import block, in a group of its own so gofmt
// leaves it where it was put — gofmt sorts inside a group and never moves a
// line between groups.
func addImport(src, path string) string {
	if strings.Contains(src, `"`+path+`"`) {
		return src
	}
	i := strings.Index(src, "\nimport (")
	if i < 0 {
		return addToSingleImport(src, path)
	}
	fim := strings.Index(src[i:], "\n)")
	if fim < 0 {
		return src
	}
	fim += i
	// A group of its own the first time, and the same group afterwards: three
	// recipes should not leave three one-line groups behind. gofmt sorts inside
	// a group and never moves a line between groups, so where it goes is a
	// choice made once, here.
	separador := "\n\n\t"
	if terminaEmImportDeProjeto(src[i:fim]) {
		separador = "\n\t"
	}
	return src[:fim] + separador + "\"" + path + "\"" + src[fim:]
}

// addToSingleImport turns a one-line import into a block with both.
func addToSingleImport(src, path string) string {
	i := strings.Index(src, "\nimport \"")
	if i < 0 {
		return src
	}
	fim := strings.Index(src[i+1:], "\n")
	if fim < 0 {
		return src
	}
	fim += i + 1
	velho := strings.TrimPrefix(strings.TrimSpace(src[i+1:fim]), "import ")
	return src[:i] + "\nimport (\n\t" + velho + "\n\n\t\"" + path + "\"\n)" + src[fim:]
}

// terminaEmImportDeProjeto reports whether the last line of the import block
// is somebody's own package rather than the standard library. The test is the
// dot: a standard library path has none before its first slash.
func terminaEmImportDeProjeto(bloco string) bool {
	linhas := strings.Split(strings.TrimRight(bloco, "\n"), "\n")
	ultima := strings.Trim(strings.TrimSpace(linhas[len(linhas)-1]), "\"")
	primeiro, _, _ := strings.Cut(ultima, "/")
	return strings.Contains(primeiro, ".")
}
